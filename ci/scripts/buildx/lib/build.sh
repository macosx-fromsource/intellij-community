#!/bin/bash
#
# Helper functions to build multiarch images using buildx.
# Depends on a bootstrapped and configured buildx drivers.
#
# Usage:
#   docker buildx create ...
#   docker buildx configure --bootstrap
#   export BUILDX_ARCHS=amd64,arm64
#   source lib/build.sh

platform_arg() {
  local platform_arg=""
  local delim=""

  [ "$BUILDX_K8S_DISABLE" == "true" ] && BUILDX_ARCHS="amd64"
  for arch in ${BUILDX_ARCHS//,/ }; do 
    platform_arg="${platform_arg}${delim}linux/${arch}";
    delim=",";
  done

  printf "$platform_arg"
}

docker_build_and_push() {
  docker buildx build \
    $(printf ' -t %s ' $*) \
    --platform "$(platform_arg)" \
    --push \
    .
}

