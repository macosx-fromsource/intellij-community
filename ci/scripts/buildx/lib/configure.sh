#!/bin/bash
#
# Configures buildx, kubectl and gcloud.
# Does not create a namespace or any buildx worker pods.
# 
# This script should be sourced to make the exported environment
# variables accessible for other buildx scripts.


if [ "${BUILDX_K8S_DISABLE}" == "true" ]; then
  echo "Skipping buildx configure"
else
  gcloud auth activate-service-account --key-file="${BUILDX_K8S_SA_JSON}" --project "${BUILDX_K8S_GCLOUD_PROJECT}"
  export KUBECONFIG="${PWD}/kubeconfig"
  export USE_GKE_GCLOUD_AUTH_PLUGIN='True'
  gcloud container clusters get-credentials "${BUILDX_K8S_CLUSTER_NAME}" --zone "${BUILDX_K8S_CLUSTER_ZONE}"

  # Prefix namespace to avoid collision
  BUILDX_K8S_NAMESPACE=${BUILDX_K8S_NAMESPACE:-operator-build-${CI_COMMIT_REF_SLUG}}
  # Trim namespace to comply with kubernetes requirements
  BUILDX_K8S_NAMESPACE=${BUILDX_K8S_NAMESPACE:0:63}
  # Trim trailing "-" from namespace
  export BUILDX_K8S_NAMESPACE=${BUILDX_K8S_NAMESPACE%%+(-)}

  _k8s_driver_opt="namespace=${BUILDX_K8S_NAMESPACE},replicas=1,loadbalance=sticky"
  _k8s_driver_opt="${_k8s_driver_opt},requests.cpu=${BUILDX_K8S_REQUEST_CPU},requests.memory=${BUILDX_K8S_REQUEST_MEMORY}"

  create_flag="--use"
  for arch in ${BUILDX_ARCHS//,/ }; do 
    docker buildx create ${create_flag} \
      --name="${BUILDX_K8S_NAMESPACE}" \
      --platform=linux/"${arch}" \
      --node=buildx-"${arch}" \
      --driver=kubernetes \
      --driver-opt="${_k8s_driver_opt},nodeselector=kubernetes.io/arch=${arch}"

    create_flag="--append"
  done
fi
