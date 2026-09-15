# Nix workflow

The root flake is an **additive** layer: a reproducible dev shell plus
build/test/deploy commands that wrap the existing scripts, Taskfile, and CI —
none of which change. Enter the shell with `nix develop` (or `direnv` via
`.envrc`); every command also works standalone, e.g. `nix run .#test`.

The flake is split into a **pure** half (cacheable, cluster-free, validated by
`nix flake check`) and an **impure** half (effects against a live cluster, run
as thin `nix run .#…` shell apps).

## Which command do I use?

| Goal | Pure (cacheable, no cluster) | Impure / local-state |
| --- | --- | --- |
| Operator manifest | `nix build .#operator-manifest` (+ `.#operator-manifest-dev`) | `nix run .#build-operator` (env-override escape hatch → `.build/operator.yaml`) |
| Operator binary | `nix build .#manager` | `nix run .#build` (→ `./bin/manager`) |
| GitLab charts | `nix build .#gitlab-charts` | `nix run .#retrieve-charts` (→ `./charts`, for the Taskfile/test flow) |
| Operator image | `nix build .#image` (a **streamer script**, not a tarball — run it to emit the archive) | `nix run .#load-image-dev` (streams it into kind) |
| Bridge image (opt-in) | `nix build .#image-bridge` (streamer; also `.#bridge-web`, `.#manager-bridge`, `.#operator-manifest-bridge`) | `nix run .#load-image-bridge` |
| CR overlay | `nix build .#cr-overlay-gateway` / `.#cr-overlay` | — |

Prefer the pure commands; the impure twins exist for the existing workflow and
runtime overrides.

## Front door: Gateway API (default) vs nginx-ingress

`deploy` / `deploy-dev` / `up` default to **Gateway API (Envoy Gateway)** —
the chart's own default from 10.1.x — via `cr-overlay-gateway`. Set
**`NGINX_INGRESS=1`** for the classic Ingress path (`cr-overlay`, bundled
nginx-ingress). Both overlays are pure/buildable.

The operator manages the Gateway API *resources* but not the Envoy Gateway
*controller* — like cert-manager, that controller is a cluster prerequisite.
The deploy path installs it automatically (`gateway-deps`), and
`nix run .#gateway-deps` installs it standalone. Envoy's `:443` is pinned to
nodePort `32443` so the kind `443->32443` host mapping reaches it.

Reachability note: kind has no cloud LoadBalancer, so Envoy is a NodePort
service. If the host mapping doesn't reach it, port-forward
(`kubectl -n gitlab-system port-forward svc/<envoy-gateway-svc> 4433:443`) or
use `NGINX_INGRESS=1`.

## Bridge (backend-for-frontend)

The bridge is an opt-in HTTP server + embedded SPA (see
[doc/developer/bridge.md](../doc/developer/bridge.md)). It is gated behind the
`bridge` Go build tag, so it is **absent from the default `.#image`** — the
bridge outputs are separate and never change the public build.

A bridge build differs from the default in two ways, both handled by the flake:
the manager is compiled with `-tags bridge`, and the Vue SPA is built and
overlaid into the `//go:embed` dir before compiling. `.#bridge-web` builds the
SPA (offline — `schema.d.ts` is committed), `.#manager-bridge` the tagged
binary, `.#image-bridge` the image (tag `dev-bridge`), and
`.#operator-manifest-bridge` renders the manifest with `bridge.enabled=true`.

| Command | Does |
| --- | --- |
| `BRIDGE=1 nix run .#up` | Full local env, bridge variant: loads `dev-bridge` image + deploys with `bridge.enabled=true`. |
| `nix run .#deploy-bridge` | Deploy the bridge-enabled image (run `.#load-image-bridge` first). |
| `nix run .#bridge-access` | Ensure caller RBAC → mint a token → print the token + UI/docs/API URLs → port-forward `:8090`. |

`bridge-access` is env-overridable (`BRIDGE_SA`, `BRIDGE_ROLE`,
`BRIDGE_TOKEN_DURATION`, `BRIDGE_LOCAL_PORT` / `BRIDGE_REMOTE_PORT`,
`BRIDGE_DEPLOYMENT`) and runs against an already-deployed bridge. The bridge
exposes only a container port (no Service/Ingress), so port-forward is the
intended access path. Paste the printed token (raw, no `Bearer ` prefix) into
the SPA token bar and select **Save token**.

## Versions & overrides

Each value has exactly one home. Runtime config lives in the flake's `cfg`
attrset; the toolchain comes from `mise.toml`; the bundled chart set comes from
`CHART_VERSIONS`.

