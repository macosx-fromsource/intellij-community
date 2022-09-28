#!/bin/sh -e
# Note: POSIX only! This script is expected to run in busybox ash.

# This script executes during the image_build job of the pipeline
# and is responsible for retrieving the correct versions of the
# GitLab chart. These charts are then baked into the operator
# container image when the Dockerfile is processed.

HELM="bin/helm"
GITLAB_CHART="gitlab/gitlab"
MAX_CHART_FETCH_ATTEMPTS=${MAX_CHART_FETCH_ATTEMPTS:-100}
CHART_FETCH_WAIT_TIME=${CHART_FETCH_WAIT_TIME:-30s}

scripts_dir="$(dirname "$0")"
. "${scripts_dir}/add_gitlab_repo.sh"

# Download the charts to the charts directory
rm -rf charts && mkdir charts
for version in $(cat CHART_VERSIONS); do
    count=0
    echo "Fetching ${GITLAB_CHART}-${version}"
    while ! $($HELM fetch "${GITLAB_CHART}" --version "${version}" --destination ./charts/ 2> /dev/null); do
        if [ $count -ge "${MAX_CHART_FETCH_ATTEMPTS}" ]; then
            echo "  Fetch attempts exhausted. Exiting."
            exit 1;
        fi

        echo "  Could not fetch chart. Sleeping for ${CHART_FETCH_WAIT_TIME} before attempting again."
        sleep "${CHART_FETCH_WAIT_TIME}"
        count=$((count+1))
    done
done
