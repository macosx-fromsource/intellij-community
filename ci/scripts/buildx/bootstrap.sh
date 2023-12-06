#!/bin/bash
#
# Creates the buildx namespace and bootstraps the driver
# (this creates the pods for the buildkit backend).
#
# Depends on the environments variables and CLI setup
# from lib/configure.sh.

set -euo pipefail

if [ "${BUILDX_K8S_DISABLE}" == "true" ]; then
  echo "Skipping buildx bootstrap"
  exit 0
fi

kubectl get namespace "${BUILDX_K8S_NAMESPACE}" || {
  echo "Creating namespace ${BUILDX_K8S_NAMESPACE}"
  kubectl create namespace "${BUILDX_K8S_NAMESPACE}"
}

docker buildx inspect --bootstrap
