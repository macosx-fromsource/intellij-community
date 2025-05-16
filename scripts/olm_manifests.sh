#!/bin/bash

# This script is used to get the manifest from the OLM catalog,
# check if a version exists in the catalog, and get the latest version from the catalog.
# See usage for more details.
#
# Requirements:
#  * kubectl
#  * jq

MARKETPLACE_NAMESPACE=${MARKETPLACE_NAMESPACE:-"openshift-marketplace"}
set -eo pipefail

echo_stderr() {
    if [[ -t 1 ]]; then
        GREEN='\033[0;32m'
        NO_COL='\033[0m' # No Color
        printf "${GREEN}$*${NO_COL}\n" >&2
    else
        printf "$*\n" >&2
    fi
}

echo_warn() {
    if [[ -t 1 ]]; then
        YELLOW='\033[0;33m'
        NO_COL='\033[0m' # No Color
        printf "${YELLOW}$*${NO_COL}\n" >&2
    else
        printf "$*\n" >&2
    fi
}

getmanifest() {
    if [[ -z "$1" ]]; then
        echo_warn "Please provide a catalog name, available: certified-operators, community-operators, operatorhub"
        return 1
    fi

    echo_stderr "Getting manifest from catalog $1"

    set +e # Disable exit on error
    manifest=$(kubectl get packagemanifest \
        -n "${MARKETPLACE_NAMESPACE}" \
        --selector=catalog=$1 \
        --field-selector metadata.name=gitlab-operator-kubernetes -o json)
    retval=$?
    set -e # Re-enable exit on error
    if [[ "$retval" != 0 ]]; then
        echo_warn "Failed to get manifest from catalog $1"
        return 1
    fi

    # if .items is empty, then the catalog does not have the operator
    if [[ "$(echo "$manifest" | jq -r '.items | length')" == 0 ]]; then
        echo_warn "Catalog $1 does not have the operator"
        return 1
    fi

    echo "$manifest"
}

# Check if version exists in the manifest.
# It always prints the latest version, and will return 1 if version does not exist.
has_version() {
    entries=$(jq -r '.items[] | select( .metadata.name == "gitlab-operator-kubernetes") | .status.channels[] | select( .name == "stable" ) | .entries | sort_by(.version) | reverse')
    latest=$(echo "$entries" | jq -r '.[0].version')
    echo_stderr "Latest version: $latest"
    echo "$latest"

    found=$(echo "$entries" | jq -r '.[] | select(.version == "'"$1"'")')
    if [[ -z "$found" ]]; then
        echo_warn "Version $1 not found"
        return 1
    fi

    echo_stderr "Version $1 found"
}

latest_version() {
    jq -r '.items[] | select( .metadata.name == "gitlab-operator-kubernetes") | .status.channels[] | select( .name == "stable" ) | .entries | sort_by(.version) | reverse | .[0].version'
}

catalog_has_version() {
    manifest=$(getmanifest "$1")
    echo "$manifest" | has_version "$2"
}

catalog_latest_version() {
    manifest=$(getmanifest "$1")
    echo "$manifest" | latest_version
}

usage() {
    echo "Usage: $0 [getmanifest|has_version|latest_version|catalog_latest_version|catalog_has_version]"
    echo "Available commands:"
    echo "  getmanifest <catalog> - Use kubectl to get manifest from a catalog. Available catalog: certified-operators, community-operators, operatorhub"
    echo "  has_version <version> - Check if version exists, read manifest from stdin, it always prints the latest version, and will return 1 if version does not exist."
    echo "  latest_version - Get latest version, read manifest from stdin"
    echo "  catalog_latest_version <catalog> - Get latest version from catalog"
    echo "  catalog_has_version <catalog> <version> - Check if version exists in catalog, see has_version for more details"

    return 1
}

case "$1" in
"getmanifest")
    if [[ "$#" != 2 ]]; then
        echo_warn "Please provide a catalog name"
        usage
    fi

    getmanifest "$2"
    ;;
"has_version")
    if [[ "$#" != 2 ]]; then
        echo_warn "Please provide a version, e.g. 1.0.2"
        usage
    fi

    has_version "$2"
    ;;
"latest_version")
    latest_version
    ;;
"catalog_latest_version")
    if [[ "$#" != 2 ]]; then
        echo_warn "Please provide a catalog name"
        usage
    fi

    catalog_latest_version "$2"
    ;;
"catalog_has_version")
    if [[ "$#" != 3 ]]; then
        echo_warn "Please provide catalog name and version"
        usage
    fi

    catalog_has_version "$2" "$3"
    ;;
*)
    usage
    ;;
esac
