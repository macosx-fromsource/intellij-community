# GitLab Operator

Kubernetes Operator for GitLab on Kubernetes/OpenShift. Go + [Kubernetes controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) + [Helm SDK](https://github.com/helm/helm).

Module: `gitlab.com/gitlab-org/cloud-native/gitlab-operator`

See [doc/developer/guide.md](doc/developer/guide.md) for full setup and project structure.

## Workflow

Follow [doc/developer/workflow.md](doc/developer/workflow.md). The rules you must not skip:

- **Spec before code.** A new or changed capability gets a brief spec (what and why) in
  [doc/specs/](doc/specs/_index.md), and technical choices go in an ADR under
  [doc/developer/adr/](doc/developer/adr/). Keep `doc/specs/` current in the same MR.
- **Meaningful tests.** Cover each requirement with a unit or end-to-end test. Coverage of behavior
  matters, not the number of tests. Bug fixes start with a failing test.
- **Proof of delivery.** Never claim a change is done without showing a passing test or a recorded
  manual run. Docs-only changes and small fixes are exempt; say so.
- **Fresh review.** Before marking an MR ready, have a fresh sub-agent or session compare the
  change with the spec. Record caveats as backlog issues.
- **Labels.** Label every issue and MR you create per
  [doc/developer/labels.md](doc/developer/labels.md): `group::operate`, one `type::`, one subtype.

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

Retrieve and build the charts, and export two variables, before running any test:

```shell
task retrieve-charts
task build_chart
export HELM_CHARTS=$(pwd)/charts
export CHART_VERSION=$(head -n1 CHART_VERSIONS)
```

```shell
task unit-tests                                        # all fast tests
task unit-tests TEST_PKGS="./controllers/gitlab/..."   # one package
task slow-unit-tests                                   # controller tests (envtest)
go test ./internal/controller/siphon/ -run TestName    # one testify test
SKIP_ENVTEST=yes go run github.com/onsi/ginkgo/v2/ginkgo --focus "description" ./controllers/gitlab/...
```

End-to-end tests come in two kinds that are not interchangeable:

- `task e2e-tests`: in-process, `//go:build e2e`, beside the code it tests. Needs a cluster and
  `task install_v2alpha1_crds`.
- `task e2e-suite SUITE=<name>`: black-box, `test/e2e/`, no build tag (gated on `E2E=true`).
  Deploys the bridge image, so it covers the `bridge` tag, `ENABLE_BRIDGE` and the real RBAC.
  `task e2e-suites` lists the suites; `E2E_CLUSTER_PROVIDER=k3s` gets a disposable cluster. New
  suites go in `test/e2e/suite/<name>/`, register from `init()`, and need a blank import in
  `test/e2e/suite/doc.go`.

Tiers: [doc/developer/testing.md](doc/developer/testing.md). E2E variables:
[test/e2e/README.md](test/e2e/README.md).

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

- **New packages:** standard `testing` + testify (`require`/`assert`, `t.Run` subtests). No Ginkgo,
  no `suite_test.go`. Follow `internal/controller/siphon/` or `internal/render/`.
- **Existing Ginkgo packages** (`controllers/`, `helm/`, `pkg/`, `api/`,
  `internal/controller/gitlabcore/`): keep Ginkgo v2 + Gomega, dot-imported, with a
  `suite_test.go` per package.

### Lint Rules (from `.golangci.yml`)

- **wsl_v5:** blank lines between control structures, no cuddled declarations
- **godot:** doc comments must end with a period
- **Excluded paths:** `api/`, `controllers/runner/`

### Logging

`logr.Logger` with structured key-value pairs: `log.Info("msg", "key", val)`. Use `log.V(1)`/`log.V(2)` for debug.

## Bridge

`internal/bridge/` serves an HTTP API over the v2alpha1 `GitLabCore` and `Siphon` resources, plus an
embedded Vue SPA. It is compiled in only with the `bridge` build tag and runs only with
`ENABLE_BRIDGE=true`. Read [internal/bridge/AGENTS.md](internal/bridge/AGENTS.md) before changing
it; setup is in [doc/developer/bridge.md](doc/developer/bridge.md).

## Key Directories

| Directory | Purpose |
|---|---|
| `controllers/` | v1beta1 GitLab reconciler (**deprecated**, frozen) |
| `controllers/gitlab/` | Per-component reconciler helpers (v1beta1) |
| `helm/` | Helm chart templating for v1beta1 (**deprecated**, frozen) |
| `internal/controller/` | v2alpha1 controllers, one package per resource (wired in behind the `bridge` build tag); see [gitlabcore.md](doc/developer/gitlabcore.md), [siphon.md](doc/developer/siphon.md) |
| `internal/render/` | Helm rendering for v2 resources (use this for new code); see [render.md](doc/developer/render.md) |
| `pkg/gitlab/` | Adapter abstraction |
| `pkg/support/` | Utilities (values, secrets, charts, kube) |
| `internal/bridge/` | Bridge (backend-for-frontend) HTTP API + embedded SPA |
| `config/` | CRDs, RBAC, webhooks |
| `test/e2e/` | Black-box end-to-end harness and suites (no build tag; gated on `E2E`) |
| `doc/developer/` | Developer docs |

Tool versions pinned in `mise.toml` (Go 1.26.0, golangci-lint 2.9.0, Helm 4.1.1, task 3.42.1).

## Releases

Semver versioning. Tags trigger CI release pipelines including Red Hat certification.
See [doc/developer/releases.md](doc/developer/releases.md) for versioning, retagging, and certification details.
