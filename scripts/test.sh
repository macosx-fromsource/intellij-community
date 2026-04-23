#!/bin/bash -e

# Functional test that verifies the GitLab operator and CR install without error

TESTS_NAMESPACE="${TESTS_NAMESPACE:-gitlab-system}"
CLEANUP="${CLEANUP:-yes}"
HOSTSUFFIX="${HOSTSUFFIX:-${TESTS_NAMESPACE}}"
DOMAIN="${DOMAIN:-example.com}"
DEBUG_CLEANUP="${DEBUG_CLEANUP:-off}"

REGISTRY_AUTH_SECRET_NS=${REGISTRY_AUTH_SECRET_NS:-""}
REGISTRY_AUTH_SECRET=${REGISTRY_AUTH_SECRET:-""}

BASE_DIR=${BASE_DIR:-$(pwd)}
export INSTALL_DIR=$(realpath ${INSTALL_DIR:-"${BASE_DIR}/.install"})
export BUILD_DIR=$(realpath ${BUILD_DIR:-"${BASE_DIR}/.build"})

# When defined - skip cleanup at the end of script run
NO_TRAP=${NO_TRAP:-""}

# Command for `yq`, expected to be https://github.com/mikefarah/yq
YQ=${YQ:-"yq"}

export IMG TAG GITLAB_CHART_VERSION NAMESPACE=${TESTS_NAMESPACE}

# Trim name override to leave room for prefixes/suffixes
NAME_OVERRIDE="g${TESTS_NAMESPACE:0:27}"
# Trim any hyphens in the suffix
NAME_OVERRIDE="${NAME_OVERRIDE%-}"
export NAME_OVERRIDE

# GitLab chart repository for sourcing CI scripts
CHART_REPO_URL="${CHART_REPO_URL:-https://gitlab.com/gitlab-org/charts/gitlab}"
CHART_CI_LIB_REF="${CHART_CI_LIB_REF:-master}"

# PostgreSQL image tag for CloudNativePG (overridable via CI variable)
CNPG_POSTGRESQL_TAG="${CNPG_POSTGRESQL_TAG:-17}"
export CNPG_POSTGRESQL_TAG

finish() {
  local exitcode=$?

  task restore_kustomize_files

  if [ $exitcode -ne 0 ]; then
    echo "!!!ERROR!!!"
    echo "deployment/${NAME_OVERRIDE}-controller-manager logs"
    kubectl -n "$TESTS_NAMESPACE" logs "deployment/${NAME_OVERRIDE}-controller-manager" -c manager || true
  fi

  if [ "$CLEANUP" = "yes" ]; then
    cleanup
  else
    echo 'Skipping cleanup'
  fi
}
[ -z "${NO_TRAP}" ] && trap finish EXIT

main() {
  [ "$CLEANUP" = "only" ] && { cleanup; exit 0; }

  echo 'Starting test'
  create_namespace
  prepare_build_directories

  install_gitlab_operator
  verify_operator_is_running
  copy_certificate

  provision_external_services

  build_gitlab_custom_resource
  install_gitlab_custom_resource
  verify_gitlab_is_running
}

_repurpose_cr(){
  # Strip all the metadata k8s adds to resource
  # upon creation and make resource more "generic"
  ${YQ} eval "del(.metadata.namespace,.metadata.creationTimestamp,.metadata.resourceVersion,.metadata.selfLink,.metadata.uid,.metadata.managedFields)" $@
}

create_namespace() {
  kubectl get namespace ${TESTS_NAMESPACE} > /dev/null 2>&1 || kubectl create namespace ${TESTS_NAMESPACE}
  if [ -n "${REGISTRY_AUTH_SECRET}" ] && [ -n "${REGISTRY_AUTH_SECRET_NS}" ]
  then
    kubectl get secret ${REGISTRY_AUTH_SECRET} --namespace=${REGISTRY_AUTH_SECRET_NS} -o yaml \
      | _repurpose_cr - \
      | sed -e "s/namespace: ${REGISTRY_AUTH_SECRET_NS}/namespace: ${TESTS_NAMESPACE}/" \
      | kubectl apply --namespace=${TESTS_NAMESPACE} -f -
  fi
}

prepare_build_directories() {
  mkdir -p ${INSTALL_DIR}
  mkdir -p ${BUILD_DIR}
}

