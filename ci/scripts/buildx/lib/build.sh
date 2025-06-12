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

  printf "%s" "$platform_arg"
}

docker_build_and_push() {
  local images=( "$@" )

  # shellcheck disable=SC2046
  docker buildx build \
    $(printf ' -t %s ' "${images[@]}") \
    --platform "$(platform_arg)" \
    --build-arg BUILD_IMAGE="${GO_IMAGE}" \
    --push \
    .
}

sign_digest() {
  local images=("$1")
  local image=${images[0]}
  local digest
  digest="$(skopeo inspect --format="{{index .Digest }}" "docker://${image}")"
  printf "%s@%s" "$image" "$digest"
}

sign() {
  if [ "${SKIP_COSIGN}" != "true" ]; then
    cosign sign "$(sign_digest "$@")"
  fi
}
