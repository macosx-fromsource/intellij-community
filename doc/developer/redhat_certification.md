---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: RedHat Operator Bundle certification process
---

This document outlines certification process for OLM bundle submission for RedHat Marketplace. It is based on
[Red Hat Software Certification Workflow Guide](https://docs.redhat.com/en/documentation/red_hat_software_certification/2025/html/red_hat_software_certification_workflow_guide/assembly-running-the-certification-suite-locally_openshift-sw-cert-workflow-working-with-operators).

The following process is partially automated in `scripts/tools/publish.sh`.
You can use `publish.sh ${VERSION} redhat-marketplace` to run this process.
For more details see the script documentation.

Common cluster infrastructure and common GitHub account ( `gl-distribution-oc` ) are used throughout this document. If using custom cluster/GitHub account - adjust accordingly.

## Provision OpenShift cluster

### Pre-requisites

1. Personal SSH key added to `gl-distribution-oc` GitHub account (with local copy in `${HOME}/.ssh/gldoc_github`, see below)
1. Because Git Operations require separate SSH key to access `gl-distribution-oc` repositories, the sample wrapper script (`operator_certification.sh`) using key from (1) may be helpful as GitHub may reject connection if other loaded private key gets offered first:

   ```shell
   #!/bin/sh
   OC_SSH_KEYFILE=${OC_SSH_KEYFILE:-"${HOME}/.ssh/gldoc_github"}
   export GIT_SSH_COMMAND="ssh -i ${OC_SSH_KEYFILE} -o IdentitiesOnly=yes"
   exec $@
   ```

1. `olm_bundle.sh` pre-requisites:
   1. `task`
   1. `yq`
   1. `opm`

### Set up environment

Clusters are deployed using deployment pipeline. Obtain artifacts from the corresponding cluster's pipeline `deploy_cluster` job.

```shell
VERSION="0.11.0"

# CLUSTER_DIR: directory where artifacts from "deploy_cluster" job are located
CLUSTER_DIR=${HOME}/clusters/redhat-certification-ocp49

export TKN=${CLUSTER_DIR}/bin/tkn

# Use provisioned cluster's "admin"-level kubeconfig
export KUBECONFIG=${CLUSTER_DIR}/auth/kubeconfig

export GIT_USERNAME="gl-distribution-oc"

# !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
# Below email is our temporary workaround so use as-is until
#    https://gitlab.com/gitlab-org/distribution/team-tasks/-/issues/1097
#    and
#    https://gitlab.com/gitlab-org/distribution/team-tasks/-/issues/1082
# are resolved
export GIT_EMAIL="dmakovey+operator-certification@gitlab.com"
# !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

export GIT_FORK_REPO_URL="git@github.com:gl-distribution-oc/certified-operators.git"
export GIT_BRANCH="gitlab-operator-kubernetes-${VERSION}"

export OPERATOR_BUNDLE_PATH="operators/gitlab-operator-kubernetes/${VERSION}"
```

It could be convenient to save above shell code as an environment file (`my.env`, for example) and source it wherever necessary: `source /path/to/my.env`.

## Setup repository

### Fork(ed) repository

Forks for both the certified and marketplace operators have already been created:

- [certified-operators](https://github.com/gl-distribution-oc/certified-operators) or
- [redhat-marketplace-operators](https://github.com/gl-distribution-oc/redhat-marketplace-operators)

{{< alert type="note" >}}

Use of `operator_certification.sh` wrapper script below is optional.

{{< /alert >}}

1. Clone fork locally:

   ```shell
   pushd ${HOME}
   operator_certification.sh git clone git@github.com:gl-distribution-oc/certified-operators.git
   ```

1. Bring `main` branch of the fork up-to-date:

   ```shell
   git remote add upstream git@github.com:redhat-openshift-ecosystem/certified-operators.git
   git rebase -i upstream/main
   operator_certification.sh git push
   ```

1. Create new branch for the release:

   ```shell
   git checkout -b gitlab-operator-kubernetes-${VERSION}
   CATALOG_REPO_CLONE=${HOME}/certified-operators
   ```

1. Return to `gitlab-operator` local directory:

   ```shell
   popd
   ```

### Generate bundle

```shell
OSDK_BASE_DIR=".build/cert" \
    DOCKER="podman" \
    OLM_PACKAGE_VERSION=${VERSION} \
    OPERATOR_TAG=${VERSION} \
    scripts/olm_bundle.sh build_manifests generate_bundle patch_bundle
```

### Properly annotate bundle for submission

```shell
BUNDLE_DIR=.build/cert/bundle PREVIOUS_OPERATOR_VERSION="x.y.z" OLM_UPGRADE_MODE="replaces" \
    redhat/operator-certification/scripts/configure_bundle.sh adjust_annotations adjust_csv
```

### Copy & Push changes into the forked repository

At this point one must copy the bundle to its new location (retrieve the value of `CATALOG_REPO_CLONE` from [fork repository](#forked-repository)):

```shell
cp -r .build/cert/bundle ${CATALOG_REPO_CLONE}/operators/gitlab-operator-kubernetes/${VERSION}
( cd ${CATALOG_REPO_CLONE} && git add operators/gitlab-operator-kubernetes/${VERSION} \
   && git commit -am "Add gitlab-operator-${VERSION}" \
   && operator_certification.sh git push origin gitlab-operator-kubernetes-${VERSION})
```

## Run certification pipeline

GitHub Username and email must be obtained for this step and used in `GIT_USERNAME` and `GIT_EMAIL`.

Switch to appropriate project in OCP:

```shell
redhat/operator-certification/scripts/operator_certification_pipeline.sh \
  set_project
```

Create workplace template:

```shell
redhat/operator-certification/scripts/operator_certification_pipeline.sh \
  create_workspace_template
```

Then run pipeline:

```shell
redhat/operator-certification/scripts/operator_certification_pipeline.sh \
  run_certification_pipeline_automated
```

This creates an upstream PR and opens submission in the RedHat portal.
