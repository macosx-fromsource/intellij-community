---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Tests
---

Retrieve and build the charts before running anything:

```shell
task retrieve-charts
task build_chart
export HELM_CHARTS=$(pwd)/charts
export CHART_VERSION=$(head -n1 CHART_VERSIONS)
```

## Tiers

| Tier | Where | Run it with | Needs |
|---|---|---|---|
| Unit, Ginkgo | `controllers/`, `helm/`, `pkg/` | `task unit-tests` | the charts |
| Unit, testify | `internal/`, `api/` | `task unit-tests`, `task bridge-unit-tests` | nothing |
| Controller, envtest | `controllers/`, `pkg/support/kube` | `task slow-unit-tests` | `KUBEBUILDER_ASSETS` |
| End-to-end, in-process | `internal/**/e2e_test.go` | `task e2e-tests` | a cluster, the v2alpha1 definitions |
| End-to-end, black-box | `test/e2e/` | `task e2e-suite` | a cluster, a built `-bridge` image |

Run one package with `task unit-tests TEST_PKGS="./controllers/gitlab/..."`, and one
test with `SKIP_ENVTEST=yes go run github.com/onsi/ginkgo/v2/ginkgo --focus "..."`.

## Why the black-box suites deploy an image

The v2alpha1 reconcilers are gated twice: the `bridge` build tag keeps them out of
the default build, and `ENABLE_BRIDGE` keeps them off at runtime even in the tagged
build. Their RBAC is not in `deploy/chart` either — it is
`config/rbac/v2alpha1_manager_role.yaml`, applied by `task install_v2alpha1_crds`.

A test that constructs a reconciler in its own process uses none of the three. It
runs under the ambient kubeconfig of whoever ran it, which is usually an
administrator, so it passes just as happily against an image built from the wrong
Dockerfile, a release installed with the bridge off, or a manager that may not write
the status of what it reconciles. `test/e2e` deploys the image and drives it through
the API server so that all three have to be right, and it fails with a different
message for each. See [test/e2e/README.md](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/test/e2e/README.md).

The in-process tests under `internal/` are still worth having: they are far faster,
and they can reach a reconciler directly. They are the tier below, not the same one.

## Why `test/e2e` carries no build tag

The in-process end-to-end tests live in the same package as the code they test, so a
`//go:build e2e` tag is the only thing that keeps them out of `task unit-tests`.
`test/e2e` is a directory of its own, so it needs no tag: `TestE2E` skips unless
`E2E=true`, and `unit-tests` excludes the package by name.

That is deliberate rather than incidental. A tagged file is invisible to gopls unless
every editor sets its build flags, and invisible to `golangci-lint` unless the tag is
on its command line, so tagged code rots quietly. Untagged, `go build ./...`,
`go vet ./...` and the default lint run all cover the harness.
