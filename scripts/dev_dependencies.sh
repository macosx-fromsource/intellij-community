#!/bin/bash

# Sets up external dependencies (Valkey, CloudNativePG, Garage) for local operator development.
# The CI library scripts are downloaded from the GitLab Charts repository at runtime,
# matching the setup used by scripts/test.sh in CI.

set -eo pipefail
[[ "${TRACE}" ]] && set -x

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}/.."

# GitLab chart repository for sourcing CI scripts (same defaults as scripts/test.sh)
CHART_REPO_URL="${CHART_REPO_URL:-https://gitlab.com/gitlab-org/charts/gitlab}"
CHART_CI_LIB_REF="${CHART_CI_LIB_REF:-master}"

NAMESPACE="${NAMESPACE:-gitlab-system}"
GARAGE_APP_VERSION="${GARAGE_APP_VERSION:-2.2.0}"
CNPG_POSTGRESQL_TAG="${CNPG_POSTGRESQL_TAG:-17}"
OUTPUT_FILE="${OUTPUT_FILE:-external-deps.yaml}"
export NAMESPACE GARAGE_APP_VERSION CNPG_POSTGRESQL_TAG

function check_prerequisites() {
  for tool in kubectl helm curl; do
    if ! command -v "${tool}" > /dev/null 2>&1; then
      echo "ERROR: ${tool} is required but not installed."
      echo "See doc/developer/guide.md for guidance."
      exit 1
    fi
  done
}

function ensure_namespace() {
  kubectl get namespace "${NAMESPACE}" > /dev/null 2>&1 \
    || kubectl create namespace "${NAMESPACE}"
  echo "    Namespace: ${NAMESPACE}"
}

# setup_chart_ci_scripts downloads and sources the CI library scripts from the
# GitLab Charts repository. The same scripts are used in CI via scripts/test.sh
# to avoid drift between local and CI environments.
function setup_chart_ci_scripts() {
  [ -n "${_CHART_CI_SCRIPTS_LOADED:-}" ] && return 0

  echo "Downloading chart CI lib scripts from ${CHART_REPO_URL} @ ${CHART_CI_LIB_REF}"
  local scripts_dir="${PROJECT_ROOT}/.build/chart-ci-lib"
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

function cmd_generate_cr() {
  cat > "${OUTPUT_FILE}" <<EOF
apiVersion: apps.gitlab.com/v1beta1
kind: GitLab
metadata:
  name: gitlab
spec:
  chart:
    values:
      global:
        redis:
          host: $(valkey_release_name)
          auth:
            secret: $(valkey_auth_secret)
            key: $(valkey_auth_secret_key)
        psql:
          host: $(cnpg_cluster_host)
          password:
            secret: $(cnpg_cluster_secret)
            key: password
        pages:
          objectStore:
            connection:
              secret: $(garage_release_name)-gitlab-object-storage
              key: config
        appConfig:
          object_store:
            connection:
              secret: $(garage_release_name)-gitlab-object-storage
              key: config
        minio:
          install: false
      postgresql:
        install: false
      redis:
        install: false
      gitlab:
        toolbox:
          backups:
            objectStorage:
              config:
                secret: $(garage_release_name)-gitlab-object-storage-s3cmd
                key: config
      registry:
        storage:
          secret: $(garage_release_name)-gitlab-registry-storage
          key: config
EOF

  echo "==> Generated ${OUTPUT_FILE}"
  echo "    Manually merge these values into your mygitlab.yaml under spec.chart.values,"
  echo "    then apply with:"
  echo "    kubectl -n ${NAMESPACE} apply -f mygitlab.yaml"
}

function cmd_setup() {
  echo "Setting up external dependencies in namespace '${NAMESPACE}'..."
  echo ""
  check_prerequisites
  setup_chart_ci_scripts
  ensure_namespace

  echo "==> Setting up Valkey..."
  deploy_external_valkey

  echo "==> Setting up CloudNativePG..."
  install_cnpg_operator
  deploy_external_postgresql

  echo "==> Setting up Garage..."
  deploy_external_garage

  echo ""
  echo "==> All external dependencies are ready."
  echo ""

  cmd_generate_cr
}

function cmd_teardown() {
  echo "Removing external dependencies from namespace '${NAMESPACE}'..."
  echo ""
  setup_chart_ci_scripts

  echo "    Removing Valkey..."
  remove_external_valkey

  echo "    Removing CloudNativePG cluster..."
  remove_external_postgresql

  echo "    Removing Garage..."
  remove_external_garage
  echo ""
  echo "==> External dependencies removed."
  echo ""
  echo "The namespace '${NAMESPACE}' and GitLab operator release are not affected."
  echo "Run 'helm uninstall gitlab-operator --namespace ${NAMESPACE}' to remove the operator as well."
}

function cmd_status() {
  setup_chart_ci_scripts

  echo "External dependency status in namespace '${NAMESPACE}':"
  echo ""

  echo "--- Valkey ($(valkey_release_name)) ---"
  kubectl get deployment \
    --namespace "${NAMESPACE}" \
    -l "app.kubernetes.io/instance=$(valkey_release_name)" 2>/dev/null \
    || echo "  Not found"

  echo ""
  echo "--- CloudNativePG operator ($(cnpg_release_name)) ---"
  kubectl get deployment \
    --namespace "${NAMESPACE}" \
    -l "app.kubernetes.io/instance=$(cnpg_release_name)" 2>/dev/null \
    || echo "  Not found"

  echo ""
  echo "--- PostgreSQL cluster ($(cnpg_cluster_name)) ---"
  kubectl get cluster \
    --namespace "${NAMESPACE}" \
    "$(cnpg_cluster_name)" 2>/dev/null \
    || echo "  Not found"

  echo ""
  echo "--- Garage ---"
  kubectl get statefulset \
    --namespace "${NAMESPACE}" \
    -l app.kubernetes.io/name=garage 2>/dev/null \
    || echo "  Not found"

  echo ""
  echo "--- Object storage secrets ---"
  for secret in \
    "$(garage_release_name)-gitlab-object-storage" \
    "$(garage_release_name)-gitlab-object-storage-s3cmd" \
    "$(garage_release_name)-gitlab-registry-storage"; do
    kubectl get secret --namespace "${NAMESPACE}" "${secret}" \
      -o jsonpath="  {.metadata.name}: present{'\n'}" 2>/dev/null \
      || echo "  ${secret}: not found"
  done
}

function usage() {
  cat <<EOF
Usage: $0 {setup|teardown|status}

  setup    Deploy Valkey, CloudNativePG, and Garage as external GitLab dependencies.
           Generates an external-deps.yaml CR with the connection values to merge into your mygitlab.yaml.
  teardown Remove the deployed external dependencies (does not remove the operator release).
  status   Show the current status of the external dependencies.

Environment variables:
  NAMESPACE           Kubernetes namespace to use (default: gitlab-system)
  OUTPUT_FILE         Output path for the generated external deps CR (default: external-deps.yaml)
  GARAGE_APP_VERSION  Garage version to install (default: 2.2.0)
  CNPG_POSTGRESQL_TAG PostgreSQL image tag for CloudNativePG (default: 17)
  CHART_REPO_URL      GitLab Charts repository URL (default: https://gitlab.com/gitlab-org/charts/gitlab)
  CHART_CI_LIB_REF    Git ref for CI library scripts (default: master)

See doc/developer/installation.md#external-dependencies for full documentation.
EOF
  exit 1
}

case "${1:-}" in
  setup)    cmd_setup ;;
  teardown) cmd_teardown ;;
  status)   cmd_status ;;
  *)        usage ;;
esac
