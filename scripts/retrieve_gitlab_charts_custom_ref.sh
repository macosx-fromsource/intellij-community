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

short_id=$(fetch_short_id "${CHARTS_REF}")

echo "Fetching chart version ${CHARTS_REF}@${short_id}"

source="/tmp/chart-${short_id}"
rm -rf "${source}" && mkdir "${source}"

curl -fsSL \
    "${API_CHARTS_PROJECT_URL}/repository/archive.tar.gz?sha=${short_id}" | \
tar -xzf - -C "${source}" --strip-component 1

pushd "${source}"
helm dependency update
chart_ver="$(yq eval '.version' Chart.yaml)"

# The chart version on master is only bumped during the release.
# We already bump the chart version here to to allow the Operator to test/apply logic specific to the upcoming release.
if [[ "${CHARTS_REF}" == "master" ]]; then
  IFS='.' read -r major minor patch <<< "$chart_ver"
  if (( minor == 11 )); then
    chart_ver="$((major + 1)).0.0"
  else
    chart_ver="$major.$((minor + 1)).0"
  fi
fi

semver="${chart_ver}+${short_id}"
helm package --version="${semver}" .
popd

mkdir -p charts
mv ${source}/gitlab-*.tgz ./charts/ && rm -rf "${source}"

echo "${semver}" >> CHART_NIGHTLY_VERSION
echo "${CHARTS_REF}@${short_id}" >> CHART_NIGHTLY_VERSION
