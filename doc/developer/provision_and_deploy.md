---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab autodeployment for testing
---

## Requirements

- `openssl` utility
- `kubectl`
- `task`
- cluster interaction tool (one of):
  - `gcloud`
  - `kind`

## Parameters

Parameters are passed via environment variables:

| Variable name              | Required      | Default                                                               | Description |
|----------------------------|---------------|-----------------------------------------------------------------------|-------------|
| `GITLAB_CHART_VERSION`     | no            | latest available                                                      | Chart version to use. Must align with the charts provided within `TAG` of the operator |
| `GITLAB_CHART_REPO`        | no            | `https://gitlab.com/gitlab-org/charts/gitlab`                         | GitLab Helm Chart repository HTTP URI. Mainly used to fetch default KinD configs. |
| `IMG`                      | no            | `registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator`         | Operator Container Image Name |
| `TAG`                      | no            | `master`                                                              | Operator Container Image Tag. Needs an override in most cases |
| `GITLAB_CHART_DIR`         | yes           |                                                                       | path to a clone of GitLab Chart repo |
| `GITLAB_OPERATOR_DIR`      | no            | `.`                                                                   | path to a clone of GitLab Operator repo |
| `GITLAB_OPERATOR_MANIFEST` | no            |                                                                       | Optional reference to manifest for Operator deployment, if empty - auto-generated from `${GITLAB_OPERATOR_DIR}`. To note: to reference proper image and tag set up `IMG` and `TAG` environment variables |
| `GITLAB_CR_DEPLOY_MODE`    | no            | `selfsigned`                                                          | Select mode of deployment: `selfsigned` or `certmanager` |
| `GITLAB_OPERATOR_DOMAIN`   | no            | `${LOCAL_IP}.nip.io` for KinD, `cloud-native.win` for other platforms | Domain for GitLab (operator) deployment |
| `GITLAB_HOST`              | no            | `*.${GITLAB_OPERATOR_DOMAIN}`                                         | Common name to use for GitLab endpoint self-signed cert |
| `GITLAB_KEY_FILE`          | no            | `gitlab.key`                                                          | Self-signed cert key file |
| `GITLAB_CERT_FILE`         | no            | `gitlab.crt`                                                          | Self-signed cert file |
| `GITLAB_PAGES_HOST`        | no            | `*.pages.${GITLAB_OPERATOR_DOMAIN}`                                   | Common name to use for GitLab Pages endpoint self-signed cert |
| `GITLAB_PAGES_KEY_FILE`    | no            | `pages.key`                                                           | Self-signed cert key file |
| `GITLAB_PAGES_CERT_FILE`   | no            | `pages.crt`                                                           | Self-signed cert file |
| `GITLAB_ACME_EMAIL`        | no            | output of `git config user.email`                                     | Email used for cert-manager. Not necessary in KinD deployments |
| `GITLAB_RUNNER_TOKEN`      | no            |                                                                       | Runner Token, if empty it's auto-retrieved from running GitLab Instance |
| `KIND`                     | no            | `kind`                                                                | command line executable name for KinD |
| `KIND_CLUSTER_NAME`        | no            | `gitlab`                                                              | KinD cluster name |
| `KIND_IMAGE`               | no            | `kindest/node:v1.18.19`                                               | value of `--image` argument for KinD |
| `KIND_LOCAL_IP`            | yes, for KinD |                                                                       | Local IP required to provision Certs etc for the domain `${LOCAL_IP}.nip.io` |
| `KUBECTL`                  | no            | `kubectl`                                                             | path to `kubectl` command |
| `HELM`                     | no            | `helm`                                                                | path to `helm` command |
| `TASK`                     | no            | `task`                                                                | path to `task` command |

### Tool pointer variables (`$KIND`, `$KUBECTL`, `$HELM`, etc.)

Main use of tool pointer variables is to point to particular tool path or path to a wrapper script (like `k` for `kubectl`, for example).

One of the alternative uses for any one of those variables is to get some debugging info:

```shell
KUBECTL="echo kubectl" provision_and_deploy.sh deploy_operator
```

However, this can also be achieved by using `bash -x provision_and_deploy.sh`.

## GCP

Example with Cert-Manager (ran from the root of `gitlab-operator` repo):

```shell
export GITLAB_CHART_DIR=~/work/gitlab \
       GITLAB_OPERATOR_DOMAIN="mydomain.k8s-ft.win" \
       GITLAB_ACME_EMAIL="somebody@gitlab.com" \
       GITLAB_CR_DEPLOY_MODE="certmanager"

# https://docs.gitlab.com/charts/installation/cloud/gke/
PROJECT="gcp-project-123" CLUSTER_NAME="mydomain" \
    bash ${GITLAB_CHART_DIR}/scripts/gke_bootstrap_script.sh up

# ...wait for provisioning to complete
scripts/provision_and_deploy.sh generic_deploy
```

Alternatively, use a CR generated by a pipeline (downloaded into `./123-my-branch.yaml`, for example):

```shell
cd scripts
export GITLAB_CHART_DIR=~/work/gitlab \
       GITLAB_OPERATOR_DIR=~/work/gitlab-operator \
       GITLAB_OPERATOR_MANIFEST=./123-my-branch.yaml \
       GITLAB_OPERATOR_DOMAIN="mydomain.k8s-ft.win" \
       GITLAB_ACME_EMAIL="somebody@gitlab.com" \
       GITLAB_CR_DEPLOY_MODE="certmanager"

PROJECT="gcp-project-123" CLUSTER_NAME="mydomain" \
    bash ${GITLAB_CHART_DIR}/scripts/gke_bootstrap_script.sh up

# ...wait for provisioning to complete
./provision_and_deploy.sh generic_deploy
```

The command above has been run from the within `scripts/` directory (note the use of `GITLAB_OPERATOR_DIR`).

One can deploy with self-signed certs, in which case `KIND_LOCAL_IP` should be provided (use "cluster IP") and not `GITLAB_OPERATOR_DOMAIN`.

## KinD

By default deployment is done with Self-Signed cert:

```shell
export KIND_CLUSTER_NAME=gitlab \
       KIND_LOCAL_IP=192.168.3.194 \
       GITLAB_CHART_DIR=~/work/gitlab

scripts/provision_and_deploy.sh kind_deploy
```

Alternatively, use a CR generated by a pipeline in `build manifest` job (downloaded into `./123-my-branch.yaml`, for example):

```shell
export KIND_CLUSTER_NAME=gitlab \
       KIND_LOCAL_IP=192.168.3.194 \
       GITLAB_CHART_DIR=~/work/gitlab \
       GITLAB_OPERATOR_MANIFEST=./123-my-branch.yaml \
       GITLAB_OPERATOR_DIR=~/work/gitlab-operator

scripts/provision_and_deploy.sh kind_deploy
```

That's it! You should now be able to navigate to `https://gitlab.(your IP).nip.io` and log in with the root password.

**NOTE**: Use of `cert-manager` for generating certificates for Ingresses in KinD is not possible unless your KinD instance is publicly accessible.

## Runner deployment

Once base deployment has been performed do the runner deployment (**retaining same exported variables**):

```shell
scripts/provision_and_deploy.sh runner_deploy
```

Alternatively, do everything in one go (we'll use `kind` deploy for example):

```shell
scripts/provision_and_deploy.sh kind_deploy runner_deploy
```
