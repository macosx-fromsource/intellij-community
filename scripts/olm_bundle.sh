#!/bin/bash

# Requirements:
#  * operator SDK
#  * docker
#  * kubectl (with built-in kustomize)
#  * podman
#  * opm
#  * yq-go : https://github.com/mikefarah/yq

# Make sure those are accurate and match your system/setup:
## Tools
set -eo pipefail

OPERATOR_SDK=${OPERATOR_SDK:-"operator-sdk"}
OPERATOR_HOME_DIR=${OPERATOR_HOME_DIR:-"."}
OPM=${OPM:-"opm"}
YQ=${YQ:-"yq"}
DOCKER=${DOCKER:-"docker"}
PODMAN=${PODMAN:-"podman"}
KIND=${KIND:-"kind"}
## Settings
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-"olm1"}
BUNDLE_REGISTRY=${BUNDLE_REGISTRY:-"registry.gitlab.com/dmakovey/gitlab-operator-bundle"}
BUNDLE_IMAGE_TAG=${BUNDLE_IMAGE_TAG:-"beta1"}
CATALOG_NAME=${CATALOG_NAME:-"gitlab-catalog"}
OLM_PACKAGE_VERSION=${OLM_PACKAGE_VERSION:-"0.2.0"}
## This will need to be set up at runtime to override specific previous version
# OLM_PACKAGE_VERSION_OLD=""
COMPILE_ONLY=${COMPILE_ONLY:-"false"}
AUTO_UPGRADE=${AUTO_UPGRADE:-"false"}

BUILD_DIR=${BUILD_DIR:-.build}
INSTALL_DIR=${INSTALL_DIR:-.install}

# Most (if not all) of the below vars can be overridden, but defaults should work
OPERATOR_IMG=${OPERATOR_IMG:-"registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator"}
OPERATOR_TAG=${OPERATOR_TAG:-"latest"}
BUNDLE_IMAGE_NAME=${BUNDLE_IMAGE_NAME:-"${BUNDLE_REGISTRY}"}
CATALOG_IMAGE_NAME=${CATALOG_IMAGE_NAME:-"${BUNDLE_REGISTRY}/gitlab-operator-catalog"}
CATALOG_IMAGE_TAG=${CATALOG_IMAGE_TAG:-"${BUNDLE_IMAGE_TAG}"}
OSDK_BASE_DIR=${OSDK_BASE_DIR:-"${BUILD_DIR}/operatorsdk"}
CATALOGSOURCE_YAML=${OSDK_BASE_DIR}/catalogsource.yaml
OPERATORGROUP_YAML=${OSDK_BASE_DIR}/operatorgroup.yaml
SUBSCRIPTION_YAML=${OSDK_BASE_DIR}/subscription.yaml
OLM_PACKAGE_NAME=${OLM_PACKAGE_NAME:-"gitlab-operator-kubernetes"}
TARGET_NAMESPACE=${TARGET_NAMESPACE:-"gitlab-system"}
OLM_NAMESPACE=${OLM_NAMESPACE:-"olm"}
OPM_DOCKER=${OPM_DOCKER:-"docker"}
KIND_CONFIG=${KIND_CONFIG:-""}
KIND_IMAGE=${KIND_IMAGE:-""}

OPERATOR_HOME_DIR=$(realpath ${OPERATOR_HOME_DIR})

build_manifests(){
  TAG=${OPERATOR_TAG} task -f build_operator_openshift
  task -f build_test_cr
  ( cd config/scorecard; kubectl kustomize . ) > ${BUILD_DIR}/scorecard.yaml
  mkdir -p ${OSDK_BASE_DIR}
  ( cd ${OSDK_BASE_DIR}; ln -sf ${OPERATOR_HOME_DIR}/config )
}

generate_bundle(){
  ${YQ} eval '.' ${BUILD_DIR}/operator-openshift.yaml ${BUILD_DIR}/test_cr.yaml ${BUILD_DIR}/scorecard.yaml \
      | (
          cd ${OSDK_BASE_DIR}
          ${OPERATOR_SDK} generate bundle -q --overwrite \
              --extra-service-accounts gitlab-manager,gitlab-nginx-ingress,gitlab-app-nonroot \
              --version ${OLM_PACKAGE_VERSION} \
              --default-channel=stable \
              --channels=stable,unstable \
              --package=${OLM_PACKAGE_NAME} \
              --use-image-digests
        )
}

operator_image() {
  local project="18899486" #canonical
  local repository="2034058" # release container repo
  local digest=$(curl --silent "https://gitlab.com/api/v4/projects/${project}/registry/repositories/${repository}/tags/${OPERATOR_TAG}" | yq -r '.digest')

  echo "${OPERATOR_IMG}@${digest}"
}

patch_bundle(){
  # Point CSV to proper image tag
  local operator_image="$(operator_image)"
  ${YQ} eval -i '(.spec.install.spec.deployments[].spec.template.spec.containers[] | select( .name=="manager").image) |= "'${operator_image}'"' \
    ${OSDK_BASE_DIR}/bundle/manifests/${OLM_PACKAGE_NAME}.clusterserviceversion.yaml
  ${YQ} eval -i '.metadata.annotations.containerImage |= "'${operator_image}'"' \
    ${OSDK_BASE_DIR}/bundle/manifests/${OLM_PACKAGE_NAME}.clusterserviceversion.yaml
  if [ -n "${OLM_PACKAGE_VERSION_OLD}" ]; then
    ${YQ} eval -i '.spec.replaces = "'${OLM_PACKAGE_NAME}.v${OLM_PACKAGE_VERSION_OLD}'"' \
      ${OSDK_BASE_DIR}/bundle/manifests/${OLM_PACKAGE_NAME}.clusterserviceversion.yaml
  fi
  ## Currently neither OLM nor OperatorSDK can't handle IngressClass presence gracefully
  ## https://github.com/operator-framework/operator-sdk/issues/5491
  # ${YQ} eval '.metadata.name="gitlab-nginx"' ${OPERATOR_HOME_DIR}/config/rbac/nginx_ingressclass.yaml > ${OSDK_BASE_DIR}/bundle/manifests/gitlab-nginx-ingressclass.yaml
  rm -rf ${OSDK_BASE_DIR}/bundle/manifests/acme.cert-manager.io*
  rm -rf ${OSDK_BASE_DIR}/bundle/manifests/cert-manager.io*
}

