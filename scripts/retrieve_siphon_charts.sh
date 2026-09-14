#!/bin/bash
#
# This script executes during the image_build job of the pipeline and is
# responsible for retrieving the correct versions of the Siphon chart. These
# charts are then baked into the operator container image when the Dockerfile is
# processed.
#
# Unlike retrieve_gitlab_charts.sh this never clears the charts directory: both
# scripts write into it, and this one runs second.

set -eo pipefail

HELM="bin/helm"
SIPHON_CHART="siphon/siphon"
SIPHON_HELM_REPO="https://gitlab.com/api/v4/projects/76780115/packages/helm/stable"
MAX_CHART_FETCH_ATTEMPTS=${MAX_CHART_FETCH_ATTEMPTS:-10}
CHART_FETCH_WAIT_TIME=${CHART_FETCH_WAIT_TIME:-30s}

scripts_dir="$(dirname "$0")"
. "${scripts_dir}/install_helm.sh"

echo "Adding ${SIPHON_HELM_REPO} to list of helm repos"
$HELM repo list | grep -q '^siphon' || $HELM repo add siphon "${SIPHON_HELM_REPO}"
$HELM repo update

mkdir -p charts

for version in $(cat SIPHON_CHART_VERSIONS); do
    count=0
    echo "Fetching ${SIPHON_CHART}-${version}"
    while ! $HELM fetch "${SIPHON_CHART}" --version "${version}" --destination ./charts/ 2> /dev/null; do
        if [ $count -ge "${MAX_CHART_FETCH_ATTEMPTS}" ]; then
            echo "  Fetch attempts exhausted. Exiting."
            exit 1
        fi

        echo "  Could not fetch chart. Sleeping for ${CHART_FETCH_WAIT_TIME} before attempting again."
        sleep "${CHART_FETCH_WAIT_TIME}"
        count=$((count+1))
    done
done
