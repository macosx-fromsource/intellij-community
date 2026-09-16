#!/bin/bash
#
# Helpers to build and sign the multi-arch Operator image.
#
# Depends on a bootstrapped buildx builder that can emit BUILD_PLATFORMS
# (see .docker_build_job in .gitlab-ci.yml).
#
# Usage:
#   source .gitlab/ci/scripts/docker_build.sh
#   docker_build_and_push "${DOCKER_TAGS}"
#   sign "${DOCKER_TAGS}"

# Where docker_build_and_push records what it pushed, for sign to read back.
# Outside the repository, so it never becomes part of the build context.
BUILD_METADATA_FILE="${BUILD_METADATA_FILE:-/tmp/docker-build-metadata.json}"

docker_build_and_push() {
  local images=( "$@" )

  # `oci-artifact=false` emits the legacy attestation format, which has no
  # `subject` field. The registry rejects a manifest whose `subject` is not
  # present yet, and BuildKit pushes the attestation concurrently with the
  # manifest it names, so a plain `--push` fails intermittently with
  # "blob unknown to registry" naming that manifest.
  #
  # Do not drop this flag for a newer BuildKit: it pushes the same way by
  # design, the OCI spec requiring registries to accept an absent `subject`.
  # The fix is registry-side, tracked at
  # https://gitlab.com/gitlab-org/container-registry/-/work_items/2375

  # shellcheck disable=SC2046
  docker buildx build \
    -f "${DOCKERFILE:-Dockerfile}" \
    $(printf ' -t %s ' "${images[@]}") \
    --platform "${BUILD_PLATFORMS}" \
    --build-arg BUILD_IMAGE="${GO_IMAGE}" \
    --output "type=image,push=true,oci-artifact=false" \
    --metadata-file "${BUILD_METADATA_FILE}" \
    .
}

# Digest of the index the build just pushed, as the build itself reported it.
# Resolving a tag instead would race any other pipeline pushing the same tag.
built_digest() {
  jq -er '."containerimage.digest"' "${BUILD_METADATA_FILE}"
}

# Strip the tag from each image reference, leaving the repositories it names.
# A tag is the last `:`-delimited component with no `/` in it, so a registry
# host:port in an untagged reference is left alone.
repositories() {
  printf '%s\n' "$@" | sed 's/:[^:/]*$//' | sort -u
}

# One signature per repository covers every tag pointing into it: cosign stores
# the signature against the digest, at <repo>:sha256-<digest>.sig, and
# `cosign verify <repo>:<tag>` resolves the tag to its digest first. A single
# build pushes one index to all of DOCKER_TAGS, and every tag is
# ${CI_REGISTRY_IMAGE}:<something>, so in practice this signs once.
sign() {
  local digest repo
  digest="$(built_digest)" || return 1

  while read -r repo; do
    cosign sign "${repo}@${digest}" || return 1
  done < <(repositories "$@")
}
