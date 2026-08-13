# Bridge (backend-for-frontend)

HTTP server exposing CRUD over the `GitLabCore` CR (`apps.gitlab.com/v2alpha1`), an OpenAPI
document, and an embedded SPA, so a frontend can configure GitLab instances. Runs inside the
operator as a `manager.Runnable`.

See the "Bridge (Backend for Frontend)" section in the root [AGENTS.md](../../AGENTS.md) for the
architecture and stack rationale. This file documents how to work on and locally deploy the bridge.

## Layout

| File | Purpose |
|---|---|
| `server.go` | `Server` (`manager.Runnable`); `NewAPI(cf)` / `NewLocalAPI(client)` build the `humago` mux |
| `handlers.go` | 6 CRUD operations registered with `huma.Register`; K8s→HTTP error mapping |
| `dto.go` | Wire DTOs (`GitLabResource`/`LicenseDTO`/`PostgreSQLDTO`/`RedisDTO`/`SecretRefDTO`/`ChartDTO`/`StatusDTO`) + mappers to/from `apiv2alpha1.GitLabCore` |
| `static.go` + `web/dist/` | `go:embed` SPA serving with client-side-routing fallback |
| `web/` | Vue 3 + TS SPA (Vite, Vue Router, Pinia); see the Frontend section |
| `web/openapi.yaml` | Generated OpenAPI doc (do not hand-edit; run `task openapi`) |
| `web/src/lib/api/` | `client.ts` (openapi-fetch) + generated `schema.d.ts` |

## Regenerating the OpenAPI doc / TS client

```shell
task openapi          # writes internal/bridge/web/openapi.yaml
task frontend-client  # regenerates the TS client from the doc
```

Run `task openapi` whenever handlers/DTOs change; the generator builds the Huma API with a nil
client (spec only), so it never touches a cluster.

## Resource

The bridge drives `GitLabCore` (`apps.gitlab.com/v2alpha1`), not the deprecated `v1beta1` `GitLab`.
`GitLabResource` mirrors the specification: `hostname`, `edition` (`ce`/`ee`), `license.secretRef`,
`postgresql` and `redis` (host + password Secret each), `objectStorage` (one connection Secret), and
`chart.version`/`chart.values`. Only
Secret references travel over the wire, never a license key or a password. The DTO repeats the CRD constraints as Huma
validation tags, so the OpenAPI document carries them and a bad hostname is a 422 from the bridge
rather than a rejection from the API server. Free-form `chart.values` stay the escape hatch for
everything the structured layer does not cover; the reconciler merges them over the values derived
from the structured fields, and they win on conflict (see [ADR 26](../../doc/developer/adr/0026-design-of-v2alpha1-custom-resources.md)).

Nothing converts a `v1beta1` `GitLab` into a `GitLabCore` (separate definitions, separate kinds), so
the bridge serves `GitLabCore` only — there is no dual-version mode. The paths keep the `gitlabs`
segment: `/api/v1` versions the bridge API, not the custom resource.

## Endpoints

- CRUD: `/api/v1/gitlabs` (list all watched ns) and `/api/v1[/namespaces/{namespace}]/gitlabs[/{name}]`
- OpenAPI: `/openapi.yaml`, `/openapi.json` — Docs UI: `/docs`
- SPA: everything else (falls back to `index.html`)
- Bind address: `BRIDGE_BIND_ADDRESS` (default `:8090`), see [controllers/settings/settings.go](../../controllers/settings/settings.go)
- Disabled by default: gated behind `ENABLE_BRIDGE=true` (Helm chart value `bridge.enabled`, default `false`)

## Auth (caller-identity delegation)

Like the old Kubernetes Dashboard: every `/api` request must send `Authorization: Bearer <token>`.
[auth.go](auth.go) holds a huma middleware that extracts the token and builds a per-request client
(`rest.AnonymousClientConfig(base)` + `BearerToken`, sharing one `RESTMapper`); handlers pull it via
`clientFrom(ctx)`. Authn **and** authz are delegated to the kube-apiserver — the caller needs their
own RBAC on `gitlabcores.apps.gitlab.com`; 401/403 from the API server are mapped through in `mapError`.
Non-`/api` routes (`/openapi.*`, `/docs`, SPA) stay open. The SPA attaches the token via an
openapi-fetch middleware ([web/src/lib/api/client.ts](web/src/lib/api/client.ts)) reading a token
held by the `auth` Pinia store ([web/src/stores/auth.ts](web/src/stores/auth.ts)); enter it in the
header token field or `/docs` Authorize.

Get a token for local use (needs k8s ≥ 1.24):

```shell
kubectl -n gitlab-system create serviceaccount bridge-user
kubectl create clusterrole gitlab-editor \
  --verb=get,list,watch,create,update,patch,delete --resource=gitlabcores.apps.gitlab.com
kubectl create clusterrolebinding bridge-user \
  --clusterrole=gitlab-editor --serviceaccount=gitlab-system:bridge-user
TOKEN=$(kubectl -n gitlab-system create token bridge-user --duration=1h)
curl -H "Authorization: Bearer $TOKEN" localhost:8090/api/v1/gitlabs
```