# setup_chart_ci_scripts downloads and sources the CI library scripts from the
# GitLab chart repository.
# The scripts are sources to avoid drifts in setup between GitLab chart and
# Operator.
setup_chart_ci_scripts() {
  [ -n "${_CHART_CI_SCRIPTS_LOADED:-}" ] && return 0

  echo "Downloading chart CI lib scripts from ${CHART_REPO_URL} @ ${CHART_CI_LIB_REF}"
  local scripts_dir="${BUILD_DIR}/chart-ci-lib"
  local base_url="${CHART_REPO_URL}/-/raw/${CHART_CI_LIB_REF}/scripts/ci/lib"

  mkdir -p "${scripts_dir}"
  for script in helpers.sh cloudnativepg.sh valkey.sh garage.sh; do
    curl -fsSL "${base_url}/${script}" -o "${scripts_dir}/${script}"
  done

  source "${scripts_dir}/helpers.sh"
  source "${scripts_dir}/cloudnativepg.sh"
  source "${scripts_dir}/valkey.sh"
  source "${scripts_dir}/garage.sh"

  _CHART_CI_SCRIPTS_LOADED=1
}

# provision_external_services deploys the external PostgreSQL (CloudNativePG),
# Redis (Valkey), and object storage (Garage) instances required by the GitLab CR.
# For vcluster environments the CNPG operator is installed per-namespace; for
# native clusters (GKE, OpenShift) the CNPG operator must be pre-installed
# cluster-wide.
provision_external_services() {
  setup_chart_ci_scripts

  echo 'Provisioning external services'
  deploy_external_valkey
  deploy_external_postgresql
  deploy_external_garage
}

# remove_external_services tears down the external services provisioned by
# provision_external_services.
remove_external_services() {
  setup_chart_ci_scripts

  echo 'Removing external services'
  remove_external_valkey
  remove_external_postgres
  remove_external_garage
}

install_gitlab_operator() {
  echo 'Installing GitLab operator'

  if [ -n "${REGISTRY_AUTH_SECRET}" ]
  then
    export ARGS="--set image.pullSecrets[0].name=${REGISTRY_AUTH_SECRET}"
  fi

  if [[ "$CI_SERVER_HOST" == 'dev.gitlab.org' ]]
  then
    export IMG_REGISTRY='dev.gitlab.org:5005'
    export IMG_REPOSITORY='gitlab/cloud-native'
  fi

  task deploy_operator

  set -x
  cp ${INSTALL_DIR}/operator.yaml ${INSTALL_DIR}/glop-${HOSTSUFFIX}.${DOMAIN}.yaml
  set +x
}

verify_operator_is_running() {
  echo 'Verifying that operator is running'
  kubectl wait --for=condition=Available -n "$TESTS_NAMESPACE" "deployment/${NAME_OVERRIDE}-controller-manager" --timeout 120s
}

build_gitlab_custom_resource() {
  # Ensure chart CI lib scripts are available (needed for dynamic name helpers)
  setup_chart_ci_scripts

  echo 'Building GitLab custom resource manifest'
  echo "Using Chart version: $GITLAB_CHART_VERSION"
  local cr_file="${BUILD_DIR}/gitlab-${HOSTSUFFIX}.${DOMAIN}.yaml"

  task build_test_cr
  set -x
  YQ_CMD="."
  [ -n "${REGISTRY_AUTH_SECRET}" ] && \
    kubectl get secret --namespace="${TESTS_NAMESPACE}" "${REGISTRY_AUTH_SECRET}" && \
    YQ_CMD=".spec.chart.values.global.image.pullSecrets[0].name=\"${REGISTRY_AUTH_SECRET}\""
  ${YQ} eval "${YQ_CMD}" ${BUILD_DIR}/test_cr.yaml  > "${cr_file}"
  [ ${TESTS_NAMESPACE} != "gitlab-system" ] \
    && ${YQ} -i eval ".spec.chart.values.global.ingress.class=\"${NAME_OVERRIDE}-nginx\"" "${cr_file}"

  # Inject dynamic external service configuration derived from chart CI helpers.
  # The helper functions (valkey_release_name, cnpg_cluster_host, etc.) compute
  # deterministic names from CI_PIPELINE_ID so that build and review jobs agree.
  ${YQ} -i eval ".spec.chart.values.global.redis.host = \"$(valkey_release_name)\"" "${cr_file}"
  ${YQ} -i eval ".spec.chart.values.global.redis.auth.secret = \"$(valkey_auth_secret)\"" "${cr_file}"
  ${YQ} -i eval ".spec.chart.values.global.redis.auth.key = \"$(valkey_auth_secret_key)\"" "${cr_file}"

  ${YQ} -i eval ".spec.chart.values.global.psql.host = \"$(cnpg_cluster_host)\"" "${cr_file}"
  ${YQ} -i eval ".spec.chart.values.global.psql.password.secret = \"$(cnpg_cluster_secret)\"" "${cr_file}"

  ${YQ} -i eval ".spec.chart.values.global.psql.host = \"$(cnpg_cluster_host)\"" "${cr_file}"
  ${YQ} -i eval ".spec.chart.values.global.psql.password.secret = \"$(cnpg_cluster_secret)\"" "${cr_file}"
  
  ${YQ} -i eval ".spec.chart.values.global.appConfig.object_store.connection.secret = \"$(garage_release_name)-gitlab-object-storage\"" "${cr_file}"
  ${YQ} -i eval ".spec.chart.values.gitlab.toolbox.backups.objectStorage.config.secret = \"$(garage_release_name)-gitlab-object-storage-s3cmd\"" "${cr_file}"
  ${YQ} -i eval ".spec.chart.values.registry.storage.secret = \"$(garage_release_name)-gitlab-registry-storage\"" "${cr_file}"
  set +x
}

