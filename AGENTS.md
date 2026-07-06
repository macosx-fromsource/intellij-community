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

## Key Directories

| Directory | Purpose |
|---|---|
| `controllers/gitlab/` | Per-component reconciler helpers |
| `pkg/gitlab/` | Adapter abstraction |
| `pkg/support/` | Utilities (values, secrets, charts, kube) |
| `helm/` | Helm chart templating |
| `config/` | CRDs, RBAC, webhooks |
| `doc/developer/` | Developer docs |

Tool versions pinned in `mise.toml` (Go 1.26.0, golangci-lint 2.9.0, Helm 4.1.1, task 3.42.1).

## Releases

Semver versioning. Tags trigger CI release pipelines including Red Hat certification.
See [doc/developer/releases.md](doc/developer/releases.md) for versioning, retagging, and certification details.
