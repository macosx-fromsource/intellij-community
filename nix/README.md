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
| CR overlay | `nix build .#cr-overlay-gateway` / `.#cr-overlay` | — |

Prefer the pure commands; the impure twins exist for the existing workflow and
runtime overrides.

## Front door: Gateway API (default) vs nginx-ingress

`deploy` / `deploy-dev` / `up-dev` default to **Gateway API (Envoy Gateway)** —
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
version requires recreating the cluster (`nix run .#kind-down && nix run .#up-dev`).

## Day-to-day

| Command | Does |
| --- | --- |
| `nix run .#up-dev` | Full clean local env: kind cluster → dev deps → build+load dev image → deploy operator + GitLab CR. |
| `nix run .#refresh-dev` | Inner loop: rebuild image → load into kind → restart the operator. |
| `nix run .#deploy` | Deploy using the **published** manifest (registry image, `:latest`). |
| `nix run .#deploy-dev` | Deploy using the **locally built** image (run `.#load-image-dev` first). |
| `nix run .#kind-up` / `.#kind-down` | Create / delete the kind cluster. |
| `nix run .#deps-dev` | Provision external dev dependencies (writes `external-deps.yaml`). |
| `nix run .#lint` / `.#test` / `.#fmt` / `.#vet` | Quality + tests. |
| `nix run .#generate` / `.#manifests` | controller-gen codegen. |

`nix flake check` renders the manifest + CR overlay offline — the same check CI
can run.

## Re-bootstrapping hashes (Renovate bumps these)

Three hashes are pinned and must be refreshed when their inputs change. The
procedure is the same for all: replace the hash with `pkgs.lib.fakeHash`, run
the build, and paste the `got: sha256-…` value Nix prints back in.

| Hash | Refresh when… | Rebuild with |
| --- | --- | --- |
| `chartDeps.outputHash` | `deploy/chart/Chart.lock` changes | `nix build .#chart-deps` |
| `gitlabCharts.outputHash` | `CHART_VERSIONS` changes | `nix build .#gitlab-charts` |
| `mkManager … vendorHash` | `go.mod` / `go.sum` change | `nix build .#manager` |

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
