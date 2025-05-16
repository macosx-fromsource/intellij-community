#!/usr/bin/env bash
set -ueo pipefail

GITLAB_CHART_VERSION=${GITLAB_CHART_VERSION:-$(sed -n 1p CHART_VERSIONS)}
GITLAB_CHART_DIR=${GITLAB_CHART_DIR:-""}
GITLAB_CHART_REPO=${GITLAB_CHART_REPO:-"https://gitlab.com/gitlab-org/charts/gitlab"}
GITLAB_OPERATOR_DIR=${GITLAB_OPERATOR_DIR:-.}
GITLAB_OPERATOR_MANIFEST=${GITLAB_OPERATOR_MANIFEST:-""}
GITLAB_CR_DEPLOY_MODE=${GITLAB_CR_DEPLOY_MODE:-"selfsigned"}

GITLAB_OPERATOR_DOMAIN="${GITLAB_OPERATOR_DOMAIN:-$USER.cloud-native.win}"
GITLAB_HOST=${GITLAB_HOST:-"*.${GITLAB_OPERATOR_DOMAIN}"}
GITLAB_KEY_FILE=${GITLAB_KEY_FILE:-gitlab.key}
GITLAB_TLSCERTNAME=${GITLAB_TLSCERTNAME:-"custom-gitlab-tls"}
GITLAB_CERT_FILE=${GITLAB_CERT_FILE:-gitlab.crt}
GITLAB_PAGES_HOST=${GITLAB_PAGES_HOST:-"*.pages.${GITLAB_OPERATOR_DOMAIN}"}
GITLAB_PAGES_KEY_FILE=${GITLAB_PAGES_KEY_FILE:-pages.key}
GITLAB_PAGES_CERT_FILE=${GITLAB_PAGES_CERT_FILE:-pages.crt}
GITLAB_ACME_EMAIL="${GITLAB_ACME_EMAIL:-$(cd "${GITLAB_OPERATOR_DIR}" && git config user.email)}"
GITLAB_RUNNER_TOKEN=${GITLAB_RUNNER_TOKEN:-""}

CERT_MANAGER_VERSION=${CERT_MANAGER_VERSION:-"1.6.1"}

KIND=${KIND:-'kind'}
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-gitlab}
KIND_IMAGE=${KIND_IMAGE:-'kindest/node:v1.30.8'}
KIND_LOCAL_IP=${KIND_LOCAL_IP:-""}
KIND_CONFIG=${KIND_CONFIG:-""}

KUBECTL=${KUBECTL:-'kubectl'}
HELM=${HELM:-'helm'}

TASK=${TASK:-'task'}

TARGET_NAMESPACE=${TARGET_NAMESPACE:-"gitlab-system"}

KUBERNETES_TIMEOUT=${KUBERNETES_TIMEOUT:-"300s"}