**Where the env-var defaults come from:** `flake.nix` maps every tunable to its
`cfg` value in one `envMap`, and derives an `envDefaults` shell prelude from it.
Every impure app sources that prelude first (`export FOO="${FOO:-<cfg default>}"`),
so a `cfg` value overrides the fallback baked into
`scripts/provision_and_deploy.sh`, while a user-supplied env var still wins over
both. The env-var name is the exact name the scripts read — one name per concept,
no aliases. `envMap` is the single table to read.

| Component | Source of truth | Env override |
| --- | --- | --- |
| go / helm / kind / golangci-lint / yq | `mise.toml` (via tool2nix) | — (pin in `mise.toml`) |
| kind node image | `cfg.kindNodeImage` | `KIND_IMAGE` |
| cert-manager | `cfg.certManagerVersion` | `CERT_MANAGER_VERSION` |
| Envoy Gateway | `cfg.envoyGatewayVersion` | `ENVOY_GATEWAY_VERSION` |
| kind cluster name | `cfg.kindClusterName` | `KIND_CLUSTER_NAME` |
| target namespace | `cfg.namespace` | `TARGET_NAMESPACE` |
| kubectl context | derived `kind-$KIND_CLUSTER_NAME` | `KUBE_CONTEXT` |
| GitLab TLS secret | `cfg.tlsSecretName` | `GITLAB_TLSCERTNAME` |
| build/scratch dir | `cfg.buildDir` | `BUILD_DIR` |
| kube wait timeout | `cfg.k8sTimeout` | `KUBERNETES_TIMEOUT` |
| GitLab chart versions | `CHART_VERSIONS` (FOD-pinned in `nix/image.nix`) | `GITLAB_CHART_VERSION` (deploy) |
| nodePorts / pages TLS secret | `cfg.{pagesTlsSecretName,httpsNodePort,sshPort}` (must match `scripts/*.tpl` + `provision_and_deploy.sh`) | — |

The kind node must be **k8s ≥ 1.31** (chart 10.1.x Gateway API CRDs use the CEL
`isIP()` function); cert-manager is bumped in lockstep. Changing the node
version requires recreating the cluster (`nix run .#down && nix run .#up`).

## Day-to-day