validate_bundle(){
  ( cd ${OSDK_BASE_DIR}; ${OPERATOR_SDK} bundle validate ./bundle )
}

compile_and_publish_bundle(){
  (
    cd ${OSDK_BASE_DIR}
    ${DOCKER} build -t ${BUNDLE_IMAGE_NAME}:${BUNDLE_IMAGE_TAG} -f bundle.Dockerfile .
    if [ ${COMPILE_ONLY} != "true" ]
    then
      ${DOCKER} push ${BUNDLE_IMAGE_NAME}:${BUNDLE_IMAGE_TAG}
    fi
  )
}

initialize_kind(){
  local kind_config_option=""
  local kind_image_option=""
  [ -n "$KIND_CONFIG" ] && kind_config_option="--config=${KIND_CONFIG}"
  [ -n "$KIND_IMAGE" ] && kind_image_option="--image=${KIND_IMAGE}"

  ${KIND} create cluster --name ${KIND_CLUSTER_NAME} ${kind_config_option} ${kind_image_option}
}

install_olm(){
  ${OPERATOR_SDK} olm install
  ${OPERATOR_SDK} olm status
}

compile_and_publish_catalog(){
  ${OPM} index add -p ${OPM_DOCKER} --bundles ${BUNDLE_IMAGE_NAME}:${BUNDLE_IMAGE_TAG} -t ${CATALOG_IMAGE_NAME}:${CATALOG_IMAGE_TAG}
  ${PODMAN} images
  if [ ${COMPILE_ONLY} != "true" ]
  then
    ${PODMAN} push ${CATALOG_IMAGE_NAME}:${CATALOG_IMAGE_TAG}
  fi
}

create_namespace(){
  kubectl get namespace ${TARGET_NAMESPACE} > /dev/null 2>&1 || kubectl create namespace ${TARGET_NAMESPACE}
}

deploy_catalogsource(){
  cat > ${CATALOGSOURCE_YAML} << EOF_CATALOGSOURCE
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${CATALOG_NAME}
  namespace: ${OLM_NAMESPACE}
spec:
  sourceType: grpc
  image: ${CATALOG_IMAGE_NAME}:${CATALOG_IMAGE_TAG}
EOF_CATALOGSOURCE

  kubectl apply -f ${CATALOGSOURCE_YAML}
  kubectl get catalogsources -n ${OLM_NAMESPACE}
  kubectl get packagemanifests | grep -F ${OLM_PACKAGE_NAME}
}

deploy_operatorgroup(){
  cat > ${OPERATORGROUP_YAML} << EOF_OPERATORGROUP
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: gitlab
  namespace: ${TARGET_NAMESPACE}
spec:
  targetNamespaces:
  - ${TARGET_NAMESPACE}
EOF_OPERATORGROUP

  kubectl apply -f ${OPERATORGROUP_YAML}
}

deploy_subscription(){
  local autoupgrade_stmt
  if [ "${AUTO_UPGRADE}" == "true" ]; then
    autoupgrade_stmt=""
  else 
    autoupgrade_stmt="installPlanApproval: Manual"
  fi
  cat > ${SUBSCRIPTION_YAML} << EOF_SUBSCRIPTION
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: gitlab-operator-subscription
  namespace: ${TARGET_NAMESPACE}
spec:
  channel: stable
  name: ${OLM_PACKAGE_NAME}
  startingCSV: ${OLM_PACKAGE_NAME}.v${OLM_PACKAGE_VERSION}
  source: ${CATALOG_NAME}
  sourceNamespace: ${OLM_NAMESPACE}
  ${autoupgrade_stmt}
EOF_SUBSCRIPTION

  kubectl apply -f ${SUBSCRIPTION_YAML}
}

all(){
  step1
  step2
  step3
}

publish(){
  build_manifests
  generate_bundle
  patch_bundle
  validate_bundle
  compile_and_publish_bundle
  compile_and_publish_catalog
}

# Convenience targets for manual testing:
step1(){
  build_manifests
  generate_bundle
  patch_bundle
}

step2(){
  validate_bundle
  compile_and_publish_bundle
  initialize_kind
  install_olm
  compile_and_publish_catalog
  create_namespace
  deploy_catalogsource
  # wait for catalogsource...
}

step3(){
  deploy_operatorgroup
  deploy_subscription
}

step2rerun(){
  validate_bundle
  compile_and_publish_bundle
  # initialize_kind
  # install_olm
  compile_and_publish_catalog
  deploy_catalogsource
}

republish(){
  validate_bundle
  compile_and_publish_bundle
  # initialize_kind
  # install_olm
  compile_and_publish_catalog
  deploy_catalogsource
  deploy_operatorgroup
  deploy_subscription
}

for cmd in "$@"
do
  $cmd
done
