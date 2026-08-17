# Full local-state snapshot/restore so a fresh `up-dev` skips BOTH the
# sequential external-dep setup (dev_dependencies.sh) AND the GitLab migration
# jobs. This is the capture half; restore is wired into up-dev once the
# captured artifacts are validated.
#
#   nix run .#snapshot-dev    capture a running gitlab-system into the cache
#
# What we capture (keyed by deployed chart version), and why:
#   pg_dumpall.sql.gz   logical dump of the CNPG DB — the migrated schema+data.
#                       Restore loads this so `db:migrate` is a no-op. Logical
#                       (not PV-level) so restore recreates CNPG fresh and just
#                       loads it — avoids fragile PVC adoption.
#   garage-data.tar.gz  Garage's metadata+data dirs (buckets + S3 keys). Garage
#                       has NO PVC (emptyDir), so its keys — which must match the
#                       dev-garage-* secrets GitLab uses — only exist here.
#   secrets.json        all gitlab-system secrets (minus SA tokens / helm state):
#                       gitlab-rails-secret etc. must match the restored DB, and
#                       the dep secrets carry CNPG/Valkey/Garage credentials.
#   dep-configmaps.json dev-*/cnpg-* configmaps (dep config; e.g. garage.toml).
#   dep-workloads.json  Valkey/CNPG-operator/Garage workloads (recreate the deps
#                       without re-running dev_dependencies.sh).
#   cnpg-cluster.json / cnpg-crds.json / cnpg-clusterrbac.json  CNPG plumbing.
#   external-deps.yaml  the CR values file dev_dependencies.sh generated (secret
#                       refs) — restore reuses it since the dep secret names are stable.
{
  pkgs,
  lib,
  cfg,
  runtimeProbe,
  cacheEnv,
  envDefaults,
  clusterTools,
  mkScript,
  mkApp,
}:
let
  snapTools = clusterTools ++ [
    pkgs.jq
    pkgs.gzip
    pkgs.gnused
    pkgs.coreutils
  ];

  # metadata cleanup shared by the resource captures — strips server-populated
  # fields so the JSON re-applies cleanly into a fresh cluster.
  stripMeta = "del(.metadata.resourceVersion,.metadata.uid,.metadata.creationTimestamp,.metadata.generation,.status,.metadata.managedFields,.metadata.ownerReferences)";

  snapshotDev = mkScript "snapshot-dev" snapTools ''
    ${envDefaults}
    ${cacheEnv}
    ${runtimeProbe}
    snapdir="$CACHE_DIR/snapshot-$CHART_VER"
    rm -rf "$snapdir"; mkdir -p "$snapdir"
    kc=(kubectl --context "$KUBE_CONTEXT" -n "$TARGET_NAMESPACE")

    echo "==> [1/7] pg_dumpall from the CNPG primary"
    prim="$("''${kc[@]}" get cluster.postgresql.cnpg.io dev-cluster \
      -o jsonpath='{.status.currentPrimary}' 2>/dev/null || true)"
    : "''${prim:=dev-cluster-1}"
    echo "    primary: $prim"
    if "''${kc[@]}" exec "$prim" -c postgres -- pg_dumpall -U postgres \
        | gzip > "$snapdir/pg_dumpall.sql.gz"; then
      echo "    $(du -h "$snapdir/pg_dumpall.sql.gz" | cut -f1)"
    else
      echo "    ERROR: pg_dumpall failed" >&2
    fi

    echo "==> [2/7] Garage data (buckets + S3 keys) — via the node (garage image is distroless: no shell/tar)"
    # The dev-garage-0 meta/data volumes are emptyDirs; read them off the kind
    # node FS (like bake) instead of exec'ing tar inside the distroless pod.
    grt="$(detect_runtime 2>/dev/null || true)"
    gnode="''${KIND_CLUSTER_NAME}-control-plane"
    guid="$("''${kc[@]}" get pod dev-garage-0 -o jsonpath='{.metadata.uid}' 2>/dev/null || true)"
    if [ -n "$grt" ] && [ -n "$guid" ]; then
      gbase="/var/lib/kubelet/pods/$guid/volumes/kubernetes.io~empty-dir"
      echo "    node=$gnode base=$gbase"
      "$grt" exec "$gnode" cat "$gbase/etc/garage.toml" > "$snapdir/garage.toml" 2>/dev/null || true
      if "$grt" exec "$gnode" tar c -C "$gbase" meta data 2>/dev/null \
          | gzip > "$snapdir/garage-data.tar.gz"; then
        echo "    garage-data: $(du -h "$snapdir/garage-data.tar.gz" | cut -f1)"
      else
        # The pipeline already wrote a (valid, ~empty) gzip before tar failed.
        # Remove it so restore can DETECT the missing capture instead of
        # silently extracting nothing and reporting success.
        echo "    warn: could not tar garage emptyDir at $gbase (meta/data) — removing partial archive"
        rm -f "$snapdir/garage-data.tar.gz"
      fi
    else
      echo "    warn: no container runtime ($grt) or garage pod uid ($guid) — garage data NOT captured"
    fi
    # StatefulSet spec so restore can repoint the meta/data emptyDir volumes at
    # the restored data (hostPath). Confirmed volumes: meta->/mnt/meta,
    # data->/mnt/data, etc->/etc/garage.toml(subPath), configmap dev-garage-config.
    "''${kc[@]}" get statefulset dev-garage -o json | jq "${stripMeta}" > "$snapdir/garage-statefulset.json" 2>/dev/null || true

    echo "==> [3/7] secrets (sanitized; minus SA tokens + helm release state)"
    "''${kc[@]}" get secret -o json | jq '{
      apiVersion:"v1", kind:"List", items:[ .items[]
        | select(.type != "kubernetes.io/service-account-token")
        | select((.metadata.name // "") | startswith("sh.helm.release") | not)
        | { apiVersion:"v1", kind:"Secret", type, data,
            metadata:{ name:.metadata.name, namespace:.metadata.namespace,
                       labels:.metadata.labels } } ] }' \
      > "$snapdir/secrets.json"
    echo "    $("''${kc[@]}" get secret -o json | jq '[.items[]|select(.type != "kubernetes.io/service-account-token")]|length') secrets"

    echo "==> [4/7] dep configmaps (dev-*/cnpg-*)"
    "''${kc[@]}" get configmap -o json | jq '{
      apiVersion:"v1", kind:"List", items:[ .items[]
        | select((.metadata.name // "") | test("^(dev-|cnpg-)"))
        | { apiVersion:"v1", kind:"ConfigMap", data, binaryData,
            metadata:{ name:.metadata.name, namespace:.metadata.namespace,
                       labels:.metadata.labels } } ] }' \
      > "$snapdir/dep-configmaps.json"

    echo "==> [5/7] dep workloads (Valkey / CNPG operator / Garage) + Valkey PVC"
    "''${kc[@]}" get deployment,statefulset,service,serviceaccount,role,rolebinding \
      -l 'app.kubernetes.io/instance in (dev-valkey,dev-cnpg,dev-garage)' -o json \
      | jq "del(.items[].metadata.resourceVersion,.items[].metadata.uid,.items[].metadata.creationTimestamp,.items[].metadata.generation,.items[].status,.items[].metadata.managedFields,.items[].metadata.ownerReferences)" \
      > "$snapdir/dep-workloads.json"
    # Valkey is a plain Deployment+PVC (not operator-managed): recreate an EMPTY
    # PVC (drop volumeName/status so local-path re-provisions fresh — it's cache).
    "''${kc[@]}" get pvc dev-valkey -o json 2>/dev/null \
      | jq 'del(.metadata.resourceVersion,.metadata.uid,.metadata.creationTimestamp,.metadata.generation,.metadata.finalizers,.metadata.annotations,.status,.spec.volumeName)' \
      > "$snapdir/dep-pvcs.json" 2>/dev/null || : > "$snapdir/dep-pvcs.json"

    echo "==> [6/7] CNPG cluster + CRDs + cluster RBAC + admission webhooks"
    "''${kc[@]}" get cluster.postgresql.cnpg.io dev-cluster -o json | jq "${stripMeta}" > "$snapdir/cnpg-cluster.json"
    kubectl --context "$KUBE_CONTEXT" get crd -o json | jq '{
      apiVersion:"v1", kind:"List", items:[ .items[]
        | select(.spec.group | test("cnpg.io"))
        | del(.metadata.resourceVersion,.metadata.uid,.metadata.creationTimestamp,.metadata.generation,.status,.metadata.managedFields) ] }' \
      > "$snapdir/cnpg-crds.json"
    kubectl --context "$KUBE_CONTEXT" get clusterrole,clusterrolebinding \
      -l 'app.kubernetes.io/instance=dev-cnpg' -o json \
      | jq "del(.items[].metadata.resourceVersion,.items[].metadata.uid,.items[].metadata.creationTimestamp,.items[].metadata.generation,.items[].status,.items[].metadata.managedFields)" \
      > "$snapdir/cnpg-clusterrbac.json" 2>/dev/null || echo '{"apiVersion":"v1","kind":"List","items":[]}' > "$snapdir/cnpg-clusterrbac.json"
    # CNPG's operator UPDATES these but does not CREATE them (helm does) — it
    # crashes at PKI setup if they're absent. Cluster-scoped, so capture by name.
    kubectl --context "$KUBE_CONTEXT" get mutatingwebhookconfiguration,validatingwebhookconfiguration -o json | jq '{
      apiVersion:"v1", kind:"List", items:[ .items[]
        | select((.metadata.name // "") | startswith("cnpg"))
        | del(.metadata.resourceVersion,.metadata.uid,.metadata.creationTimestamp,.metadata.generation,.status,.metadata.managedFields) ] }' \
      > "$snapdir/cnpg-webhooks.json"

    echo "==> [7/7] CR values file (external-deps.yaml)"
    if [ -f external-deps.yaml ]; then
      cp external-deps.yaml "$snapdir/external-deps.yaml"
      echo "    captured external-deps.yaml"
    else
      echo "    warn: external-deps.yaml not in repo root (restore will regenerate refs)"
    fi

    echo ""
    echo "==> snapshot written to $snapdir"
    ls -lh "$snapdir"
  '';
  # Restore a captured snapshot into a FRESH (empty) cluster, bypassing
  # dev_dependencies.sh and pre-migrating the DB. Run after the cluster exists
  # (kind-up / up-dev) and before the operator + CR are deployed.
  restoreDev = mkScript "restore-dev" snapTools ''
    ${envDefaults}
    ${cacheEnv}
    ${runtimeProbe}
    snapdir="$CACHE_DIR/snapshot-$CHART_VER"
    if [ ! -d "$snapdir" ]; then
      echo "no snapshot at $snapdir — run 'nix run .#snapshot-dev' after a full up-dev" >&2
      exit 1
    fi
    kc=(kubectl --context "$KUBE_CONTEXT")
    kcn=(kubectl --context "$KUBE_CONTEXT" -n "$TARGET_NAMESPACE")

    echo "==> [1/8] namespace + CNPG CRDs"
    "''${kc[@]}" create namespace "$TARGET_NAMESPACE" --dry-run=client -o yaml | "''${kc[@]}" apply -f -
    # Server-side apply: the CNPG CRDs (clusters/poolers) exceed the 256KB
    # client-side last-applied-configuration annotation limit.
    "''${kc[@]}" apply --server-side --force-conflicts -f "$snapdir/cnpg-crds.json"
    # The CNPG operator crashes at PKI setup if its webhook configs are absent
    # (it manages but doesn't create them). Restore before the operator pod
    # starts; it re-injects a fresh caBundle on startup (manageWebhookConfigurations).
    if [ -s "$snapdir/cnpg-webhooks.json" ]; then
      "''${kc[@]}" apply --server-side --force-conflicts -f "$snapdir/cnpg-webhooks.json"
    fi

    echo "==> [2/8] app secrets (GitLab + Valkey + Garage; CNPG/TLS regenerated)"
    # Keep only secrets that must match the restored DB/deps: GitLab app secrets
    # (esp. gitlab-rails-secret -> DB column encryption), Valkey auth, Garage S3
    # keys. Drop kubernetes.io/tls (deploy.sh + operator + cert-manager reissue
    # for THIS env) and CNPG-owned secrets (dev-cluster-*/cnpg-* — CNPG
    # regenerates its PKI + app secret on the fresh cluster).
    jq '.items |= map(select(
          (.type != "kubernetes.io/tls")
          and (((.metadata.name // "") | test("^dev-cluster|^cnpg-|^gitlab-wildcard-tls")) | not)))' \
      "$snapdir/secrets.json" | "''${kcn[@]}" apply -f -

    echo "==> [3/8] dep configmaps + CNPG cluster RBAC"
    "''${kcn[@]}" apply -f "$snapdir/dep-configmaps.json"
    "''${kc[@]}" apply -f "$snapdir/cnpg-clusterrbac.json" || true

    echo "==> [4/8] pre-place Garage data on the node (hostPath)"
    grt="$(detect_runtime 2>/dev/null || true)"
    gnode="''${KIND_CLUSTER_NAME}-control-plane"
    ghost="/var/lib/gitlab-snapshot/garage"
    # Guard on a non-empty archive: snapshot-dev removes it when capture failed,
    # so its absence means "no data captured" — warn loudly and let Garage start
    # empty rather than aborting the whole restore or silently claiming success.
    if [ -n "$grt" ] && [ -s "$snapdir/garage-data.tar.gz" ]; then
      "$grt" exec "$gnode" sh -c "rm -rf $ghost && mkdir -p $ghost"
      gzip -dc "$snapdir/garage-data.tar.gz" | "$grt" exec -i "$gnode" tar x -C "$ghost"
    elif [ -z "$grt" ]; then
      echo "  warn: no container runtime — Garage will start EMPTY (object storage broken)"
    else
      echo "  warn: no garage snapshot ($snapdir/garage-data.tar.gz missing/empty) — Garage will start EMPTY (object storage broken)"
    fi

    echo "==> [5/8] dep workloads (Valkey, CNPG operator) + Garage (meta/data -> hostPath)"
    # Empty Valkey PVC first so the Deployment can bind (WaitForFirstConsumer).
    if [ -s "$snapdir/dep-pvcs.json" ]; then
      "''${kcn[@]}" apply -f "$snapdir/dep-pvcs.json"
    fi
    # Everything except the Garage StatefulSet, which we apply patched below.
    jq '.items |= map(select((.kind=="StatefulSet" and .metadata.name=="dev-garage") | not))' \
      "$snapdir/dep-workloads.json" | "''${kcn[@]}" apply -f -
    # shellcheck disable=SC2016  # $meta/$data are jq --arg vars, not shell
    jq --arg meta "$ghost/meta" --arg data "$ghost/data" '
      .spec.template.spec.volumes |= map(
        if .name=="meta" then {name:"meta", hostPath:{path:$meta, type:"DirectoryOrCreate"}}
        elif .name=="data" then {name:"data", hostPath:{path:$data, type:"DirectoryOrCreate"}}
        else . end)' \
      "$snapdir/garage-statefulset.json" | "''${kcn[@]}" apply -f -

    echo "==> [6/8] wait for the CNPG operator"
    "''${kcn[@]}" rollout status deploy/dev-cnpg-cloudnative-pg --timeout=180s

    echo "==> [7/8] CNPG cluster (fresh initdb) + load the migrated DB"
    "''${kcn[@]}" apply -f "$snapdir/cnpg-cluster.json"
    echo "    waiting for the CNPG cluster to be healthy..."
    for _ in $(seq 1 60); do
      ph="$("''${kcn[@]}" get cluster.postgresql.cnpg.io dev-cluster -o jsonpath='{.status.phase}' 2>/dev/null || true)"
      if [ "$ph" = "Cluster in healthy state" ]; then break; fi
      sleep 5
    done
    prim="$("''${kcn[@]}" get cluster.postgresql.cnpg.io dev-cluster -o jsonpath='{.status.currentPrimary}' 2>/dev/null || true)"
    : "''${prim:=dev-cluster-1}"
    "''${kcn[@]}" wait --for=condition=Ready "pod/$prim" --timeout=180s || true
    echo "    loading pg_dumpall into $prim (errors for pre-existing roles are expected)..."
    # ON_ERROR_STOP=0 so pre-existing roles/objects don't abort the load, but
    # DON'T discard everything: capture stderr, count real SQL errors, and show
    # a sample. A large/unexpected count is the signal that the restore didn't
    # actually take (schema mismatch after a chart bump) and the migration job
    # will do real work — better to see it here than debug it two minutes later.
    pglog="$(mktemp)"
    if ! gzip -dc "$snapdir/pg_dumpall.sql.gz" \
        | "''${kcn[@]}" exec -i "$prim" -c postgres -- psql -U postgres -v ON_ERROR_STOP=0 -q >/dev/null 2>"$pglog"; then
      echo "    warn: psql/pipe exited non-zero while loading the dump" >&2
    fi
    errs="$(grep -c 'ERROR:' "$pglog" 2>/dev/null || true)"
    : "''${errs:=0}"
    if [ "$errs" -gt 0 ]; then
      echo "    note: $errs SQL error line(s) during load (pre-existing roles/objects are expected); sample:"
      grep 'ERROR:' "$pglog" | tail -n 3 | sed 's/^/      /' || true
    fi
    rm -f "$pglog"

    # pg_dumpall's globals reset the app-role passwords to the OLD dump values,
    # but CNPG owns the (freshly generated) dev-cluster-* secrets GitLab reads.
    # Resync each role's password to its current CNPG secret so GitLab (and the
    # registry) can authenticate. Without this the migration job fails with
    # "password authentication failed, username: gitlab".
    echo "    resyncing app-role passwords to the CNPG-managed secrets..."
    resync_role() {
      local role="$1" secret="$2" user pw pw_esc
      user="$("''${kcn[@]}" get secret "$secret" -o jsonpath='{.data.username}' 2>/dev/null | base64 -d || true)"
      pw="$("''${kcn[@]}" get secret "$secret" -o jsonpath='{.data.password}' 2>/dev/null | base64 -d || true)"
      : "''${user:=$role}"
      if [ -n "$pw" ]; then
        # Double any single quote so the password can't break out of the SQL
        # string literal (CNPG secrets are base64-random today, but don't rely
        # on that). Identifiers stay double-quoted.
        pw_esc=''${pw//\'/\'\'}
        "''${kcn[@]}" exec -i "$prim" -c postgres -- \
          psql -U postgres -q -c "ALTER ROLE \"$user\" WITH LOGIN PASSWORD '$pw_esc'" || true
      fi
    }
    resync_role gitlab dev-cluster-app
    resync_role registry dev-cluster-registry-app

    echo "==> [8/8] restore CR values file (external-deps.yaml)"
    if [ -f "$snapdir/external-deps.yaml" ]; then
      cp "$snapdir/external-deps.yaml" external-deps.yaml
    fi
    echo "==> restore complete — deps + migrated DB + Garage restored; the CR's migration job should no-op"
  '';
in
{
  inherit restoreDev;
  apps = {
    snapshot-dev = mkApp snapshotDev;
    restore-dev = mkApp restoreDev;
  };
}