A client-cert or exec/OIDC kubeconfig can't be reduced to a bearer token — use
`kubectl create token <sa>` (or your OIDC id-token) instead, or run the `kubectl bridge` plugin.

## Local serve mode (`kubectl bridge` plugin)

[../../cmd/kubectl-bridge](../../cmd/kubectl-bridge) is a kubectl plugin that runs the same server on
the user's machine. It builds one client from the ambient kubeconfig via `clientcmd` (honoring
`--context`/`--kubeconfig`/`KUBECONFIG`), so client-cert, exec/OIDC and token kubeconfigs all work —
client-go builds the transport, exactly as kubectl does. It uses a private `flag.FlagSet` to avoid
the global `--kubeconfig` flag that transitively-imported k8s libraries register.

Server side this is `NewLocalAPI(c, acceptHosts...)`: same routes, but `localClientMiddleware`
([auth.go](auth.go)) injects that fixed client into `clientCtxKey` for `/api/` requests instead of
`authMiddleware` building one per bearer token, and the Huma config declares no bearer security
scheme (no **Authorize** button in `/docs`). The in-cluster `NewAPI`/`authMiddleware` path is
untouched. There is **no request authentication** in this mode, so the plugin defaults to loopback and
warns (`warnIfNotLoopback`) when bound anywhere else.

Loopback stops other machines, not other browser tabs — with no token to guess, any page the user has
open could `fetch` `http://127.0.0.1:<port>/api/...` and spend their cluster permissions (the class of
issue behind CVE-2020-8558, mitigated in `kubectl proxy` by `--accept-hosts`). `localGuardMiddleware`
([auth.go](auth.go)) runs before `localClientMiddleware` and 403s `/api/` requests that either carry a
`Host` outside the loopback names plus `acceptHosts` (DNS rebinding) or look like a browser fetch for
another origin (`Sec-Fetch-Site` other than `same-origin`/`none`, or an `Origin`/`Referer` authority
differing from `Host`). Requests with none of those headers pass, so `curl` still works. The plugin
feeds `acceptHosts` from `--accept-hosts` plus its bind address (`apiAcceptHosts`).

`registerStatic(mux, localMode)` ([static.go](static.go)) injects
`<script>window.__BRIDGE_AUTH__="local"</script>` before `</head>` of `index.html` when
`localMode` is set. The SPA reads it through
[web/src/lib/authMode.ts](web/src/lib/authMode.ts) and `App.vue` hides the token field, showing a
"Local — kubeconfig identity" indicator instead. Local-mode behavior is covered by
[local_test.go](local_test.go).

```shell
task build-kubectl-plugin     # bin/kubectl-bridge (deps: frontend-build)
task install-kubectl-plugin   # go install -> $GOBIN, else $(go env GOPATH)/bin
kubectl bridge --verbose      # --port/-p, --address, --context, --kubeconfig, --no-open
```

## Testing

Unlike the rest of the repo (Ginkgo/Gomega), the bridge uses Go's standard `testing` package
with **testify** (`require`). `handlers_test.go` drives the real handlers via `httptest` + a fake
client (no cluster needed). The SPA has Vitest specs beside the sources:
`stores/gitlabs.spec.ts` (store) and `views/GitLabFormView.spec.ts` (steps, per-step validation, and
the body the form posts), run with `npm run test:unit`.

```shell
SKIP_ENVTEST=yes go test ./internal/bridge/...
```

## Frontend (Vue 3 + TypeScript SPA)

`web/` is a Vue 3 SPA scaffolded with **create-vue** (Vite + Vue Router + Pinia + ESLint + Prettier
+ Vitest). It talks to the bridge through a typed **openapi-fetch** client whose types are generated
from `openapi.yaml` by **openapi-typescript**. Node is pinned to **26** via [mise.toml](../../mise.toml), matching the
[Dockerfile.bridge](../../Dockerfile.bridge) webbuilder stage;
run commands inside a mise-activated shell (or prefix `mise exec --`).

Structure: `src/lib/api/` (typed client), `src/stores/gitlabs.ts` (Pinia CRUD store),
`src/views/GitLabsListView.vue` + `GitLabFormView.vue`, `src/router/index.ts`. The form edits the
structured fields (hostname, edition, PostgreSQL, Redis, license, chart version) as inputs and
`chart.values` as a YAML textarea (free-form; no schema yet), across three steps — Basics
(name/chart version/hostname/edition/license), Dependencies (PostgreSQL/Valkey/object storage), Overrides (chart
values). The namespace is not a field: creation goes to `gitlab-system` (`defaultNamespace`), and an
edit keeps the namespace of the route. The chart versions are compiled in rather than fetched:
`vite.config.ts` reads `CHART_VERSIONS` into `__CHART_VERSIONS__`, which
[web/src/lib/chartVersions.ts](web/src/lib/chartVersions.ts) exports, and the version field is a
free-form input prefilled with the latest of them. The Docker webbuilder stage copies
`CHART_VERSIONS` next to the SPA; a missing file fails that build rather than shipping an empty
prefill. `validateStep`/`goTo` check a step on the way forward only, and the panels use
`v-show` so the CodeMirror editor of the last step mounts once. The PostgreSQL, Valkey, object storage, and license
groups are all-or-nothing, validated client-side before the request; the license group sits under
the edition select and is hidden (and left out of the request) for `ce`. The Valkey group writes
`spec.redis` — the wire field, the resource, and the chart all keep the Redis name. The data store groups carry the
official project logo from `src/assets/icons/`, and object storage a plain glyph painted through a
CSS mask; see the [README](web/src/assets/icons/README.md) there before touching those files.

