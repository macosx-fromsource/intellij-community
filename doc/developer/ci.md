---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: CI
---

## Review environments

`review_ocp` is the only review job with an externally accessible GitLab environment.
It deploys to the shared OpenShift CI cluster and is uninstalled automatically after
1 hour. To keep it up longer, pin the environment on the [Environments page](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/environments).
When you are done, manually trigger the job in the `Cleanup` stage, so that the cluster has enough
resources to run review apps for other merge requests.

The [k3d cluster tests](#k3d-cluster-tests) have no environment and no cleanup job. Each job creates
and destroys its own cluster.

For more information, see the [environments documentation](https://docs.gitlab.com/ci/environments/).

## Token Management

Read about our [IaC managed Project Access Tokens](https://gitlab.com/gitlab-org/distribution/runbooks/-/blob/main/iac-managed-project-access-tokens.md).

## OpenShift CI cluster

We manage one OpenShift cluster in Google Cloud that is used for acceptance tests, including the QA suite.
`review_ocp`, the `qa_ocp` jobs, and `validate_olm_release` run against it.

The kubeconfig file for connecting to the cluster is stored in the 1Password cloud-native vault. Search for `ocp-ci`.

The cluster is orchestrated using the [`openshift-provisioning`](https://gitlab.com/gitlab-org/distribution/infrastructure/openshift-provisioning)
project. CI access is managed using [`kube-agents`](https://gitlab.com/gitlab-org/distribution/infrastructure/kube-agents).

## k3d cluster tests

The `k3d` trigger job in `.gitlab/ci/review-k3d.gitlab-ci.yml` holds a `parallel:matrix` with one row per
tested Kubernetes version. Each row spawns a child pipeline from
`.gitlab/ci/k3d-version-pipeline.gitlab-ci.yml` whose `review_k3d` job creates a single-use
[k3d](https://k3d.io) cluster inside the job's Docker-in-Docker environment. The job deploys the
operator and a GitLab custom resource, runs the QA smoke suite against it over a
[nip.io](https://nip.io) domain, and destroys the cluster when the job ends. The jobs have no
GitLab environment or cleanup job because nothing outlives the job.

These jobs run on the privileged `e2e` runner fleet, except the arm64 row, which sets its own
`RUNNER_TAG` (see [Tested configurations](#tested-configurations)).

The primary version (matrix row with `K3D_PRIMARY: "primary"`) runs automatically on merge request
and branch pipelines. The other versions are manual there, and all versions run automatically on
nightly (scheduled), stable-branch, and default-branch pipelines.

Chart-version pipelines are already at the maximum descendant-pipeline depth, so they run the same
test as the direct `review_k3d_chart_version` job with the same matrix instead of the trigger job.

Each child pipeline also has a manual `qa_k3d_full` job, with one parallel worker per full-suite
shard. Like `review_k3d`, each worker is self-contained: it provisions its own k3d cluster,
operator, and GitLab instance, then runs its shard of the full suite against it. Trigger it from
any merge request or branch pipeline to run the full suite for that Kubernetes version. Nightly
pipelines also run it automatically.

`review_k3d_gitlabcore` is a separate direct job, not a matrix row: it deploys GitLab through the
alpha `apps.gitlab.com/v2alpha1` `GitLabCore` CR (see [GitLabCore](gitlabcore.md)) instead of the
`v1beta1` `GitLab` CR the matrix rows use, against the bridge-tagged operator image
(`build_bridge_image`). Chart-version pipelines never build that image, so this job cannot share
the `k3d_matrix`/`review_k3d_chart_version` pair the other rows do. The `GitLabCore` reconciler
never applies cluster-scoped objects, so the job routes through the Gateway API instead of the
chart-bundled NGINX controller, and `scripts/test.sh` applies the matching `GatewayClass` itself.
Because `gitlab-qa` has never run against a `GitLabCore`-deployed instance, the job sets `SKIP_QA`
and treats `scripts/test.sh` reaching a successful curl of the GitLab endpoint as its only
pass/fail signal.

### Tested configurations

The review jobs cover these combinations of operator scope and Ingress path:

| Job | Kubernetes | Architecture | Operator scope | Ingress path | GitLab CR |
|---|---|---|---|---|---|
| `k3d: [v134]` | 1.34 | amd64 | Cluster-wide | Chart-bundled NGINX controller, HTTP | `v1beta1` |
| `k3d: [v135]` | 1.35 | amd64 | Cluster-wide | Chart-bundled NGINX controller, HTTP | `v1beta1` |
| `k3d: [v136-arm]` | 1.36 | arm64 | Cluster-wide | Chart-bundled NGINX controller, HTTP | `v1beta1` |
| `k3d: [v136]` (primary) | 1.36 | amd64 | Namespaced | In-job Envoy Gateway, chart-rendered GatewayClass, HTTP | `v1beta1` |
| `review_k3d_gitlabcore` | 1.36 | amd64 | Namespaced | In-job Envoy Gateway, chart-rendered GatewayClass, HTTP | `v2alpha1` `GitLabCore` |
| `review_ocp` | OpenShift | amd64 | Cluster-wide | Pre-provisioned external gateway, TLS | `v1beta1` |

The namespaced operator (`CLUSTER_MODE: "false"`) deploys with `nginx-ingress.create=false`, so it
does not create the cluster-scoped resources (IngressClass, RBAC) that the chart-bundled NGINX
controller requires to start. Namespaced rows therefore pair with Gateway API instead. The k3d jobs
serve plain HTTP because a single-use cluster has no pre-provisioned wildcard TLS certificate.

The operator never installs Envoy Gateway. It applies no CustomResourceDefinition and does not deploy
the chart's subchart dependencies, so the Gateway API CRDs and the Envoy Gateway controller must
already be in the cluster. The `v136` row and `review_k3d_gitlabcore` therefore install them in the
job through `INSTALL_ENVOY_GATEWAY: "true"`, which runs `task install_envoy_gateway`. The
`global.gatewayApi.installEnvoy: true` of the `gatewayapi` overlay only makes the chart render the
GatewayClass and EnvoyProxy resources that the reconciler applies.

`review_ocp` covers the opposite arrangement. Its `gatewayapi-external-ocp` overlay sets
`installEnvoy: false`, so the chart renders no GatewayClass and no EnvoyProxy, and the Gateway
attaches to the GatewayClass and OpenShift Gateway API controller that the cluster already provides.

`TEST_CR_FILES_DIR` selects the test CR from `config/test`. `base` routes through the chart-bundled
NGINX controller and is the default, `overlays/gatewayapi` switches to the Gateway API,
`overlays/gatewayapi-external-ocp` adds the OpenShift specifics on top of it, and
`v2alpha1/base` is the `GitLabCore` CR that `review_k3d_gitlabcore` uses.

### Add or remove a Kubernetes version

Edit the `parallel:matrix` in `.gitlab/ci/review-k3d.gitlab-ci.yml`. Each row sets:

- `K3D_K8S_IMAGE`: the [`rancher/k3s`](https://hub.docker.com/r/rancher/k3s/tags) image tag that
  pins the Kubernetes version.
- `K3D_K8S_VERSION_SLUG`: a short label, for example `v135`, used in the child pipeline name.
- `CLUSTER_MODE`: `"true"` for the cluster-wide operator, `"false"` for the namespaced operator.
- Optional. `K3D_PRIMARY: "primary"` on exactly one row: the version that runs automatically on
  merge request pipelines.
- Optional. `TEST_CR_FILES_DIR` and `INSTALL_ENVOY_GATEWAY: "true"` for the Gateway API/Envoy
  variant instead of the default NGINX Ingress overlay.
- Optional. `RUNNER_TAG` to run the row on a runner fleet other than the default `e2e`, for example
  `saas-linux-large-arm64` for the arm64 row.

`review_k3d_gitlabcore` is not part of this matrix (see [k3d cluster tests](#k3d-cluster-tests)), so
adding a Kubernetes version there means editing its job definition directly instead of adding a
matrix row.

## QA pipelines

By default, QA pipelines run the smoke suite, a [small subset of fast end-to-end functional tests](https://docs.gitlab.com/development/testing_guide/smoke/) that quickly checks basic
functionality. To run the full suite of end-to-end tests instead, trigger the manual full-suite job
for the cluster: `qa_k3d_full` for k3d (see [k3d cluster tests](#k3d-cluster-tests)), or
`qa_ocp_full_suite_trigger` followed by `qa_ocp_full` for OpenShift.

To debug failures in tests, please follow [investigate QA failures](https://handbook.gitlab.com/handbook/engineering/testing/distribution/#investigate-qa-failures) guide.

## Container builds

The Operator image can be built for multiple architectures, by configuring a Kubernetes buildx driver using the `BUILDX_K8S_*`
variables. Set the `BUILDX_ARCHS` to a comma-separated string of the target architectures (for example `amd64,arm64`).
If `BUILDX_K8S_DISABLE` is set to `true` - automatically reduces number of platforms to build for down to `amd64`.

If no Kubernetes driver is configured you can (cross-) compile only one architecture.

## DockerHub rate limits

By default, CI uses images from DockerHub. The shared runners by default use a
mirror to avoid hitting DockerHub rate limits. If you use custom runnners, that
don't use caching or mirroring, you should enable the [dependency proxy](https://docs.gitlab.com/user/packages/dependency_proxy/)
by setting the `DOCKERHUB_PREFIX` to your proxy, for example
`DOCKERHUB_PREFIX: ${CI_DEPENDENCY_PROXY_GROUP_IMAGE_PREFIX}`, and
`DEPENDENCY_PROXY_LOGIN="true"`.

The container build context by default uses the gcr DockerHub mirror. This
behavior can be changed by overriding the `DOCKER_OPTIONS` or `DOCKER_MIRROR`
variables.
