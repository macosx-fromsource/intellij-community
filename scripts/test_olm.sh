#!/bin/bash

set -ueo pipefail

cleanup_files(){
  if [ "${_cleanup_kind_config}" == "true" ]; then
    rm "${KIND_CONFIG}"
  fi
}

get_chart_version(){
    local operator_version=$1
    curl "https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/raw/${operator_version}/CHART_VERSIONS" \
        | head -n 1
}

trap cleanup_files EXIT

# shellcheck source=/dev/null
source "${OLM_TESTING_ENVIRONMENT:-./test_olm.env}"

OPERATOR_VERSION=${OPERATOR_VERSION:-""}
PREVIOUS_OPERATOR_VERSION=${PREVIOUS_OPERATOR_VERSION:-""}
BUNDLE_VERSION=${BUNDLE_VERSION:-${OPERATOR_VERSION}}

OPERATOR_TAG=${OPERATOR_TAG:-${OPERATOR_VERSION}}
export OPERATOR_TAG

DO_NOT_PUBLISH=${DO_NOT_PUBLISH:-""}

OLM_BUNDLE_SH="scripts/olm_bundle.sh"
PROVISION_AND_DEPLOY_SH="scripts/provision_and_deploy.sh"

DEFAULT_NAMESPACE="gitlab-system"

OPM=${OPM:-"opm"}
YQ=${YQ:-"yq"}

# export OPERATORHUB_NAME="gitlab-operator-kubernetes"
export OSDK_BASE_DIR=".build/operatorhub-io"
export OLM_PACKAGE_VERSION=${OPERATOR_TAG}

KUBERNETES_TIMEOUT=${KUBERNETES_TIMEOUT:-"120s"}
KUBECTL="kubectl"

# Normally this needs no override
GITLAB_OPERATOR_OLM_REGISTRY=${GITLAB_OPERATOR_OLM_REGISTRY:-"registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator/bundle"}

FLAT_VERSION=$(echo "${OPERATOR_TAG}" | tr -d '.')
# Really assumes that bundle is tagged the same as operator
PREVIOUS_BUNDLE_VERSION=${PREVIOUS_BUNDLE_VERSION:-${PREVIOUS_OPERATOR_VERSION}}

# PREVIOUS_CHART_VERSION=${PREVIOUS_CHART_VERSION:-get_chart_version ${PREVIOUS_BUNDLE_VERSION}}
if [[ -z "${PREVIOUS_CHART_VERSION+}" ]]; then
    PREVIOUS_CHART_VERSION=$(get_chart_version "${PREVIOUS_BUNDLE_VERSION}")
fi

GITLAB_CHART_REPO=${GITLAB_CHART_REPO:-"https://gitlab.com/gitlab-org/charts/gitlab"}

CATALOG_TAG_SUFFIX=""

# GitLab deployment vars
export LOCAL_IP=${LOCAL_IP:-"127.0.0.1"}
# export GITLAB_CHART_DIR=${GITLAB_CHART_DIR:-"${HOME}/work/gitlab"}
export GITLAB_OPERATOR_DOMAIN=${GITLAB_OPERATOR_DOMAIN:-"${LOCAL_IP}.nip.io"}
export GITLAB_OPERATOR_DIR=${GITLAB_OPERATOR_DIR:-"."}
GITLAB_CHART_VERSION=${GITLAB_CHART_VERSION:-$(cd "${GITLAB_OPERATOR_DIR}"; git show "${OPERATOR_VERSION}":CHART_VERSIONS | head -n 1)}
export GITLAB_CHART_VERSION

K8S_VERSION=${K8S_VERSION:-"1.30.8"}
_cleanup_kind_config="false"
# export KIND_CONFIG="${GITLAB_CHART_DIR}/examples/kind/kind-ssl.yaml"
if [ -z "${KIND_CONFIG+}" ]; then
    KIND_CONFIG=$(mktemp -p . kind-ssl.XXXXX)
    export KIND_CONFIG
    _cleanup_kind_config="true"
    curl -o "${KIND_CONFIG}" "${GITLAB_CHART_REPO}/-/raw/master/examples/kind/kind-ssl.yaml"
