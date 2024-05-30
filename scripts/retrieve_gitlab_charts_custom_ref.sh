#!/bin/bash
#
# This script is used to fetch the GitLab chart from the repository using a custom ref.
# It is used in the Taskfile, during the unit_tests_custom_ref jobs.
# 
# This script depends on yq 4.44.1+.

set -eo pipefail

CHARTS_REF=${CHARTS_REF:-master} 
CHARTS_PROJECT_ID=3828396
API_CHARTS_PROJECT_URL="https://gitlab.com/api/v4/projects/${CHARTS_PROJECT_ID}"

fetch_short_id() {
    local ref=$1
    local charts_ref_encoded=$(printf "${ref}" | yq '. | @uri' -)
    local charts_short_id=$(curl -fsSL \
        "${API_CHARTS_PROJECT_URL}/repository/commits/${charts_ref_encoded}" | \
    yq e '.short_id' -)
    echo "${charts_short_id}"
}

# if used with --ref parameter, print the version and exit
# Taskfile depends on this output, so don't change it or output anything else
if [ "${1}" = "--ref" ]; then
    fetch_short_id "${CHARTS_REF}"
    exit 0
fi

echo "Fetching chart using ref ${CHARTS_REF}"

version=$(fetch_short_id "${CHARTS_REF}")

echo "Fetching chart version ${CHARTS_REF}@${version}"

source="/tmp/chart-${version}"
rm -rf "${source}" && mkdir "${source}"

curl -fsSL \
    "${API_CHARTS_PROJECT_URL}/repository/archive.tar.gz?sha=${version}" | \
tar -xzf - -C "${source}" --strip-component 1

pushd "${source}"
helm dependency update
semver="$(yq eval '.version' Chart.yaml)-${version}"
helm package --version="${semver}" .
popd

mkdir -p charts
mv ${source}/gitlab-*.tgz ./charts/ && rm -rf "${source}"

echo "${semver}" >> CHART_NIGHTLY_VERSION
echo "${CHARTS_REF}@${charts_short_id}" >> CHART_NIGHTLY_VERSION 
