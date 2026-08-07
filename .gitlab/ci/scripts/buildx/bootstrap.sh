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
else
  kubectl get namespace "${BUILDX_K8S_NAMESPACE}" || {
    echo "Creating namespace ${BUILDX_K8S_NAMESPACE}"
    kubectl create namespace "${BUILDX_K8S_NAMESPACE}"
    until kubectl get namespace ${BUILDX_K8S_NAMESPACE}; do sleep 2; done
  }

  NEW_EXPIRY=$(kubectl run -it -n ${BUILDX_K8S_NAMESPACE} datecheck-${CI_JOB_ID} --image=busybox --rm --restart=Never -q --command -- /bin/sh -c 'date "+%Y-%m-%dT%H:%M" --date="@$(expr $(date +%s) + 6 \* 60 \* 60)"')
  kubectl annotate --overwrite namespace ${BUILDX_K8S_NAMESPACE} "janitor/expires=${NEW_EXPIRY}"

  docker buildx inspect --bootstrap
fi