main(){
  if [ "$1" == 'help' ]; then
    help; exit 0;
  fi

  if [ $# -eq 0 ]; then
    kind_deploy
  else
    for cmd in "$@"
    do
      $cmd
    done
  fi
}

kind_deploy(){
  if [ -z "${KIND_LOCAL_IP}" ]; then
    echo 'Error: KIND_LOCAL_IP variable is unset'; exit 1;
  else
    GITLAB_OPERATOR_DOMAIN="${KIND_LOCAL_IP}.nip.io"
  fi
  create_kind_cluster
  create_namespace
  install_certmanager
  wait_for_certmanager
  create_gitlab_cert
  deploy_gitlab_cert
  create_pages_cert
  deploy_pages_cert
  deploy_operator
  wait_for_operator
  deploy_gitlab "$GITLAB_CR_DEPLOY_MODE"
}

generic_deploy(){
  create_namespace
  install_certmanager
  wait_for_certmanager
  deploy_operator
  wait_for_operator
  deploy_gitlab "$GITLAB_CR_DEPLOY_MODE"
}

runner_deploy(){
  wait_for_gitlab
  # Run this to if you do not have pre-generated
  [ "$GITLAB_CR_DEPLOY_MODE" != "selfsigned" ] && fetch_gitlab_cert
  fetch_runner_token
  install_runner "$GITLAB_CR_DEPLOY_MODE"
}

create_kind_cluster(){
  if [ -z "${GITLAB_CHART_DIR+}" -a -z "${KIND_CONFIG+}" ]; then
    echo "Missing one of the required env vars:"
    echo " GITLAB_CHART_DIR: ${GITLAB_CHART_DIR+}"
    echo " KIND_CONFIG: ${KIND_CONFIG+}"
    echo " auto-generating KIND_CONFIG..."
    KIND_CONFIG=$(mkdir -p .build; mktemp -p .build kind-ssl.XXXXX)
    export KIND_CONFIG
    _cleanup_kind_config="true"
    curl -o "${KIND_CONFIG}" "${GITLAB_CHART_REPO}/-/raw/master/examples/kind/kind-ssl.yaml"
  elif [ -n "${GITLAB_CHART_DIR+}" -a -z "${KIND_CONFIG}" ]; then
    KIND_CONFIG="${GITLAB_CHART_DIR}/examples/kind/kind-ssl.yaml"
  fi
  ${KIND} create cluster --name="${KIND_CLUSTER_NAME}" --config="${KIND_CONFIG}" --image="${KIND_IMAGE}"
}

create_namespace(){
    $KUBECTL get namespace -o name ${TARGET_NAMESPACE} > /dev/null 2>&1 || $KUBECTL create namespace ${TARGET_NAMESPACE}
}

install_certmanager(){
  $KUBECTL apply -f https://github.com/jetstack/cert-manager/releases/download/v${CERT_MANAGER_VERSION}/cert-manager.yaml
}

create_gitlab_cert(){
  openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout "${GITLAB_KEY_FILE}" -out "${GITLAB_CERT_FILE}" \
    -subj "/CN=${GITLAB_HOST}/O=${GITLAB_HOST}"
}

deploy_gitlab_cert(){
  $KUBECTL create secret -n ${TARGET_NAMESPACE} tls ${GITLAB_TLSCERTNAME} --key="${GITLAB_KEY_FILE}" --cert="${GITLAB_CERT_FILE}"
}

create_pages_cert(){
  openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout "${GITLAB_PAGES_KEY_FILE}" -out "${GITLAB_PAGES_CERT_FILE}" \
    -subj "/CN=${GITLAB_PAGES_HOST}/O=${GITLAB_PAGES_HOST}"
}

deploy_pages_cert(){
  $KUBECTL create secret -n ${TARGET_NAMESPACE} tls custom-pages-tls --key="${GITLAB_PAGES_KEY_FILE}" --cert="${GITLAB_PAGES_CERT_FILE}"
}

deploy_operator(){
  if [ -z "${GITLAB_OPERATOR_MANIFEST}" ]; then
    cd "${GITLAB_OPERATOR_DIR}"
    $TASK build_operator
    $KUBECTL apply -f .build/operator.yaml
  else
    $KUBECTL apply -f "${GITLAB_OPERATOR_MANIFEST}"
  fi
}

deploy_gitlab(){
  local template=$(cat scripts/manifests/gitlab-cr-${GITLAB_CR_DEPLOY_MODE}.yaml.tpl)

  $KUBECTL -n ${TARGET_NAMESPACE} apply -f <(eval "echo \"$template\"")
}

install_runner(){

  if [ "${GITLAB_CR_DEPLOY_MODE}" == 'selfsigned' ]; then
    GITLAB_CERTS_SECRET_NAME='custom-runner-tls'
  else
    GITLAB_CERTS_SECRET_NAME='null'
  fi

  local template=$(cat scripts/manifests/gitlab-runner-cr.yaml.tpl)

  $HELM upgrade --install -n ${TARGET_NAMESPACE} gitlab-runner gitlab/gitlab-runner -f <(eval "echo \"$template\"")
}

fetch_runner_token(){
  if [ -z "${GITLAB_RUNNER_TOKEN}" ]; then
    GITLAB_RUNNER_TOKEN=$($KUBECTL -n ${TARGET_NAMESPACE} get secret gitlab-gitlab-runner-secret -o jsonpath='{.data}' | jq -r '.["runner-registration-token"]' | base64 --decode)
  fi
}

fetch_gitlab_cert(){
  openssl s_client -connect "gitlab.${GITLAB_OPERATOR_DOMAIN}:443" 2>/dev/null </dev/null |  sed -ne '/-BEGIN CERTIFICATE-/,/-END CERTIFICATE-/p' > "${GITLAB_CERT_FILE}"
}

# private function since we can't really pass parameters 
# from CLI
_wait_for_resource(){
  local namespace=${1}
  local resource=${2}
  local wait_loop="until $KUBECTL get -n ${namespace} ${resource} ; do echo 'waiting'; sleep 5; done"
  # make sure it's been created
  timeout ${KUBERNETES_TIMEOUT} bash -c "${wait_loop}"
  $KUBECTL wait --for=condition=Available --timeout=${KUBERNETES_TIMEOUT} -n ${namespace} ${resource}
}

wait_for_operator(){
  _wait_for_resource ${TARGET_NAMESPACE} deployment/gitlab-controller-manager
}

wait_for_certmanager(){
  _wait_for_resource cert-manager deployment/cert-manager
  # have to wait for cert-manager-webhook service
  _wait_for_resource cert-manager deployment/cert-manager-webhook
}

wait_for_gitlab(){
  _wait_for_resource ${TARGET_NAMESPACE} deployment/gitlab-webservice-default
}

help(){
  grep -F '()''{' "$0" | sed -e 's/(.*$//'
}

main "$@"