fi
export KIND_IMAGE=${KIND_IMAGE:-"kindest/node:v${K8S_VERSION}"}

# Operator-SDK internally uses podman, so for better interoperability
# we'll be using podman across the board
export DOCKER="podman"

# export OSDK_BASE_DIR=".build/operatortest1" \
export \
    KIND_CLUSTER_NAME="optest${FLAT_VERSION}u" \
    BUNDLE_IMAGE_TAG="test-${OPERATOR_TAG}"

# upgrade-specific
export CATALOG_IMAGE_TAG="upgrade-${OPERATOR_TAG}-${PREVIOUS_BUNDLE_VERSION}${CATALOG_TAG_SUFFIX}"

setup_kind_cluster(){
    ${OLM_BUNDLE_SH} initialize_kind install_olm create_namespace
}


publish_bundle_and_catalog(){
    # In reality we're publishing both: bundle and catalog here
    if [ "${DO_NOT_PUBLISH}" == "yes" ]; then
        echo "Will not publish bundle and catalog. Will use already published instead."
    else
        ${OLM_BUNDLE_SH} publish
    fi
}

publish_upgrade_catalog(){
    local _current_registry
    if [ "${DO_NOT_PUBLISH}" == "yes" ]; then
        _current_registry=${GITLAB_OPERATOR_OLM_REGISTRY}
        _current_tag=${BUNDLE_VERSION}
    else
        _current_registry=${BUNDLE_REGISTRY}
        _current_tag=${BUNDLE_IMAGE_TAG}
    fi
    ${OPM} index add -p docker \
        --bundles "${GITLAB_OPERATOR_OLM_REGISTRY}:${PREVIOUS_BUNDLE_VERSION},${_current_registry}:${_current_tag}" \
        --mode semver \
        --tag "${BUNDLE_REGISTRY}/gitlab-operator-catalog:${CATALOG_IMAGE_TAG}"

    # ${OPM} uses podman with no visible way to alter that:
    podman push "${BUNDLE_REGISTRY}/gitlab-operator-catalog:${CATALOG_IMAGE_TAG}"
}

deploy_catalogsource_and_operatorgroup(){
    mkdir -p ${OSDK_BASE_DIR}
    ${OLM_BUNDLE_SH} deploy_catalogsource
    ${OLM_BUNDLE_SH} deploy_operatorgroup
}

check_packagemanifest(){
    ${KUBECTL} get packagemanifests | grep -F gitlab | grep -vF "Community Operators" 
}

deploy_subscription(){
    local olm_version=${1:-"${OLM_PACKAGE_VERSION}"}
    OLM_PACKAGE_VERSION="${olm_version}" ${OLM_BUNDLE_SH} deploy_subscription
}

deploy_gitlab(){
    ${PROVISION_AND_DEPLOY_SH} deploy_gitlab
}

deploy_gitlab_test(){
    #XXX HOSTSUFFIX=
    CHART_VERSION=${GITLAB_CHART_VERSION} \
        DOMAIN=${GITLAB_OPERATOR_DOMAIN} \
        TLSCERTNAME="custom-gitlab-tls" \
        ${TEST_SH} build_gitlab_custom_resource
    # produced:
    #  * $BUILD_DIR/test_cr.yaml
    #  * ${BUILD_DIR}/gitlab-${HOSTSUFFIX}.${DOMAIN}.yaml
    CHART_VERSION=${GITLAB_CHART_VERSION} \
        DOMAIN=${GITLAB_OPERATOR_DOMAIN} \
        TLSCERTNAME="custom-gitlab-tls" \
        ${TEST_SH} install_gitlab_custom_resource

}

deploy_ingressclass(){
    cat << EOF | ${KUBECTL} apply -f -
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: gitlab-nginx
spec:
  controller: k8s.io/ingress-nginx
EOF
}

upgrade_chart(){
    local gitlab_object="gitlab"
    "${KUBECTL}" patch gitlab -n "${TARGET_NAMESPACE:-${DEFAULT_NAMESPACE}}" ${gitlab_object} --type=merge -p '{"spec": { "chart": { "version": "'"${GITLAB_CHART_VERSION}"'" }}}'
}

