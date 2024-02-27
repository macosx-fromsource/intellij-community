#!/bin/bash

function vcluster_name() {
  local vcluster_name
  # Trim the vcluster name to 52 characters:
  # 1. Helm release names are limited to 53 characters.
  # 2. The vcluster chart creates a StatefulSet with the release name:
  #    To ensure that the Pod labels do not exceed 63 characters,
  #    the StatefulSet name should not exceed 52 characters.
  vcluster_name="vc1${VCLUSTER_K8S_MINOR_VERSION}-${TESTS_NAMESPACE:0:46}"
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