| Command | Does |
| --- | --- |
| `nix run .#up` | Full clean local env: kind cluster → dev deps (or a restored snapshot on a fresh cluster) → build+load dev image → deploy operator + GitLab CR. Uses the [prebaked node image + snapshot cache](#caching-up-speedup) when present. |
| `nix run .#refresh-dev` | Inner loop: rebuild image → load into kind → restart the operator. |
| `nix run .#deploy` | Deploy using the **published** manifest (registry image, `:latest`). |
| `nix run .#deploy-dev` | Deploy using the **locally built** image (run `.#load-image-dev` first). |
| `nix run .#deploy-bridge` / `.#bridge-access` | Bridge variant — see [Bridge](#bridge-backend-for-frontend). |
| `nix run .#kind-up` / `.#down` | Create / delete the kind cluster. |
| `nix run .#deps-dev` | Provision external dev dependencies (writes `external-deps.yaml`). |
| `nix run .#lint` / `.#test` / `.#fmt` / `.#vet` | Quality + tests. |
| `nix run .#generate` / `.#manifests` | controller-gen codegen. |
| `nix run .#refresh-hashes [go\|npm\|charts\|deps\|all]` | Re-pin the flake's FOD/vendor/npm hashes after their inputs change (see [Re-bootstrapping hashes](#re-bootstrapping-hashes-renovate-bumps-these)). |

`nix flake check` renders the manifest + CR overlay offline — the same check CI
can run.

## Caching (`up` speedup)

The first `up` is slow: it pulls the full set of GitLab + dependency images
over the network, then waits on the external-dep setup (`dev_dependencies.sh`)
and the GitLab migration jobs. Two **opt-in, additive** caches make every later
standup fast. Both are keyed by the canonical chart version, so cached state
always matches what you deploy:

1. **Prebaked kind node image** — a copy of the base node with every cluster
   image already in containerd, committed as `kindest/node-gitlab:<chartver>`.
   A fresh `up` then does **zero** network image pulls. `up` selects it
   automatically (`nodeImageResolver`); set `NO_BAKED_NODE=1` to ignore it.
2. **Post-migration state snapshot** — a logical dump of the migrated CNPG DB +
   Garage object storage + the dep workloads/secrets. On a **freshly created**
   cluster `up` restores this instead of running `dev_dependencies.sh` + the
   migration jobs; set `NO_RESTORE=1` to force the slow path. Restore only runs
   on a cluster `up` just created — never against a live, already-migrated
   GitLab.

### Building + using the caches

```sh
nix run .#up        # 1. first standup (slow — nothing cached yet)
nix run .#warm-cache    # 2. while it's up: capture → prewarm → bake → snapshot
nix run .#down
nix run .#up        # 3. fast standup: 0 image pulls + restored state
```

| Command | Does |
| --- | --- |
| `nix run .#warm-cache` | One shot: `capture-images → prewarm-images → bake-node-image → snapshot-dev`. Run once after the first `up`. |
| `nix run .#capture-images` | Record every image containerd pulled onto the node. |
| `nix run .#prewarm-images` | skopeo-pull that set into the local archive cache (parallelism: `PREWARM_JOBS`, default 6). |
| `nix run .#bake-node-image` | Commit `kindest/node-gitlab:<chartver>` with those images preloaded. |
| `nix run .#snapshot-dev` / `.#restore-dev` | Capture / restore the migrated DB + Garage + deps (restore also runs automatically inside `up`). |
| `nix run .#cache-clean [--node\|--snapshot\|--images\|--all]` | Invalidate caches for the **current** chart version (see below). |

The archive cache lives under `$GITLAB_OPERATOR_CACHE` (default
`~/.cache/gitlab-operator`); the baked node image lives in your container
runtime's image store (docker/podman/nerdctl).

> **⚠️ Disk usage.** These caches are large. The prebaked node image is on the
> order of ~20 GB, and the pulled image-archive cache is a comparable multi-GB
> set (self-contained docker-archives, so shared layers are stored per image and
> can total *more* than the node). Add the per-version DB/Garage snapshot, and
> note that **every distinct chart version you cache adds another full set**.
> Budget on the order of tens of GB and clean up versions you no longer use.

### Clearing the cache

`cache-clean` operates on the **current** chart version only (set
`GITLAB_CHART_VERSION` to target another). With no flags it drops just the
derived caches (baked node + snapshot) and keeps the expensive image archives:

```sh
nix run .#cache-clean            # baked node + snapshot for this version
nix run .#cache-clean -- --all   # + the pulled image archives for this version
```

`nix run` needs `--` before app flags — `nix run .#cache-clean -- --all`, not
`--all`.

To **completely** wipe every cache across **all** chart versions (full reset):

```sh
# 1. archives + snapshots + image lists (all versions)
rm -rf "${GITLAB_OPERATOR_CACHE:-$HOME/.cache/gitlab-operator}"
# 2. every baked node image (swap docker for podman/nerdctl if that's your runtime)
docker rmi $(docker images 'kindest/node-gitlab' -q)
```

## Re-bootstrapping hashes (Renovate bumps these)

Four hashes are pinned and must be refreshed when their inputs change.
`nix run .#refresh-hashes [go|npm|charts|deps|all]` (default: `go charts deps`)
automates it — for each target it swaps the hash for the `fakeHash` sentinel,
builds the attr so Nix reports the real `got: sha256-…`, and patches it back
into `nix/image.nix` / `nix/manifests.nix`. To refresh one by hand, follow the
same procedure: replace the hash with `pkgs.lib.fakeHash`, run the build, and
paste the `got: sha256-…` value Nix prints back in.

| Hash | Refresh when… | Rebuild with |
| --- | --- | --- |
| `chartDeps.outputHash` | `deploy/chart/Chart.lock` changes | `nix build .#chart-deps` |
| `gitlabCharts.outputHash` | `CHART_VERSIONS` changes | `nix build .#gitlab-charts` |
| `mkManager … vendorHash` | `go.mod` / `go.sum` change | `nix build .#manager` |
| `bridgeWeb … npmDepsHash` | `internal/bridge/web/package-lock.json` changes | `nix build .#bridge-web` |

`vendorHash` is shared by `.#manager` and `.#manager-bridge`: `go mod vendor`
pulls the bridge's imports (Huma) in via `internal/bridge` regardless of the
build tag, so both vendor the same tree. `npmDepsHash` uses
`nix run nixpkgs#prefetch-npm-deps -- internal/bridge/web/package-lock.json`
rather than the `fakeHash` procedure.

## Tool versions (mise + tool2nix)

Both the dev shell and the build/test/deploy operations resolve their tools from
`mise.toml` via `tool2nix` (`packagesAttrsFrom`), so they share one set —
including the `helm` used by the pure manifest render.

Caveats:
- `tool2nix` maps mise tools to **nixpkgs** attributes, so versions track
  nixpkgs, not the exact mise pin. Exact-version pinning is a future enhancement.
- `yq` is kept on `pkgs.yq-go`: tool2nix resolves "yq" to the Python yq, but the
  scripts need the Go yq. (In the interactive shell, `yq` is the Python one — use
  `yq -y -i` for in-place edits, or `nix run nixpkgs#yq-go`.)
- `kubectl` is not in `mise.toml`, so it comes from nixpkgs.