```shell
task frontend-install   # npm ci
task frontend-client    # regenerate src/lib/api/schema.d.ts from openapi.yaml
task frontend-build     # build into web/dist (embedded by go:embed)
task frontend-dev       # Vite dev server (HMR) on :5173

# raw npm (inside web/): npm run lint | npm run type-check | npm run test:unit
```

**Dev loop with HMR:** run the bridge (`task run`, or `kubectl -n gitlab-system port-forward
deploy/gitlab-controller-manager 8090:8090`) so `:8090` is reachable, then `task frontend-dev` and
open `http://localhost:5173`. Vite proxies `/api`, `/openapi*`, `/docs`, `/schemas` to `:8090`
(override with `BRIDGE_URL`, see [web/vite.config.ts](web/vite.config.ts)).

## Local build & deploy (OrbStack / local k8s sharing the Docker daemon)

OrbStack's k8s uses the same Docker backend, so an image built locally with the deploy tag is
usable directly — set `imagePullPolicy: IfNotPresent` so k8s does not try to pull from the
registry (the tag is `:latest`, which otherwise defaults to `Always`).

Prerequisites: `kubectl` context pointing at the local cluster (`kubectl config current-context`),
Docker running. This repo defaults `CONTAINER_CLI` to `podman`; override to `docker` locally.

```shell
# 0. cert-manager is REQUIRED (the chart provisions webhook certs via a self-signed
#    Certificate/Issuer, manager.webhook.selfSignedCert.create=true). Install once:
helm upgrade --install cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --version 1.19.2 -n cert-manager --create-namespace --set crds.enabled=true
kubectl wait --for=condition=Available deploy/cert-manager-webhook -n cert-manager --timeout=180s

# 1. Build the image with the deploy tag (charts must already be present; run
#    `task retrieve-charts` first if charts/ is empty).
mkdir -p .go/pkg/mod
docker build . -t registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator:latest
# equivalently: CONTAINER_CLI=docker task docker-build

# 2. Deploy the operator, forcing use of the local image.
export HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS)
CONTAINER_CLI=docker ARGS='--set image.pullPolicy=IfNotPresent' task deploy_operator

# 3. Verify the pod runs the local image and the bridge started.
kubectl -n gitlab-system rollout status deploy/gitlab-controller-manager --timeout=120s
kubectl -n gitlab-system logs deploy/gitlab-controller-manager | grep bridge
#   -> "starting bridge server","addr":":8090"

# 4. Smoke-test the API.
kubectl -n gitlab-system port-forward deploy/gitlab-controller-manager 8090:8090 &
curl -s localhost:8090/api/v1/gitlabs                 # -> {"items":[]}
curl -s localhost:8090/openapi.yaml | grep title      # -> GitLab Operator Bridge
```

### Iterating

After code changes, rebuild the image (same tag) and restart the deployment to pick it up
(`pullPolicy: IfNotPresent` keeps using the local image):

```shell
docker build . -t registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator:latest
kubectl -n gitlab-system rollout restart deploy/gitlab-controller-manager
```

### Gotchas

- **Dockerfile must `COPY internal/`** — the builder stage copies source dirs explicitly;
  `cmd/manager` imports `internal/bridge`, so omitting it breaks the image build (and would break CI/registry
  builds too, not just local).
- **SPA build happens in the image** — the Dockerfile has a `webbuilder` Node stage that runs
  `npm ci && npm run build` and copies `web/dist` into the Go builder before `go build`, so
  `docker build` is self-contained (no host pre-build needed). Local `go build` relies on the
  committed `web/dist/.gitkeep` to satisfy `go:embed`, but you must `task frontend-build` to get a
  real UI (built assets are gitignored). `internal/bridge/web` is excluded in `.golangci.yml`/
  `go` tooling concerns via `.dockerignore`/lint excludes so `node_modules` Go files don't leak in.
- **cert-manager first** — without it the Helm install fails on the `Certificate`/`Issuer` CRs and
  the validating webhook has no cert, so CR writes (incl. via the bridge) are rejected.
- **`CONTAINER_CLI=docker`** — the Taskfile defaults to `podman`; the `docker-build`/`docker-push`
  tasks need the override on machines without podman.