wait_for_packagemanifests(){
    local wait_loop="until $KUBECTL get -A packagemanifests | grep -vF \"Community Operators\" | grep gitlab ; do echo 'waiting'; sleep 5; done"
    # make sure it's been created
    timeout "${KUBERNETES_TIMEOUT}" bash -c "${wait_loop}"
}

wait_for_operator(){
    ${PROVISION_AND_DEPLOY_SH} wait_for_operator
}

wait_for_gitlab(){
    local timeout=${KUBERNETES_TIMEOUT}
    export KUBERNETES_TIMEOUT="420s"
    ${PROVISION_AND_DEPLOY_SH} wait_for_gitlab
    export KUBERNETES_TIMEOUT=${timeout}
}

get_latest_installplan(){
    local _install_plan_id="null"
    while [ "$_install_plan_id" == "null" ]
    do
        _install_plan_id=$(${KUBECTL} get -n "${TARGET_NAMESPACE:-${DEFAULT_NAMESPACE}}" subscription gitlab-operator-subscription -o json | jq -r '.status.installPlanRef.name')
    done
    INSTALL_PLAN_ID=${_install_plan_id}
    echo "${INSTALL_PLAN_ID}"
}

approve_installplan(){
    "${KUBECTL}" -n "${TARGET_NAMESPACE:-${DEFAULT_NAMESPACE}}" patch installplan "${INSTALL_PLAN_ID}" -p '{"spec":{"approved":true}}' --type merge
}

check_gitlab(){
    local status
    status=$(kubectl get gitlab/gitlab -n "${TARGET_NAMESPACE:-gitlab-system}" -ojson | jq -r '.status.phase')
    echo "Status: ${status}"
    [ "${status}" == "Running" ]
}

check_gitlab2(){
    local status
    status=$(curl -ks "https://gitlab.${GITLAB_OPERATOR_DOMAIN}/-/readiness" | jq -r '.status')
    echo "Status: ${status}"
    echo -n "Ready: "
    ${KUBECTL} get deployment -A -lrelease=gitlab -o yaml | \
        ${YQ} eval -P '{ "items": (.items | length), "ready": ([ .items[] | select(.status.readyReplicas == .status.replicas) ] | length) } | .items==.ready' -
}

upgrade_test_step1(){
    # setup_kind_cluster
    deploy_ingressclass
    echo "Publish bundle and catalog"
    publish_bundle_and_catalog
    publish_upgrade_catalog
    echo "Deploy CalalogSource and OperatorGroup"
    deploy_catalogsource_and_operatorgroup
    ## Why is it dropping off right here???

    echo "Checking PackageManifests"
    check_packagemanifest || true # we just check here
    wait_for_packagemanifests
    check_packagemanifest

    echo "Deploying subscription"
    local _auto_upgrade=${AUTO_UPGRADE+""}
    export AUTO_UPGRADE="false"
    deploy_subscription "${PREVIOUS_BUNDLE_VERSION}"
    export AUTO_UPGRADE=${_auto_upgrade}

    echo "Approving InstallPlan for Operator..."
    get_latest_installplan
    approve_installplan
    wait_for_operator
}

upgrade_test_step2(){
    ## after installplan was approved
    ## We need to deploy older chart here
    GITLAB_CHART_VERSION=${PREVIOUS_CHART_VERSION} deploy_gitlab
    wait_for_gitlab
    echo -n "GitLab status is: "
    check_gitlab
}

upgrade_test_step3(){
    get_latest_installplan
    approve_installplan
}

upgrade_test_step4(){
    ## after deployment confirmed to be functional
    ## after upgrade installplan was approved
    upgrade_chart
}

test(){
    ##XXX Broken ???
    # setup_cluster
    export CATALOG_IMAGE_TAG="${OPERATOR_TAG}${CATALOG_TAG_SUFFIX}"
    deploy_ingressclass
    publish_bundle_and_catalog
    deploy_catalogsource_and_operatorgroup

    check_packagemanifest
    wait_for_packagemanifests
    check_packagemanifest

    AUTO_UPGRADE="true"
    deploy_subscription
    wait_for_operator

    deploy_gitlab
    wait_for_gitlab
}

for cmd in "$@"
do
    $cmd
done
