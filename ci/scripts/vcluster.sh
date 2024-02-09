#!/bin/bash

function vcluster_name() {
  local vcluster_name
  # trim the vcluster name to 54 characters for helm
  vcluster_name="vc1${VCLUSTER_K8S_MINOR_VERSION}-${TESTS_NAMESPACE:0:47}"
  # remove any trailing hyphens
  shopt -s extglob
  vcluster_name=${vcluster_name%%+(-)}
  echo ${vcluster_name}
}

function kas_cluster_connect() {
  if [ -z ${AGENT_NAME+x} ] || [ -z ${AGENT_PROJECT_PATH+x} ]; then
    echo "No AGENT_NAME or AGENT_PROJECT_PATH set, using the default"
  else
    kubectl config get-contexts
    kubectl config use-context "${AGENT_PROJECT_PATH}:${AGENT_NAME}"
  fi
}

function vcluster_create() {
  vcluster create "${VCLUSTER_NAME}" \
    --upgrade \
    --namespace="${VCLUSTER_NAME}" \
    --kubernetes-version=1."${VCLUSTER_K8S_MINOR_VERSION}" \
    --connect=false \
    --update-current=false
  kubectl label --overwrite namespace "${VCLUSTER_NAME}" release="${TESTS_NAMESPACE}"
}

function vcluster_run() {
  vcluster connect "${VCLUSTER_NAME}" -- "$@"
}

function vcluster_delete() {
  vcluster delete "${VCLUSTER_NAME}"
}

function vcluster_delete_all() {
  kubectl delete namespace --selector=release="${TESTS_NAMESPACE}"
}

function vcluster_info() {
  echo "To connect to the virtual cluster:"
  echo "1. Connect to host cluster via kubectl: ${AGENT_NAME}"
  echo "2. Connect to virtual cluster: vcluster connect ${VCLUSTER_NAME}"
  echo "3. Open a separate terminal window and run your kubectl and helm commands."
}
