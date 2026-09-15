# Black-box end-to-end suites

These deploy the Operator as an image into a cluster and drive it through the API
server. Nothing here runs a reconciler in the test process, so a suite exercises
the `bridge` build tag, the `ENABLE_BRIDGE` flag, and the RBAC a release really
grants — three things an in-process test cannot get wrong because it never uses
them.

For the in-process tests that live beside the code they test, see `task e2e-tests`.

## Run them

```shell
task e2e-suites                      # list the suites
task e2e-suite-local SUITE=operator  # build the image, then run one suite
task e2e-suite SUITE=siphon          # run one suite against an image already built
task e2e-suite                       # run all of them
```

The default is the cluster of the current kubeconfig context, and an Operator
release already installed there is **adopted, not replaced**. Against a disposable
cluster instead:

```shell
E2E_CLUSTER_PROVIDER=k3s task e2e-suite SUITE=siphon
```

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `E2E` | unset | Must be `true`, or `TestE2E` skips. `task e2e-suite` sets it. |
| `E2E_STRICT` | `0` | Turn every skip into a failure. CI sets it, so a job whose requirements went unsatisfied is red rather than an empty green. |
| `E2E_CLUSTER_PROVIDER` | `existing` | `existing` for the current kubeconfig, `k3s` for a disposable container. |
| `E2E_KUBE_CONTEXT` | current | Context to use, for `existing`. |
| `E2E_K3S_IMAGE` | `rancher/k3s:v1.36.1-k3s1` | Node image, for `k3s`. |
| `E2E_K3S_PLATFORM` | the host platform | Platform to import the image as, for example `linux/amd64`. |
| `E2E_KEEP_CLUSTER` | `0` | Leave the k3s container running. |
| `E2E_K3D_CLUSTER` | unset | k3d cluster name. Set it and the harness loads the image with `k3d image import`. |
| `E2E_IMAGE_LOAD_CMD` | unset | Arbitrary load command, run through `sh -c` with `IMAGE` in the environment. Takes precedence over `E2E_K3D_CLUSTER`. |
| `E2E_OPERATOR_IMAGE` | the `-bridge` tag the Taskfile builds | Image under test. Must be `registry/repository/name:tag`, because that is what `deploy/chart` assembles; a digest reference is rejected. |
| `E2E_OPERATOR_NAMESPACE` | `gitlab-system` | Where the Operator goes. |
| `E2E_NAME_OVERRIDE` | `gitlab` | The chart's `nameOverride`, which every derived name follows. |
| `E2E_OPERATOR_REUSE` | `1` on `existing`, always `0` on `k3s` | Adopt an installed release instead of upgrading over it. |
| `E2E_ADDONS` | `install` | `skip` to only probe, and skip the suite when an add-on is absent. For a shared cluster the harness must not write cluster-scoped objects into. |
| `E2E_CERTMANAGER_VERSION` | `1.19.2` | Matches `CERTMANAGER_VERSION` in the Taskfile. |
| `E2E_KEEP_NAMESPACE` | `0` | Leave the per-suite namespaces behind. |
| `E2E_ARTIFACTS_DIR` | `.build/e2e` | Where the failure diagnostics go. Relative paths resolve against the repository root. |
| `E2E_HELM_BINARY` | `helm` | |
| `E2E_SIPHON_GITLABREF` | `0` | Run the `siphon` case that creates a real GitLabCore. Off because the Operator then starts rendering it, which pulls images. |

`HELM_CHARTS` and `CHART_VERSION` are the usual ones, and `task e2e-suite` sets
both. The charts have to be staged: `task retrieve-charts`.

## Failures

When a suite fails, the harness writes what the namespace looked like to
`$E2E_ARTIFACTS_DIR/<test name>/` **before** deleting it: the resources with their
status, the events in time order, every pod log, the Operator log, and the helm
values the Operator was installed with. This is the artifact that explains a
failure; in CI it is uploaded whatever the outcome.

## Adding a suite

1. Create `test/e2e/suite/<name>/<name>.go`.
2. Register it from `init()`:

   ```go
   func init() {
       framework.Register("<name>", "one line, logged when the suite starts", run)
   }

   func run(t *testing.T, env *framework.Env) {
       env.RequireAddons(t, framework.CertManager)
       env.RequireOperator(t)

       t.Run("...", func(t *testing.T) { ... })
   }
   ```

3. Add the blank import to [suite/doc.go](suite/doc.go). A suite that is not
   imported there does not exist.

Ask for what the suite needs with the `Require` methods of `Env` rather than
declaring it: each installs or adopts what it can, skips when it cannot, and is
memoized, so two suites in one run share the work whatever order they ask in.

A suite that is not runnable yet calls `t.Skip` on its first line, naming what it
needs. See [suite/siphonrelease](suite/siphonrelease/siphonrelease.go).

## Notes on the tree

**No build tag.** The gate is the `E2E` variable, not `//go:build e2e`. A tagged
file is invisible to gopls unless every editor sets its build flags, and to
`golangci-lint` unless the tag is on its command line. As a result the default
`golangci-lint run` and `go vet ./...` cover this tree, and `unit-tests` excludes
it by package instead.

**One module.** The harness imports `internal/`, which only works from within the
same module.

**Assertions are literals.** The suites spell out names such as
`siphon_producer_main` rather than importing the constants that produce them. A
black-box test that imports the value it asserts cannot catch a rename, and these
names are the ones a database administrator has to know.

## Provider notes for `k3s`

- The container runs **privileged**. Docker Desktop, OrbStack and a privileged CI
  runner are fine. Rootless Docker or podman needs
  `TESTCONTAINERS_RYUK_PRIVILEGED=true`.
- Loaded images must be `imagePullPolicy: Never`, and the harness always sets it
  explicitly. Left to Kubernetes, a `latest` tag defaults to `Always`, which turns
  a successful load into an `ImagePullBackOff`.
- The image must be built by **the same container daemon testcontainers talks to**.
  `CONTAINER_CLI` in the Taskfile defaults to `podman` while testcontainers reads
  `DOCKER_HOST`, so pick one and stay with it. For podman:
  `export DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')`.
- Apple Silicon works natively. A mixed setup — an `amd64` image on an `arm64`
  node — crash-loops with `exec format error`, which the harness detects and
  reports as the platform mismatch it is rather than as a broken Operator.
- k3s runs with Traefik disabled, and only the API server port is mapped. Reach an
  in-cluster service through a port-forward, not a host port.
