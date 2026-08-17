# GitLab Operator

Kubernetes Operator for GitLab on Kubernetes/OpenShift. Go + [Kubernetes controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) + [Helm SDK](https://github.com/helm/helm).

Module: `gitlab.com/gitlab-org/cloud-native/gitlab-operator`

See [doc/developer/guide.md](doc/developer/guide.md) for full setup and project structure.

## Build & Lint

Uses **Taskfile** (`task`), NOT Make. Run `task --list` for all commands.

```shell
task manager          # build binary
task fmt              # go fmt
task vet              # go vet
task lint             # golangci-lint (see .golangci.yml for config)
task generate         # generate CRD types
task manifests        # generate CRD/webhook manifests
```

## Tests

Ginkgo v2 + Gomega.

### Prerequisites

Charts must be retrieved and built before running any tests:

```shell
task retrieve-charts
task build_chart
```

### Required environment variables

Every test invocation requires these two env vars:

```shell
export HELM_CHARTS=$(pwd)/charts
export CHART_VERSION=$(head -n1 CHART_VERSIONS)
```

### Running tests

```shell
# All fast tests
HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS) \
  task unit-tests

# Single package
HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS) \
  task unit-tests TEST_PKGS="./controllers/gitlab/..."

# Single test by name
HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS) SKIP_ENVTEST=yes \
  go run github.com/onsi/ginkgo/v2/ginkgo --focus "test description" ./controllers/gitlab/...

# Controller tests (requires envtest)
HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS) \
  task slow-unit-tests
```

**Other env vars:** `SKIP_ENVTEST=yes` (skip envtest setup for fast tests).

## Code Style

### Imports

Three groups separated by blank lines: (1) stdlib, (2) third-party, (3) local.
Enforced by `goimports` with local prefix `gitlab.com/gitlab-org/cloud-native/gitlab-operator`.

**Standard aliases:** `ctrl` → controller-runtime, `appsv1`/`corev1`/`batchv1`/`metav1` → k8s.io/API,
`apiv1beta1` → operator API, `gitlabctl` → controllers/GitLab, `rt` → pkg/runtime, `feature` → pkg/GitLab/features.

### Naming

- Files: `snake_case.go`, tests in same package (`_test.go` suffix, not `_test` package)
- Types: PascalCase; constants: PascalCase (exported), camelCase (unexported)
- Component/Kind names: constants in `controllers/gitlab/utils.go`

### Error Handling

- Return errors up; don't log-and-return
- Reconciler: `requeue(err)`, `requeueWithDefaultDelay()`, `doNotRequeue()`
- Use `errors.IsNotFound(err)` for missing K8s resources

### Controller Patterns

- One file per component in `controllers/gitlab/` exporting template query functions
- Main reconciler calls `r.createOrPatch(ctx, obj, adapter)`
- `adapter.WantsComponent(component.X)` / `adapter.WantsFeature(feature.X)` for conditionals

### Test Patterns

BDD-style with dot-imported ginkgo/gomega. Each package has `suite_test.go`.

```go
var _ = Describe("Component", func() {
    When("condition", func() {
        chartValues := support.Values{}
        _ = chartValues.SetValue("key", value)
        mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
        adapter := CreateMockAdapter(mockGitLab)
        template, err := GetTemplate(adapter)

        It("does something", func() {
            Expect(err).To(BeNil())
        })
    })
})
```

### Lint Rules (from `.golangci.yml`)

- **wsl_v5:** blank lines between control structures, no cuddled declarations
- **godot:** doc comments must end with a period
- **Excluded paths:** `api/`, `controllers/runner/`

### Logging

`logr.Logger` with structured key-value pairs: `log.Info("msg", "key", val)`. Use `log.V(1)`/`log.V(2)` for debug.

## Bridge (Backend for Frontend)

`internal/bridge/` hosts a bridge (backend-for-frontend) HTTP server that exposes CRUD over the
`GitLabCore` CR (`apps.gitlab.com/v2alpha1`) so a SPA can configure GitLab instances.

- **Stack:** [Huma](https://github.com/danielgtaylor/huma) (code-first, `net/http` via the
  `humago` adapter) emits **OpenAPI 3.1**; the TypeScript client is generated with
  **openapi-typescript** + **openapi-fetch**.
- **Why Huma:** it reflects the existing kubebuilder CR Go types, keeping a single source of
  truth aligned with the CRD (no second schema, unlike a proto-first approach).
- **Runtime:** disabled by default; enable with `ENABLE_BRIDGE=true` (chart: `bridge.enabled`).
  Registered via `mgr.Add` in [cmd/manager/main.go](cmd/manager/main.go) as a non-leader-elected `manager.Runnable`;
  reuses `mgr.GetClient()`. Binds `BRIDGE_BIND_ADDRESS` (default `:8090`, set in
  [controllers/settings/settings.go](controllers/settings/settings.go)). Logs via stdlib `slog`.
- **Resource:** `GitLabCore` only; the deprecated `v1beta1` `GitLab` is a separate definition that
  nothing converts from, so no dual-version mode. The wire type mirrors the spec — `hostname`,
  `edition`, `license.secretRef`, `postgresql`, `redis`, `objectStorage`, `chart.version`/
  `chart.values` — and repeats the CRD constraints as Huma validation tags. Free-form `chart.values` remain the escape
  hatch and win over the values derived from the structured fields (ADR 26); the mapping to chart
  values lives in `internal/controller/gitlabcore/values.go`.
- **Endpoints:** CRUD under `/api/v1[/namespaces/{namespace}]/gitlabs[/{name}]` (`/api/v1` versions
  the bridge API, not the CR), OpenAPI at `/openapi.yaml` (+ `/openapi.json`), docs UI at `/docs`,
  SPA embedded via `go:embed` (`internal/bridge/web/dist`). The chart version the form prefills is
  compiled into the SPA from `CHART_VERSIONS` (Vite `define`), not served.
- **SPA:** `internal/bridge/web/` is a Vue 3 + TypeScript app (Vite, Vue Router, Pinia) built into
  `web/dist`; the operator image builds it in a Node stage. `task frontend-dev` / `frontend-build`;
  details in [internal/bridge/CLAUDE.md](internal/bridge/CLAUDE.md).
- **Regenerate:** `task openapi` writes `internal/bridge/web/openapi.yaml`; `task frontend-client`
  regenerates the TS client.
- **Auth:** caller-identity delegation, like the old Kubernetes Dashboard. Every `/api` request
  must carry `Authorization: Bearer <token>`; the bridge builds a per-request client from that
  token (`rest.AnonymousClientConfig` + `BearerToken`, see
  [internal/bridge/auth.go](internal/bridge/auth.go)), so authn/authz are delegated to the
  kube-apiserver and the caller's own RBAC applies — the operator's service account is never lent
  out. Get a token with `kubectl create token <sa>`; the SPA has a token field and `/docs` an
  Authorize button.
- **`kubectl bridge` plugin:** [cmd/kubectl-bridge](cmd/kubectl-bridge) runs the same server locally
  under the caller's kubeconfig (client-cert/exec-OIDC/token all work, no token to paste).
  `bridge.NewLocalAPI(c, acceptHosts...)` swaps `authMiddleware` for `localClientMiddleware` (one
  fixed client, no bearer check) and marks `index.html` so the SPA hides the token field. It does no
  request auth of its own, so it binds loopback by default and warns otherwise, and
  `localGuardMiddleware` 403s `/api/` requests with an unexpected `Host` (DNS rebinding) or a
  cross-origin `Sec-Fetch-Site`/`Origin`/`Referer`, so a stray browser tab can't drive the cluster
  (`--accept-hosts` widens the host allowlist).
  `task build-kubectl-plugin` / `install-kubectl-plugin`.
- **PoC caveats:** the SPA keeps the token in `localStorage` (XSS-exposed; use short-lived tokens);
  `spec.chart.values` is a free-form object (no schema until the chart's `values.schema.json` is
  wired in), and the structured layer covers only hostname, edition, license, PostgreSQL, Redis, and
  object storage so far. The reconciler defaults `registry.enabled` off, because the registry storage
  has no structured field yet.

## Key Directories

| Directory | Purpose |
|---|---|
| `controllers/` | v1beta1 GitLab reconciler (**deprecated**, frozen) |
| `controllers/gitlab/` | Per-component reconciler helpers (v1beta1) |
| `helm/` | Helm chart templating for v1beta1 (**deprecated**, frozen) |
| `internal/controller/` | v2alpha1 controllers, one package per resource (wired in behind the `bridge` build tag) |
| `internal/render/` | Helm rendering for v2 resources (use this for new code) |
| `pkg/gitlab/` | Adapter abstraction |
| `pkg/support/` | Utilities (values, secrets, charts, kube) |
| `internal/bridge/` | Bridge (backend-for-frontend) HTTP API + embedded SPA |
| `config/` | CRDs, RBAC, webhooks |
| `doc/developer/` | Developer docs |

Tool versions pinned in `mise.toml` (Go 1.26.0, golangci-lint 2.9.0, Helm 4.1.1, task 3.42.1).

## Releases

Semver versioning. Tags trigger CI release pipelines including Red Hat certification.
See [doc/developer/releases.md](doc/developer/releases.md) for versioning, retagging, and certification details.
