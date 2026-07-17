---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: CI
---

## Review environments

The review environments are automatically uninstalled after 1 hour. If you need the review environment to stay up longer, you can pin the environment
on the [Environments page](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/environments). However, make sure to manually trigger the jobs
in the `Cleanup` stage when you're done. This helps to ensure that the clusters have enough resources to run review apps for other merge requests.

See the [environments documentation](https://docs.gitlab.com/ci/environments/) for more information.

## Token Management

Read about our [IaC managed Project Access Tokens](https://gitlab.com/gitlab-org/distribution/runbooks/-/blob/main/iac-managed-project-access-tokens.md).

## OpenShift CI clusters

We manage OpenShift clusters in Google Cloud that are used for acceptance tests, including QA suite.

kubeconfig files for connecting to these clusters are stored in the 1Password cloud-native vault. Search for `ocp-ci`.

The clusters are orchestrated using the [`openshift-provisioning`](https://gitlab.com/gitlab-org/distribution/infrastructure/openshift-provisioning)
project. CI access is managed using [`kube-agents`](https://gitlab.com/gitlab-org/distribution/infrastructure/kube-agents) .

## Kubernetes CI clusters

We manage Kubernetes clusters in Google Cloud using GKE. These clusters are used to run the same acceptance tests that run on the OpenShift CI clusters.

The clusters are orchestrated using the [`infrastructure-provisioning`](https://gitlab.com/gitlab-org/distribution/infrastructure/infrastructure-provisioning)
project. CI access is managed using [`kube-agents`](https://gitlab.com/gitlab-org/distribution/infrastructure/kube-agents) .

## k3d cluster tests

The `review_k3d_*` jobs create a single-use [k3d](https://k3d.io) cluster inside the job's Docker-in-Docker environment
instead of connecting to a shared, always-on cluster. Each job deploys the operator and a GitLab custom resource, runs
the QA smoke suite against it over a [nip.io](https://nip.io) domain, and destroys the cluster when the job ends. There
is no GitLab environment or cleanup job for these tests — nothing outlives the job.

These jobs run on the privileged `e2e` runner fleet. The Kubernetes version is pinned with the `K3D_K8S_IMAGE` variable
(a [`rancher/k3s`](https://hub.docker.com/r/rancher/k3s/tags) image tag).

The shared GKE and vcluster environments are being migrated to k3d.
See [epic &98](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/98) for the migration plan.

## QA pipelines

By default, QA pipelines will include Smoke suite - a [small subset of fast end-to-end functional tests](https://docs.gitlab.com/development/testing_guide/smoke/)
to quickly ensure that basic functionality is working. If additional testing is required, it's possible to trigger manual
QA pipeline with Full suite of end-to-end tests using `qa_<cluster>_full_suite_manual_trigger` job for the specific cluster.

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
