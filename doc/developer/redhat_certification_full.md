---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: RedHat Operator Bundle certification process
---

This document outlines certification process for OLM bundle submission for RedHat Marketplace. It is based on
[Red Hat Software Certification Workflow Guide](https://docs.redhat.com/en/documentation/red_hat_software_certification/2025/html/red_hat_software_certification_workflow_guide/assembly-running-the-certification-suite-locally_openshift-sw-cert-workflow-working-with-operators).

Below process outlines **full** setup for certification.

## Pre-requisites

1. GitHub account is required.
   1. personal account could be used
   1. service account (like `gl-distribution-oc`) can be used
      1. SSH key pair is necessary for proper operation
      1. for convenience `operator_certification.sh` can be used (replace `gldoc_github` with our key name):

         ```shell
          #!/bin/sh
          OC_SSH_KEYFILE=${OC_SSH_KEYFILE:-"${HOME}/.ssh/gldoc_github"}
          export GIT_SSH_COMMAND="ssh -i ${OC_SSH_KEYFILE} -o IdentitiesOnly=yes"
          exec $@
          ```

1. `olm-bundle.sh` pre-requisites:
   1. `task`
   1. `operator-sdk`
   1. `yq`
   1. `opm`

## Provision OpenShift cluster

Please check existing clusters - perhaps one have already been set up.

Existing OpenShift cluster is required. If using already provisioned OpenShift cluster adjust steps accordingly. Required components to be installed in cluster:

- `cert-manager` (Operator Requirement)
- OpenShift Pipelines (Certification pipeline requirement)
- (optional) `external-dns` (It is **required** in CI)

### OpenShift-provisioning pipeline

One way to provision a new OpenShift cluster is using the OpenShift-provisioning pipeline (`https://gitlab.com/gitlab-org/distribution/infrastructure/openshift-provisioning`).

Pipeline creates convenient artifact that includes auth information for cluster as well as all necessary binaries. Download Zip file for the artifact produced by `deploy_cluster` job and extract it into some convenient location (`${HOME}/mycluster`)

```shell
BASEDIR=${HOME}/mycluster
OC=${BASEDIR}/bin/oc-x.y.z
TKN=${BASEDIR}/bin/tkn
KUBECONFIG=${BASEDIR}/x.y.z/auth/kubeconfig
export OC TKN KUBECONFIG
```

### Manual provisioning

Follow OpenShift installation documentation to provision new cluster. Make sure to extract and preserve:

- `oc` binary corresponding to OCP release set up
  - See [RedHat files mirror](https://mirror.openshift.com/pub/openshift-v4/clients/ocp/)
- download `tkn` binary compatible with the release
  - Documentation for your release, for example [release notes for 4.9](https://docs.openshift.com/container-platform/4.9/cicd/pipelines/op-release-notes.html)
  - Pre-requisites section from [Chapter 21. Running the certification test suite locally](https://docs.redhat.com/en/documentation/red_hat_software_certification/2024/html/red_hat_software_certification_workflow_guide/assembly-running-the-certification-suite-locally_openshift-sw-cert-workflow-working-with-operators)
  - [Teknton CD requirements](https://github.com/tektoncd/pipeline#required-kubernetes-version)
- contents of the `auth` directory under the installation root directory

```shell
OC=/path/to/oc
TKN=/path/to/tkn
KUBECONFIG=/path/to/kubeconfig
export OC TKN KUBECONFIG
```

## Fork repo

You'll need to fork one of:

- [certified-operators](https://github.com/redhat-openshift-ecosystem/certified-operators) or
- [redhat-marketplace-operators](https://github.com/redhat-openshift-ecosystem/redhat-marketplace-operators)

for the current GitHub user.

Use GitHub UI to create a fork

### Create Deploy Keys

Create SSH key pair with empty passphrase:

```shell
ssh-keygen -f certified-operators-key
```

for forked repo create `Deploy Keys` (`Settings`/`Deploy keys`/`Add new` for the repo)

and add `certified-operators-key.pub`

## Setup environment

Using some of the settings above and settings we'll need later,
it may be useful to create a `my.env` file for shell setup:

```shell
VERSION="0.11.0"
# Pipeline
export KUBECONFIG=/path/to/secure/location/kubeconfig

CLUSTER_DIR=/path/to/clusters/clusterX
export OC=${CLUSTER_DIR}/bin/oc-x.y.z
export TKN=${CLUSTER_DIR}/bin/tkn

# Generate Pyxis API key in RH Connect portal, or locate one in
# 1Password (operator-secrets.yaml)
export PYXIS_API_KEY_FILE=pyxis-key-operator-bundle.txt

# Deployment key for the forked repo (see "Fork repo" section)
# alternatively, is using 'gl-distribution-oc' - obtain from
# 1Password (GitHub Operator Certification deploy key)
export SSH_KEY_FILE=gl-distribution-oc/gl-distribution-oc-deploy

# GitHub API key for the forked repo (see "Fork repo" section)
# alternatively, is using 'gl-distribution-oc' - obtain from
# 1Pasword (GitHub Operator Certification API token)
export GITHUB_TOKEN_FILE=gl-distribution-oc/gh-token.txt

# Below section is written assuming 'gl-distribution-oc' GitHub user
# if using different GitHub account - adjust accordingly
export GIT_USERNAME="gl-distribution-oc"
export GIT_EMAIL="dmakovey+operator-certification@gitlab.com"
export GIT_FORK_REPO_URL="git@github.com:gl-distribution-oc/certified-operators.git"
export GIT_BRANCH="gitlab-operator-kubernetes-${VERSION}"

# Path within upstream repo to operator bundle
export OPERATOR_BUNDLE_PATH="operators/gitlab-operator-kubernetes/${VERSION}"
```

**NOTE** we're using `gl-distribution-oc` user here by default, if you're attempting to set up with different user - please adjust above settings accordingly

before executing any of the following commands, make sure to source `my.env` file:

```shell
source my.env
```

## Setup OpenShift cluster

```shell
redhat/operator-certification/scripts/operator_certification_pipeline.sh \
  create_cluster_infra

redhat/operator-certification/scripts/install_oco.sh create_manifest \
  apply_manifest
```

## Setup repo

### Create API token (PAT)

In GitHub navigate to profile settings (`https://github.com/settings/profile`), `Developer settings`/`Personal access tokens`, and generate a new ("classic") one with scope `repo`.
Save it to a local file in a secure location (`${HOME}/secure/github_api_token.txt`).

**NOTE** this token as access to **all** of GitHub user's repos at this point.

### Clone repo

```shell
REPO_HOME=${HOME}
pushd ${REPO_HOME}
git clone git@github.com:<YOUR-GITHUB-ACCOUNT>/certified-operators.git
git checkout -b gitlab-operator-kubernetes-${VERSION}
popd
```

add to your environment `CATALOG_REPO_CLONE` for future references:

```shell
export CATALOG_REPO_CLONE=${REPO_HOME}/certified-operators
```

## Setup Project in RedHat Connect portal

Navigate to `Gitlab Operator Bundle`, Open `Settings` tab.

Add GitHub user to `Authorized GitHub user accounts`

## Setup pipeline

### Pre-requisites

- [Provisioned OpenShift cluster](#provision-openshift-cluster) you should have:
  - associated `kubeconfig` file (`$KUBECONFIG`)
  - associated `tkn` binary (`$TKN`)
  - associated `oc` binary (`$OC`)
- GitHub PAT (`$GITHUB_TOKEN_FILE`)
  - 1Password: look for GitHub API token
- Pyxis API Token (`$PYXIS_API_KEY_FILE`) obtained from RedHat
  - 1Password: has to be extracted from `operator-secrets.yaml`
- SSH Key pair (`$SSH_KEY_FILE` - private key file)
  - Added as "deployment key" to forked project
  - 1Password: look for GitHub Deploy key

**NOTE** make sure files `$GITHUB_TOKEN_FILE` and `$PYXIS_API_KEY_FILE` do not contain newline character (`0x0a` at the end of the file):

```shell
hexdump -C $GITHUB_TOKEN_FILE
hexdump -C $PYXIS_API_KEY_FILE
```

then execute create secrets and install pipeline:

```shell
GITHUB_TOKEN_FILE=/path/to/github_token.txt \
  PYXIS_API_KEY_FILE=/path/to/pyxis_api_key.txt \
  SSH_KEY_FILE=/path/to/certified-operators-key \
  KUBECONFIG="${BASEDIR}/auth/kubeconfig" \
  redhat/operator-certification/scripts/operator_certification_pipeline.sh create_secrets install_pipeline_automated create_workspace_template
```

## Generate bundle

```shell
OSDK_BASE_DIR=".build/cert" \
    DOCKER="podman" \
    OLM_PACKAGE_VERSION=${VERSION} \
    OPERATOR_TAG=${VERSION} \
    scripts/olm_bundle.sh build_manifests generate_bundle patch_bundle
```

## Properly annotate bundle for submission

```shell
BUNDLE_DIR=.build/cert/bundle PREVIOUS_OPERATOR_VERSION="x.y.z" \
    redhat/operator-certification/scripts/configure_bundle.sh adjust_annotations adjust_csv
```

## Copy & Push changes into the forked repo

At this point one needs to copy bundle to it's new location (you'll need value of `CATALOG_REPO_CLONE` from [fork repo](#fork-repo)):

```shell
cp -r .build/cert/bundle ${CATALOG_REPO_CLONE}/operators/gitlab-operator-kubernetes/${VERSION}
( cd ${CATALOG_REPO_CLONE} && git add operators/gitlab-operator-kubernetes/${VERSION} \
   && git commit -am "Add gitlab-operator-${VERSION}" \
   && git push origin gitlab-operator-kubernetes-${VERSION})
```

## Run certification pipeline

GitHub Username and email will need to be obtained for this step and used respectively in `GIT_USERNAME` and `GIT_EMAIL`

```shell
redhat/operator-certification/scripts/operator_certification_pipeline.sh \
  run_certification_pipeline_automated
```

this will create upstream PR and open submission in RH portal

**NOTE** if you are getting

```plaintext
ValueError: Invalid header value b'Bearer XXXXXXXXXXXXXXXXXXXXXXX\n'
```

in the output - likely one of the Secrets contains unwanted newline: `pyxis-api-secret` or `github-api-token` context of the error should help determine which one.

## Cleanup

it is a good idea to delete person-centric secrets after pipeline has been completed:

```shell
redhat/operator-certification/scripts/operator_certification_pipeline.sh cleanup_secrets
```
