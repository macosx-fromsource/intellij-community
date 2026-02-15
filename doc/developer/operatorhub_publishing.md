---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: How to publish to OperatorHub.io
---

OperatorHub.io is a home for the Kubernetes community to share Operators.

{{< alert type="note" >}}

The following process is partially automated in `scripts/tools/publish.sh`.
You can use `publish.sh ${VERSION} operatorhub` to run this process.
For more details see the script documentation.

{{< /alert >}}

To publish the GitLab Operator to OperatorHub:

1. Fork the [community-operators repository](https://github.com/k8s-operatorhub/community-operators).
1. Clone the forked community-operators repository:
   - If the fork was freshly created:

     ```shell
     git clone -o mine git@github.com:<your_github_username>/community-operators.git
     cd community-operators
     git remote add upstream https://github.com/k8s-operatorhub/community-operators.git
     ```

   - If this is a subsequent update to the already created fork:

     ```shell
     cd community-operators
     git fetch --all
     git checkout main
     git rebase -i upstream/main
     ```

1. Set up your shell:

   ```shell
   # Published Operator Image tag
   export OPERATOR_TAG="0.3.1"
   # Version that we're going to apply to OLM
   export OLM_PACKAGE_VERSION=${OPERATOR_TAG}
   export OPERATORHUB_DIR="${HOME}/work/community-operators"
   export OPERATORHUB_NAME="gitlab-operator-kubernetes"
   export OSDK_BASE_DIR=".build/operatorhub-io"
   # Optional
   # point to local instance of "yq" binary:
   export YQ="yq-go"
   ```

1. Create a new branch of `https://gitlab.com/gitlab-org/cloud-native/gitlab-operator`:

   ```shell
   cd ${OPERATORHUB_DIR}
   git checkout -B gitlab-release-${OPERATOR_TAG}
   ```

1. Edit [`config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml)
   under `metadata.annotations.alm-examples` to refer to a valid chart version
   that this version of Operator ships with.

1. Test Operator bundle in local KinD cluster:

   1. `BUNDLE_REGISTRY` has to point to a valid public registry (create your own project/registry for that purpose):

       ```shell
       export BUNDLE_REGISTRY=registry.gitlab.com/dmakovey/gitlab-operator-bundle
       ```

   1. `podman` (or `docker`) has to be logged into `BUNDLE_REGISTRY`

   Note that we're temporarily overriding previously set values to have Kind-Specific bundle etc.

   ```shell
   OSDK_BASE_DIR=".build/operatortest1" KIND_CLUSTER_NAME="optest1" BUNDLE_IMAGE_TAG="beta1" DOCKER="podman" OPERATOR_TAG=0.6.0 KIND_CONFIG="${HOME}/work/gitlab/examples/kind/kind-ssl.yaml" KIND_IMAGE="kindest/node:v1.25.9" scripts/olm_bundle.sh step1 step2
   ```

   1. Wait for the `packagemanifest` for `gitlab-operator-kubernetes` to become available (note that we skip over `Community Operators`):

      ```shell
      $ kubectl get packagemanifests | grep -F gitlab | grep -vF "Community Operators"
      gitlab-operator-kubernetes                                       48m
      ```

   1. Deploy Operator (to avoid manually approving install we'll set `AUTO_UPGRADE="true"`):

      ```shell
      OSDK_BASE_DIR=".build/operatortest1" AUTO_UPGRADE="true" scripts/olm_bundle.sh step3
      ```

   1. Create IngressClass:

      ```shell
      cat << EOF | kubectl apply -f -
      apiVersion: networking.k8s.io/v1
      kind: IngressClass
      metadata:
        # Ensure this value matches `spec.chart.values.global.ingress.class`
        # in the GitLab CR on the next step.
        name: gitlab-nginx
      spec:
        controller: k8s.io/ingress-nginx
      EOF
      ```

   1. Deploy GitLab (values need to be customized to **your** setup):

      ```shell
      KIND_CLUSTER_NAME="optest1" GITLAB_CR_DEPLOPOY_MODE="ss" LOCAL_IP=192.168.3.194 GITLAB_CHART_DIR=~/work/gitlab GITLAB_OPERATOR_DOMAIN=192.168.3.194.nip.io GITLAB_OPERATOR_DIR=. scripts/provision_and_deploy.sh  deploy_gitlab
      ```

   1. Delete KinD cluster:

      ```shell
      kind delete cluster --name=${KIND_CLUSTER_NAME}
      ```

1. Test OLM bundle upgrade

   1. Reuse variables used in testing (see above):

      ```shell
      export BUNDLE_REGISTRY=registry.gitlab.com/dmakovey/gitlab-operator-bundle
      export BUNDLE_IMAGE_TAG="beta1"
      export KIND_CLUSTER_NAME="optest1u"
      export KIND_CONFIG="${HOME}/work/gitlab/examples/kind/kind-ssl.yaml"
      export KIND_IMAGE="kindest/node:v1.25.9"
      ```

   1. Make sure to create **NEW** KinD cluster:

      ```shell
      scripts/olm_bundle.sh initialize_kind install_olm create_namespace
      ```

   1. At this point you should have bundle for the "test" version published at `${BUNDLE_REGISTRY}:${BUNDLE_IMAGE_TAG}` (if not - follow "Test Operator bundle in local KinD cluster" ) assuming previous release was `0.3.1`, we will create catalog for testing (note catalog tag `beta1u` differs from catalog tag published earlier - `beta1`):

      ```shell
      export CATALOG_IMAGE_TAG="beta1u"
      PREVIOUS_BUNDLE_VERSION="0.3.1"

      opm index add -p docker \
         --bundles registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator/bundle:${PREVIOUS_BUNDLE_VERSION},${BUNDLE_REGISTRY}:${BUNDLE_IMAGE_TAG} \
         --mode semver \
         --tag ${BUNDLE_REGISTRY}/gitlab-operator-catalog:${CATALOG_IMAGE_TAG}

      podman push ${BUNDLE_REGISTRY}/gitlab-operator-catalog:${CATALOG_IMAGE_TAG}
      ```

   1. deploy `CatalogSource` and `OperatorGroup` in preparation for operator deployment:

      ```shell

      OSDK_BASE_DIR=".build/operatortest1" scripts/olm_bundle.sh deploy_catalogsource

      OSDK_BASE_DIR=".build/operatortest1" scripts/olm_bundle.sh deploy_operatorgroup
      ```

   1. wait for the `PackageManifest`:

      ```shell
      kubectl get packagemanifests | grep -F gitlab | grep -vF "Community Operators"
      ```

   1. deploy `Subscription`:

      ```shell
      # deploy previous release
      OSDK_BASE_DIR=".build/operatortest1" OLM_PACKAGE_VERSION=${PREVIOUS_BUNDLE_VERSION} scripts/olm_bundle.sh deploy_subscription
      ```

   1. locate `InstallPlan`:

      ```shell
      $ kubectl get installplans -A
      NAMESPACE       NAME            CSV                                 APPROVAL   APPROVED
      gitlab-system   install-jfqrb   gitlab-operator-kubernetes.v0.3.1   Manual     false
      ```

   1. approve `InstallPlan`:

      ```shell
      kubectl -n gitlab-system patch installplan install-jfqrb -p '{"spec":{"approved":true}}' --type merge
      ```

      this should trigger automatical creation of a new install plan for the current version ( `0.6.1` ):

      ```shell
      $ kubectl get installplans -A
      NAMESPACE       NAME            CSV                                 APPROVAL   APPROVED
      gitlab-system   install-4dvgh   gitlab-operator-kubernetes.v0.6.1   Manual     false
      gitlab-system   install-jfqrb   gitlab-operator-kubernetes.v0.3.1   Manual     true

      ```

   1. approve upgrade:

      ```shell
      kubectl -n gitlab-system patch installplan install-4dvgh -p '{"spec":{"approved":true}}' --type merge
      ```

   1. Delete KinD cluster:

      ```shell
      kind delete cluster --name=${KIND_CLUSTER_NAME}
      ```

1. Create the Operator bundle

   ```shell
   # assemble bundle
   scripts/olm_bundle.sh build_manifests generate_bundle patch_bundle
   # validate bundle
   scripts/olm_bundle.sh validate_bundle
   ```

1. Copy bundle files to the right location:

   ```shell
   mkdir -p ${OPERATORHUB_DIR}/operators/${OPERATORHUB_NAME}/${OLM_PACKAGE_VERSION}
   cp -r ${OSDK_BASE_DIR}/bundle/* ${OPERATORHUB_DIR}/operators/${OPERATORHUB_NAME}/${OLM_PACKAGE_VERSION}
   ```

1. Add and commit (with sign) your changes:

   ```shell
   cd ${OPERATORHUB_DIR}
   git add operators/${OPERATOR_HUB_NAME}/${OLM_PACKAGE_VERSION}
   git commit -s
   ```

1. Push your branch to your fork and create a pull request upstream.
   Wait for approval from GitLab team members and/or OperatorHub reviewers
   before the merge is completed.
