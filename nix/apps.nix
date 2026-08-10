# Flake apps (impure commands + dev workflow). Returns the `apps` attrset.
# All build artifacts (manifests, charts, image) and the tool groups are
# passed in from flake.nix so this module stays pure assembly of commands.
{
  pkgs,
  lib,
  cfg,
  devImageName,
  envDefaults,
  kindK8sVersion,
  goTools,
  helmTools,
  clusterTools,
  mkScript,
  mkApp,
  mkOperatorSetFlags,
  operatorManifest,
  operatorManifestDev,
  operatorManifestBridge,
  crOverlay,
  crOverlayGateway,
  gitlabCharts,
  image,
  imageBridge,
}:
let
  # --- codegen / quality / test ----------------------------------------
  generate = mkScript "generate" goTools ''
    controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
  '';
  manifests = mkScript "manifests" goTools ''
    controller-gen rbac:roleName=manager-role crd webhook paths="./..." \
      output:crd:artifacts:config=config/crd/bases
  '';
  fmt = mkScript "fmt" goTools "go fmt ./...";
  vet = mkScript "vet" goTools "go vet ./...";
  lint = mkScript "lint" goTools ''
    golangci-lint run \
      --output.text.path=stdout \
      --output.text.print-issued-lines=false \
      --output.code-climate.path=gl-code-quality-report.json
  '';
  # Impure local build into ./bin. For a reproducible/cacheable binary use
  # `nix build .#manager` instead.
  build = mkScript "build" goTools ''
    mkdir -p bin
    go build -o bin/manager ./cmd/manager
  '';
  # Impure fetch into ./charts (for the existing Taskfile/test workflow).
  # For a pure, hash-pinned chart set use `nix build .#gitlab-charts`.
  retrieveCharts = mkScript "retrieve-charts" helmTools ''
    helm repo list 2>/dev/null | grep -q '^gitlab' \
      || helm repo add gitlab https://charts.gitlab.io/
    helm repo update gitlab
    rm -rf charts && mkdir charts
    while read -r version || [ -n "$version" ]; do
      [ -n "$version" ] || continue
      echo "Fetching gitlab/gitlab-$version"
      helm fetch gitlab/gitlab --version "$version" --destination ./charts/
    done < CHART_VERSIONS
  '';

  test = mkScript "test" (goTools ++ helmTools) ''
    CHART_VERSION="''${CHART_VERSION:-$(head -n1 CHART_VERSIONS)}"
    SKIP_ENVTEST="''${SKIP_ENVTEST:-yes}"
    # Default to the pure gitlabCharts derivation (no impure retrieve-charts
    # step needed); override HELM_CHARTS to point at your own checkout.
    HELM_CHARTS="''${HELM_CHARTS:-${gitlabCharts}}"
    export CHART_VERSION SKIP_ENVTEST HELM_CHARTS
    if [ -z "$(ls -A "$HELM_CHARTS"/*.tgz 2>/dev/null)" ]; then
      echo "No charts in $HELM_CHARTS — set HELM_CHARTS or run 'nix run .#retrieve-charts'" >&2
      exit 1
    fi
    if [ "$SKIP_ENVTEST" != "yes" ]; then
      # Pin the envtest assets to the kind node's k8s minor (single-sourced
      # from cfg.kindNodeImage) so they can't drift from the API server the
      # operator targets. Override with ENVTEST_K8S_VERSION if needed.
      ENVTEST_K8S_VERSION="''${ENVTEST_K8S_VERSION:-${kindK8sVersion}}"
      KUBEBUILDER_ASSETS="$(setup-envtest use "$ENVTEST_K8S_VERSION" -p path)"
      export KUBEBUILDER_ASSETS
    fi
    if [ "$#" -eq 0 ]; then set -- ./...; fi
    go run github.com/onsi/ginkgo/v2/ginkgo -cover -output-dir=coverage "$@"
  '';

  # ESCAPE HATCH. Prefer the pure `nix build .#operator-manifest` (and its
  # `-dev` variant), which bake the cfg defaults and are cacheable/checked.
  # Use this only when you need non-default image/tag/registry overrides at
  # runtime via env vars. Writes to .build/operator.yaml.
  buildOperator = mkScript "build-operator" helmTools ''
    ${envDefaults}
    NAME_OVERRIDE="''${NAME_OVERRIDE:-${cfg.nameOverride}}"
    IMG_REGISTRY="''${IMG_REGISTRY:-${cfg.image.registry}}"
    IMG_REPOSITORY="''${IMG_REPOSITORY:-${cfg.image.repository}}"
    IMG_NAME="''${IMG_NAME:-${cfg.image.name}}"
    TAG="''${TAG:-${cfg.image.tag}}"
    CLUSTER_MODE="''${CLUSTER_MODE:-${lib.boolToString cfg.clusterMode}}"

    mkdir -p "$BUILD_DIR"
    helm dependency build deploy/chart
    helm template deploy/chart --include-crds \
      --namespace "$TARGET_NAMESPACE" \
      ${mkOperatorSetFlags {
        nameOverride = ''"$NAME_OVERRIDE"'';
        registry = ''"$IMG_REGISTRY"'';
        repository = ''"$IMG_REPOSITORY"'';
        name = ''"$IMG_NAME"'';
        tag = ''"$TAG"'';
        clusterMode = ''"$CLUSTER_MODE"'';
      }} \
      > "$BUILD_DIR/operator.yaml"
    echo "wrote $BUILD_DIR/operator.yaml"
  '';

  kindUp = mkScript "kind-up" clusterTools ''
    ${envDefaults}
    exec bash ./scripts/provision_and_deploy.sh create_kind_cluster
  '';
  kindDown = mkScript "kind-down" clusterTools ''
    ${envDefaults}
    exec kind delete cluster --name "$KIND_CLUSTER_NAME"
  '';
  devDeps = mkScript "deps-dev" (clusterTools ++ [ pkgs.yq-go ]) ''
    bash ./scripts/dev_dependencies.sh setup
    # Chart 10.x consolidated object storage requires each item (incl. pages) to
    # use a `bucket` with an EMPTY connection; dev_dependencies.sh emits the older
    # `connection` form for pages, which the chart rejects at render time. Rewrite
    # the generated external-deps.yaml to the bucket form (Garage creates the
    # gitlab-pages bucket; the global appConfig connection supplies credentials).
    if [ -f external-deps.yaml ]; then
      yq -i '.spec.chart.values.global.pages.objectStore = {"bucket": "gitlab-pages"}' external-deps.yaml
    fi
  '';

  # Stream the Nix-built operator image into the kind cluster so the dev
  # manifest (pullPolicy=Never) can use it without a registry push.
  mkLoadImage =
    {
      name,
      imageStreamer,
      tag,
    }:
    mkScript name clusterTools ''
      ${envDefaults}
      archive="$(mktemp -t operator-image.XXXXXX.tar)"
      trap 'rm -f "$archive"' EXIT
      echo "Streaming ${devImageName}:${tag} archive..."
      "${imageStreamer}" > "$archive"
      kind load image-archive "$archive" --name "$KIND_CLUSTER_NAME"
      echo "Loaded ${devImageName}:${tag} into kind cluster $KIND_CLUSTER_NAME"
    '';
  loadImage = mkLoadImage {
    name = "load-image-dev";
    imageStreamer = image;
    tag = cfg.devImageTag;
  };
  # Bridge-enabled image (tag dev-bridge). Load it, then `nix run .#deploy-bridge`
  # to deploy the operator with the bridge server enabled. The bridge exposes
  # only a container port (no Service/Ingress); reach it with `kubectl
  # port-forward deploy/gitlab-controller-manager 8090:8090`.
  loadImageBridge = mkLoadImage {
    name = "load-image-bridge";
    imageStreamer = imageBridge;
    tag = "${cfg.devImageTag}-bridge";
  };

  # Full deploy flow. Thin wrapper: exports the Nix-built artifacts (pure
  # manifest + CR overlay) and config defaults, then hands off to the
  # tracked scripts/deploy.sh which sequences the impure cluster steps.
  #
  # Two named variants share one body. The only difference is DEV_IMAGE, which
  # selects the local-image dev manifest in deploy.sh. Exposing them as
  # `deploy` (published image) and `deploy-dev` (local dev image) makes the
  # mode visible at the command line instead of a `DEV_IMAGE=1` env toggle.
  # Envoy Gateway is a cluster prerequisite for the Gateway-API front door: the
  # operator creates the GatewayClass/Gateway/EnvoyProxy CRs but not the
  # controller that programs them (the Gateway otherwise sits "Waiting for
  # controller"). Installing gateway-helm brings the matching CRDs + controller;
  # the stock controllerName matches the chart's GatewayClass. Must run before
  # the operator reconciles, so the Gateway API CRDs exist for its CR apply.
  gatewayDeps = mkScript "gateway-deps" clusterTools ''
    ${envDefaults}
    ns="${cfg.envoyGatewayNamespace}"
    echo "Installing Envoy Gateway $ENVOY_GATEWAY_VERSION (controller + CRDs)..."
    kubectl --context "$KUBE_CONTEXT" get ns "$ns" >/dev/null 2>&1 || kubectl --context "$KUBE_CONTEXT" create ns "$ns"
    # Render + apply rather than `helm install` (its certgen pre-install hook +
    # --wait stalls here). Pull the OCI chart first (its stdout would corrupt the
    # apply pipe), then server-side apply the templated manifest. Clear any prior
    # certgen Job so its immutable fields don't clash on re-apply.
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    helm pull oci://registry-1.docker.io/envoyproxy/gateway-helm \
      --version "$ENVOY_GATEWAY_VERSION" --untar --untardir "$tmp"
    kubectl --context "$KUBE_CONTEXT" -n "$ns" delete job --all --ignore-not-found >/dev/null 2>&1 || true
    helm template envoy-gateway "$tmp/gateway-helm" \
      --namespace "$ns" --include-crds \
      | kubectl --context "$KUBE_CONTEXT" apply --server-side --force-conflicts -f -
    echo "Waiting for the Envoy Gateway controller to be ready..."
    kubectl --context "$KUBE_CONTEXT" -n "$ns" rollout status deploy/envoy-gateway --timeout="$KUBERNETES_TIMEOUT"
  '';

  # `bridge = true` deploys the bridge-enabled dev image (tag dev-bridge, with
  # the chart's bridge.enabled=true). It pins deploy.sh's escape-hatch
  # GITLAB_OPERATOR_MANIFEST to the bridge manifest — the fully-rendered
  # manifest, no further selection needed — so we do NOT also set DEV_IMAGE
  # (which would only select a manifest, and only when GITLAB_OPERATOR_MANIFEST
  # is unset). Keeping them mutually exclusive avoids relying on deploy.sh's
  # internal precedence. Load the image first with `nix run .#load-image-bridge`.
  mkDeploy =
    {
      dev,
      bridge ? false,
    }:
    mkScript
      (
        if bridge then
          "deploy-bridge"
        else if dev then
          "deploy-dev"
        else
          "deploy"
      )
      (
        clusterTools
        ++ [
          pkgs.gawk
          pkgs.yq-go
        ]
      )
      ''
        ${envDefaults}
        export OPERATOR_MANIFEST="${operatorManifest}"
        export OPERATOR_MANIFEST_DEV="${operatorManifestDev}"
        ${lib.optionalString bridge ''
          export GITLAB_OPERATOR_MANIFEST="${operatorManifestBridge}"
          echo "Deploying the bridge-enabled operator image (${devImageName}:${cfg.devImageTag}-bridge)."
        ''}
        # Front door defaults to Gateway API (Envoy Gateway) — the chart's own
        # 10.1.0 default. Set NGINX_INGRESS=1 to fall back to classic Ingress
        # (bundled nginx-ingress), e.g. for the host:443->32443 nodePort path.
        if [ -n "''${NGINX_INGRESS:-}" ]; then
          export CR_OVERLAY="${crOverlay}"
          echo "NGINX_INGRESS set — deploying with classic Ingress (nginx-ingress)."
        else
          export CR_OVERLAY="${crOverlayGateway}"
          echo "Deploying with Gateway API (Envoy Gateway); set NGINX_INGRESS=1 for nginx-ingress."
          echo "==> ensuring Gateway API + Envoy Gateway CRDs"
          ${lib.getExe gatewayDeps}
        fi
        export DEV_IMAGE_NAME="${devImageName}"
        export DEV_IMAGE_TAG="${cfg.devImageTag}${lib.optionalString bridge "-bridge"}"
        ${lib.optionalString (dev && !bridge) "export DEV_IMAGE=1"}
        exec bash ./scripts/deploy.sh
      '';
  deploy = mkDeploy { dev = false; };
  deployDev = mkDeploy { dev = true; };
  deployBridge = mkDeploy {
    dev = true;
    bridge = true;
  };

  # One command to reach a deployed bridge (see deploy-bridge / up-dev BRIDGE=1):
  # ensure a caller service account with CRUD RBAC on gitlabs.apps.gitlab.com,
  # mint a short-lived token (the bridge delegates to the caller's identity —
  # see doc/developer/bridge.md), print it with the UI/docs/API URLs, then
  # port-forward the manager's bridge port. Everything is env-overridable.
  # RBAC applies are idempotent (create piped through apply). Ctrl-C stops the
  # forward. Runs against an already-deployed bridge; it does not deploy one.
  bridgeAccess = mkScript "bridge-access" clusterTools ''
    ${envDefaults}
    SA="''${BRIDGE_SA:-bridge-user}"
    ROLE="''${BRIDGE_ROLE:-gitlab-editor}"
    DURATION="''${BRIDGE_TOKEN_DURATION:-1h}"
    LOCAL_PORT="''${BRIDGE_LOCAL_PORT:-8090}"
    REMOTE_PORT="''${BRIDGE_REMOTE_PORT:-8090}"
    DEPLOYMENT="''${BRIDGE_DEPLOYMENT:-deploy/gitlab-controller-manager}"

    kc=(kubectl --context "$KUBE_CONTEXT")

    echo "==> ensuring RBAC ($SA / $ROLE) on gitlabs.apps.gitlab.com in $TARGET_NAMESPACE"
    "''${kc[@]}" -n "$TARGET_NAMESPACE" create serviceaccount "$SA" \
      --dry-run=client -o yaml | "''${kc[@]}" apply -f -
    "''${kc[@]}" create clusterrole "$ROLE" \
      --verb=get,list,watch,create,update,patch,delete \
      --resource=gitlabs.apps.gitlab.com \
      --dry-run=client -o yaml | "''${kc[@]}" apply -f -
    "''${kc[@]}" create clusterrolebinding "$SA" \
      --clusterrole="$ROLE" --serviceaccount="$TARGET_NAMESPACE:$SA" \
      --dry-run=client -o yaml | "''${kc[@]}" apply -f -

    echo "==> minting token for $SA (duration $DURATION)"
    TOKEN="$("''${kc[@]}" -n "$TARGET_NAMESPACE" create token "$SA" --duration="$DURATION")"

    printf '\nBridge token (send as: Authorization: Bearer <token>):\n\n%s\n\n' "$TOKEN"
    printf 'UI:   http://localhost:%s/\n' "$LOCAL_PORT"
    printf 'Docs: http://localhost:%s/docs\n' "$LOCAL_PORT"
    printf 'API:  curl -H "Authorization: Bearer %s" http://localhost:%s/api/v1/gitlabs\n\n' "$TOKEN" "$LOCAL_PORT"

    echo "==> port-forwarding $DEPLOYMENT $LOCAL_PORT:$REMOTE_PORT (Ctrl-C to stop)"
    exec "''${kc[@]}" -n "$TARGET_NAMESPACE" port-forward "$DEPLOYMENT" "$LOCAL_PORT:$REMOTE_PORT"
  '';

  # Inner dev loop: rebuild the operator image, load it into kind, and
  # restart the operator so it picks up the new image (the dev manifest
  # uses tag=dev + pullPolicy=Never, so a restart is all that's needed).
  devRefresh = mkScript "refresh-dev" clusterTools ''
    ${envDefaults}
    echo "Building + loading the operator image..."
    ${lib.getExe loadImage}
    echo "Restarting the operator to pick up the new image..."
    kubectl --context "$KUBE_CONTEXT" -n "$TARGET_NAMESPACE" \
      rollout restart deployment/gitlab-controller-manager
    kubectl --context "$KUBE_CONTEXT" -n "$TARGET_NAMESPACE" \
      rollout status deployment/gitlab-controller-manager --timeout="$KUBERNETES_TIMEOUT"
  '';

  # One command for a clean local dev environment: kind cluster -> external
  # deps -> build+load the dev image -> deploy operator + GitLab CR using that
  # local image. Each step is an existing app, composed here. The cluster must
  # come first: dev-deps installs into the cluster (kubectl/helm) and relies on
  # the current context, which `kind create cluster` points at the new cluster.
  #
  # Set BRIDGE=1 to bring up the bridge-enabled variant instead: it loads the
  # dev-bridge image and deploys with bridge.enabled=true. Reach the bridge
  # afterwards with `nix run .#bridge-access` (mints a token + port-forwards).
  up = mkScript "up-dev" clusterTools ''
    ${envDefaults}
    echo "==> kind cluster"
    if kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER_NAME"; then
      echo "kind cluster $KIND_CLUSTER_NAME already exists; skipping creation"
    else
      ${lib.getExe kindUp}
    fi
    echo "==> dev dependencies (external-deps.yaml)"
    ${lib.getExe devDeps}
    if [ -n "''${BRIDGE:-}" ]; then
      echo "==> build + load bridge operator image"
      ${lib.getExe loadImageBridge}
      echo "==> deploy operator (bridge enabled) + GitLab CR"
      ${lib.getExe deployBridge}
      echo "==> bridge is enabled — run 'nix run .#bridge-access' to mint a token and port-forward"
    else
      echo "==> build + load operator image"
      ${lib.getExe loadImage}
      echo "==> deploy operator + GitLab CR (local dev image)"
      ${lib.getExe deployDev}
    fi
  '';
in
{
  apps = {
    generate = mkApp generate;
    manifests = mkApp manifests;
    fmt = mkApp fmt;
    vet = mkApp vet;
    lint = mkApp lint;
    build = mkApp build;
    test = mkApp test;
    retrieve-charts = mkApp retrieveCharts;
    build-operator = mkApp buildOperator;
    deploy = mkApp deploy;
    deploy-dev = mkApp deployDev;
    deploy-bridge = mkApp deployBridge;
    bridge-access = mkApp bridgeAccess;
    refresh-dev = mkApp devRefresh;
    up-dev = mkApp up;
    kind-up = mkApp kindUp;
    kind-down = mkApp kindDown;
    deps-dev = mkApp devDeps;
    gateway-deps = mkApp gatewayDeps;
    load-image-dev = mkApp loadImage;
    load-image-bridge = mkApp loadImageBridge;
  };
}
