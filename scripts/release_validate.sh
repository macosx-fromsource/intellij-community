#!/bin/bash

# This script is used to validate the release version in the OLM catalogs.
# It calls the olm_manifests.sh script to check if the version exists in the catalogs.
# It also has the option to comment on the GitLab issue with the status of the release.
# If a version is not found in the catalogs, it will return a non-zero exit code.
# See usage for more details.
#
# Requirements:
#  * kubectl
#  * jq
#  * curl

set -eo pipefail

SCRIPT_DIR=$(dirname "$0")

# Required environment variables
OPERATOR_RELEASE_VERSION=${1:-$OPERATOR_RELEASE_VERSION}
# Optional environment variables for commenting on the issue
OPERATOR_RELEASE_COMMENT=${OPERATOR_RELEASE_COMMENT:-""}
OPERATOR_RELEASE_ISSUE_ID=${OPERATOR_RELEASE_ISSUE_ID:-""}
OPERATOR_RELEASE_PROJECT_ID=${OPERATOR_RELEASE_PROJECT_ID:-""}
OPERATOR_RELEASE_TOKEN=${OPERATOR_RELEASE_TOKEN:-""}

manifest() {
    $SCRIPT_DIR/olm_manifests.sh "$@"
}

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

# if any of the checks fail, the return value will be 1
validate() {
    retval=0
    echo_stderr "Validating operator release ${OPERATOR_RELEASE_VERSION}"

    set +e # Disable exit on error
    for catalog in operatorhub community-operators certified-operators; do
        echo_stderr "Checking ${catalog}"
        latest=$(manifest catalog_has_version ${catalog} ${OPERATOR_RELEASE_VERSION})
        if [[ "$?" == 0 ]]; then
            echo "✅ Catalog ${catalog} has version ${OPERATOR_RELEASE_VERSION}, latest version is ${latest}"
        else
            echo "❌ Catalog ${catalog} does not have version ${OPERATOR_RELEASE_VERSION}, latest version is ${latest:-"unknown"}"
            retval=1
        fi
    done
    set -e # Re-enable exit on error

    return $retval
}

comment_on_issue() {
    # Escape newlines for markdown
    comment=""
    while IFS= read -r line; do
        comment+="$line"$'\n\n'
    done <<<"$1"

    echo_stderr "Commenting on issue ${OPERATOR_RELEASE_ISSUE_ID}"

    set +e # Disable exit on error
    curl -fsSL -X POST \
        -H "Content-Type: application/json" \
        -H "PRIVATE-TOKEN: ${OPERATOR_RELEASE_TOKEN}" \
        -d "$(comment=$comment jq -n '{"body": env.comment }')" \
        "https://gitlab.com/api/v4/projects/${OPERATOR_RELEASE_PROJECT_ID}/issues/${OPERATOR_RELEASE_ISSUE_ID}/notes" >/dev/null
    retval=$?
    set -e # Re-enable exit on error

    if [[ "$retval" != 0 ]]; then
        echo_warn "Failed to comment on issue ${OPERATOR_RELEASE_ISSUE_ID}"
        return 1
    fi
}

usage() {
    echo "Usage: $0 <release>"
    echo "  release: The release version to check"
    echo "You can also set the OPERATOR_RELEASE_VERSION environment variables"
    echo "The parameters take precedence over the environment variables"
    echo "If a version is not found in the catalogs, the script will return a non-zero exit code"
    echo ""
    echo "Optionally, you can set the following environment variables to comment the status on the issue:"
    echo "  OPERATOR_RELEASE_ISSUE_ID: The GitLab issue ID for the release"
    echo "  OPERATOR_RELEASE_PROJECT_ID: The GitLab project ID for the release"
    echo "  OPERATOR_RELEASE_TOKEN: The GitLab token used for commenting on the release issue"
    return 1
}

if [[ ! "$OPERATOR_RELEASE_VERSION" ]]; then
    echo_warn "Please provide a operator release version"
    usage
fi

if [[ "$OPERATOR_RELEASE_COMMENT" ]]; then
    if [[ ! "$OPERATOR_RELEASE_ISSUE_ID" || ! "$OPERATOR_RELEASE_PROJECT_ID" || ! "$OPERATOR_RELEASE_TOKEN" ]]; then
        echo_warn "Please provide the OPERATOR_RELEASE_ISSUE_ID, OPERATOR_RELEASE_PROJECT_ID, and OPERATOR_RELEASE_TOKEN environment variables"
        usage
    fi
fi

set +e # Disable exit on error
comment=$(validate)
retval=$?
set -e # Re-enable exit on error

echo "$comment"

if [[ "$OPERATOR_RELEASE_COMMENT" ]]; then
    comment_on_issue "$comment"
fi

exit $retval
