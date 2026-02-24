#!/bin/sh

set -eu

OPENSHIFT_MIN=${OPENSHIFT_MIN:-"4.12"}
OPENSHIFT_MAX=${OPENSHIFT_MAX:-"4.21"}
OLM_PACKAGE_NAME="${OLM_PACKAGE_NAME:-"gitlab-operator-kubernetes"}"
OLM_SKIP_VERSION=${OLM_SKIP_VERSION:-""}

BUNDLE_DIR=${BUNDLE_DIR:-"."}

BUNDLE_DIR=$(realpath ${BUNDLE_DIR})

YQ=${YQ:-yq}

adjust_annotations() {
    local version_range="v${OPENSHIFT_MIN}-v${OPENSHIFT_MAX}"
    "${YQ}" eval -i '.annotations["com.redhat.openshift.versions"]="'"$version_range"'"' "${BUNDLE_DIR}"/metadata/annotations.yaml
}

adjust_csv() {
    local csv_files=$(grep -l 'kind: ClusterServiceVersion' "${BUNDLE_DIR}"/manifests/*.yaml)
    for csv in $csv_files; do
        ${YQ} eval -i '.metadata.annotations["olm.properties"]="[{\"type\": \"olm.maxOpenShiftVersion\", \"value\": \"'${OPENSHIFT_MAX}'\"}]"' $csv
    done
}

set_upgrade_path() {
    local csv_files=$(grep -l 'kind: ClusterServiceVersion' "${BUNDLE_DIR}"/manifests/*.yaml)
    for csv in $csv_files; do
        if [ -n "${OLM_SKIP_VERSION}" ]; then
            ${YQ} eval -i ".spec.skips=[\"${OLM_PACKAGE_NAME}.v${OLM_SKIP_VERSION}\"]" $csv
        fi
        if [ -n "${PREVIOUS_OPERATOR_VERSION}" ]; then
            ${YQ} eval -i ".spec.replaces=\"${OLM_PACKAGE_NAME}.v${PREVIOUS_OPERATOR_VERSION}\"" $csv
        fi
    done
}

for cmd in $@; do
    $cmd
done
