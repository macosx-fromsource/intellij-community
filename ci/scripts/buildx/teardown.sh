#!/bin/bash 
#
# Removes the buildx namespace and driver.
#
# Depends on the environments variables and CLI setup
# from lib/configure.sh.

set -euo pipefail

if [ "${BUILDX_K8S_DISABLE}" == "true" ]; then
  echo "Skipping buildx teardown"
  exit 0
fi

docker buildx stop ${BUILDX_K8S_NAMESPACE}
docker buildx rm ${BUILDX_K8S_NAMESPACE}
kubectl delete namespace ${BUILDX_K8S_NAMESPACE} || true