install_gitlab_custom_resource() {
  # requres "build_gitlab_custom_resource" to be ran first
  echo 'Installing GitLab custom resource'
  set -x
  kubectl apply -n ${TESTS_NAMESPACE} -f ${BUILD_DIR}/gitlab-${HOSTSUFFIX}.${DOMAIN}.yaml
  cp ${BUILD_DIR}/gitlab-${HOSTSUFFIX}.${DOMAIN}.yaml ${INSTALL_DIR}/gitlab-${HOSTSUFFIX}.${DOMAIN}.yaml
  set +x
}

copy_certificate() {
  echo 'Copying certificate to namespace'
  kubectl get secret -n default gitlab-ci-tls -o yaml \
    | ${YQ} eval 'del(.metadata.["namespace","resourceVersion","uid","annotations","creationTimestamp","selfLink","managedFields"])' - \
    | kubectl apply -n "$TESTS_NAMESPACE" -f -
}

verify_gitlab_is_running() {
  wait_until_gitlab_running
  test_gitlab_endpoint
}

cleanup() {
  echo 'Cleaning up test resources'
  signal_failure=0

  # Turn off exit immediately if command fails so debug out can get generated
  set +e

  # make sure we know where manifest is:
  prepare_build_directories

  set -x
  # delete CR
  kubectl delete -f ${BUILD_DIR}/gitlab-${HOSTSUFFIX}.${DOMAIN}.yaml
  set +x

  remove_external_services

  task delete_operator

  set -x
  kubectl delete ns "$TESTS_NAMESPACE"
  set +x

  if [[ $? -ne 0 ]]; then
    signal_failure=1
  fi

  if [[ $signal_failure -eq 1 ]]; then
    exit 1
  fi

  # Turn back on to exit immediately
  set -e
}

wait_until_gitlab_running() {
  local sleepSeconds=10
  local maxattempts=60
  local attempts=0
  local exitcode
  local output

  echo 'Verifying that GitLab is running'

  while true; do
    output="$(kubectl -n "$TESTS_NAMESPACE" get gitlab/gitlab -ojsonpath='{.status.phase}' 2>&1)"
    exitcode=$?

    if [ $exitcode -ne 0 ]; then
      echo "$output"; exit $exitcode
    fi

    attempts=$((attempts+1))
    if [ "$attempts" -ge "$maxattempts" ]; then
      echo "Failed waiting for GitLab to be Running, current status is $output"; exit 1;
    fi

    if [[ "$output" == 'Running' ]]; then
      break
    else
      echo -n '.'; sleep $sleepSeconds
    fi
  done
}

test_gitlab_endpoint() {
  local endpoint="https://gitlab-$HOSTSUFFIX.$DOMAIN"

  echo "Testing GitLab endpoint: $endpoint"
  sleep 5
  curl --retry 5 --retry-delay 10 --retry-connrefused -fIL "$endpoint"
}

# main
if [ "$#" -lt 1 ]
then
  main
else
  for cmd in "$@"
  do
    $cmd
  done
fi
