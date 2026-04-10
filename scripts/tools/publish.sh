#!/usr/bin/env bash

#
# A helper script to publish GitLab Operator to OperatorHub, RedHat Marketplace,
# and RedHat Community Operators.
#
# Note: Use mise to install dependencies.
# Note: Use GNU sed. The script is not portable to MacOS sed.
# Note: The script uses global Git config such as `user.name` and `user.email`.
#
# Usage:
#
#   scripts/tools/publish.sh VERSION [TARGETS]
#
#   VERSION    Operator version that is published.
#   TARGET     Can be one of `operatorhub`, `redhat-community`, or
#              `redhat-marketplace`. If not specified all targets are attempted.
#
# Also uses the following environemnt variables:
#
#   SSH_KEY_FILE       defaults to `.build/gl-distribution-oc` and is used for Git authentication.
#   GITHUB_TOKEN_FILE  defaults to `.build/gh-token` and is used for GitHub authentication.
#   GITHUB_ACCOUNT     defaults to `gl-distribution-oc` and is used for publication.
#   SKIP_CHECKS        defaults to ``, when not empty skips requirement checks.
#

set -euo pipefail

print_usage() {
    cat <<-EOU
Usage:

  scripts/tools/publish.sh VERSION [TARGETS]

  VERSION    Operator version that is published.
  TARGET     Can be one of 'operatorhub', 'redhat-community', or
             'redhat-marketplace'. If not specified all targets are attempted.
EOU
    exit 0
}

setup_gnu_sed() {
    local sed_cmd=""
    
    # Check if gsed is available (GNU sed installed via Homebrew on macOS)
    if command -v gsed &> /dev/null; then
        sed_cmd="gsed"
    elif command -v sed &> /dev/null; then
        # Test if sed is GNU sed by checking for GNU-specific option
        if sed --version 2>/dev/null | grep -q "GNU sed"; then
            sed_cmd="sed"
        else
            echo "[error] BSD sed detected, please install GNU sed (gsed) or use Homebrew to install it."
            exit 1
        fi
    else
        echo "[error] No sed command found"
        exit 1
    fi
    
    echo "$sed_cmd"
}

# Initialize sed command
SED_CMD="$(setup_gnu_sed)"

print_targets() {
    printf 'Attempting to publish "%s" to the following targets:\n\n' ${VERSION}
    printf '\t> %s\n' ${TARGET}
    printf '\n'
}

VERSION="${1:-}"

[ -z "${VERSION}" ] && print_usage

shift
TARGET="${@:-operatorhub redhat-community redhat-marketplace}"

print_targets

OPERATOR_HOME_DIR="${PWD}"
BUILD_DIR="${OPERATOR_HOME_DIR}/.build"

SSH_KEY_FILE="${SSH_KEY_FILE:-${BUILD_DIR}/gl-distribution-oc}"
GITHUB_ACCOUNT="${GITHUB_ACCOUNT:-gl-distribution-oc}"
GITHUB_TOKEN_FILE="${GITHUB_TOKEN_FILE:-${BUILD_DIR}/gh-token}"

OH_REPOSITORY='community-operators'
OH_OWNER='k8s-operatorhub'
OH_BUILD_DIR="${BUILD_DIR}/${OH_REPOSITORY}"
OH_BUNDLE_DIR="${BUILD_DIR}/operatorhub-io"

RH_REPOSITORY='certified-operators'
RH_OWNER='redhat-openshift-ecosystem'
RH_BUILD_DIR="${BUILD_DIR}/${RH_REPOSITORY}"
RH_BUNDLE_DIR="${BUILD_DIR}/redhat-cert"

RHC_REPOSITORY='community-operators-prod'
RHC_OWNER='redhat-openshift-ecosystem'
RHC_BUILD_DIR="${BUILD_DIR}/${RHC_REPOSITORY}"
RHC_BUNDLE_DIR="${BUILD_DIR}/redhat-community"

GITLAB_OPERATOR_DIR='gitlab-operator-kubernetes'
BRANCH_NAME="gitlab-operator-kubernetes-${VERSION}"

CANONICAL_REMOTE='origin'
BUILD_REMOTE='dev'
SECURITY_REMOTE='security'
RELEASES_PAGE='https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases'
DOCKER_IMAGE='registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator'

run_check() {
    if "${@}" &> /dev/null; then
        echo passed
    else
        echo failed
        exit 2
    fi
}

check_requirements() {
    local stable_branch="${VERSION%*.*}"
    stable_branch="${stable_branch/./-}-stable"

    printf '[requirement check] Upgrade path is defined: '
    run_check test -n "${PREVIOUS_OPERATOR_VERSION:-}${OLM_SKIP_VERSION:-}"

    if [ -n "${OLM_SKIP_VERSION:-}" ]; then
        printf '[requirement check] Upgrade mode: skips %s' "${OLM_SKIP_VERSION}"
    else
        printf '[requirement check] Upgrade mode: replaces %s' "${PREVIOUS_OPERATOR_VERSION}"
    fi

    printf '[requirement check] script is running on the %s branch: ' "${stable_branch}"
    run_check test "$(git rev-parse --abbrev-ref HEAD)" = "${stable_branch}"

    for the_remote in ${CANONICAL_REMOTE} ${SECURITY_REMOTE} ${BUILD_REMOTE}; do
        printf '[requirement check] remote %s has %s tag: ' "${the_remote}" "${VERSION}"
        run_check git ls-remote -q --tags --exit-code "${the_remote}" "${VERSION}"

        printf '[requirement check] remote %s has %s branch: ' "${the_remote}" "${stable_branch}"
        run_check git ls-remote -q --heads --exit-code "${the_remote}" "${stable_branch}"
    done

    printf '[requirement check] release page for version %s is available: ' "${VERSION}"
    run_check curl -Lfsqo /dev/null "${RELEASES_PAGE}/{$VERSION}"

    printf '[requirement check] docker image tag %s is available: ' "${VERSION}"
    run_check docker manifest inspect "${DOCKER_IMAGE}:${VERSION}"
}

fork_operators() {
    local repo_name="${2}"
    local repo_owner="${1}"

    local current_owner=$(gh repo list \
        --fork --json 'name,parent' \
        --jq '.[] | select(.name == "'${repo_name}'") | .parent.owner.login')

    gh config set git_protocol ssh

    [ "${current_owner}" != "${repo_owner}" ] && gh repo fork "${repo_owner}/${repo_name}"
    cd "${BUILD_DIR}"
    [ -d "${repo_name}" ] || gh repo clone "${repo_name}"
    cd "${repo_name}"

    git config user.name "${GIT_USERNAME}"
    git config user.email "${GIT_EMAIL}"
}

pull_operators() {
    local build_dir="${1}"

    cd "${build_dir}"
    git fetch --all
    git checkout main
    git rebase upstream/main
    git push origin main --force
}

checkout_publish_branch() {
    local build_dir="${1}"

    cd "${build_dir}"

    if git rev-parse --verify --quiet "${BRANCH_NAME}"; then
        git checkout "${BRANCH_NAME}"
        git rebase main
    else
        git checkout -b "${BRANCH_NAME}"
    fi
}

lookup_chart_version() {
    git show ${VERSION}:CHART_VERSIONS | head -n 1
}

edit_manifest() {
    cd "${OPERATOR_HOME_DIR}"

    local latest_chart_version="$(lookup_chart_version)"
    $SED_CMD -i 's/"version": ".*"/"version": "'${latest_chart_version}'"/' \
        config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml
    $SED_CMD -i 's/gitlab-operator:.*/gitlab-operator:'${VERSION}'/' \
        config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml
}

create_bundle() {
    local bundle_dir="${1}"

    cd "${OPERATOR_HOME_DIR}"
    local latest_chart_version="$(lookup_chart_version)"
    rm -rf "${bundle_dir}/bundle"
    OSDK_BASE_DIR="${bundle_dir}" scripts/olm_bundle.sh build_manifests generate_bundle patch_bundle
    OSDK_BASE_DIR="${bundle_dir}" scripts/olm_bundle.sh validate_bundle
    $SED_CMD -i 's/"version": ".*"/"version": "'${latest_chart_version}'"/' \
        "${bundle_dir}/bundle/manifests/gitlab-operator-kubernetes.clusterserviceversion.yaml"
}

annotate_bundle() {
    local bundle_dir="${1}"

    BUNDLE_DIR="${bundle_dir}/bundle" redhat/operator-certification/scripts/configure_bundle.sh adjust_annotations adjust_csv set_upgrade_path
}

copy_bundle() {
    local build_dir="${1}"
    local bundle_dir="${2}"

    cd "${build_dir}"
    local target_dir="operators/${GITLAB_OPERATOR_DIR}/${VERSION}"
    mkdir -p "${target_dir}"
    cp -R "${bundle_dir}/bundle"/* "${target_dir}"
}

commit_publish_branch() {
    local build_dir="${1}"

    cd "${build_dir}"
    local target_dir="operators/${GITLAB_OPERATOR_DIR}/${VERSION}"
    git add "${target_dir}"
    git commit -m "Add GitLab Operator ${VERSION}" --signoff

    # Make sure that only the commit only contains the files that are specific
    # to the version.
    if git diff --name-only "${BRANCH_NAME}" upstream/main | grep -vFq "${target_dir}"; then
        echo "[error] The commit contains excess. Check ${BRANCH_NAME} branch of ${build_dir}."
        exit 3
    fi

    git push origin "${BRANCH_NAME}" --force
}

cleanup_changes() {
    cd "${OPERATOR_HOME_DIR}"
    git restore ./config
}

publish_operatorhub() {
    fork_operators "${OH_OWNER}" "${OH_REPOSITORY}"
    pull_operators "${OH_BUILD_DIR}"
    checkout_publish_branch "${OH_BUILD_DIR}"
    edit_manifest
    create_bundle "${OH_BUNDLE_DIR}"

    copy_bundle "${OH_BUILD_DIR}" "${OH_BUNDLE_DIR}"
    commit_publish_branch "${OH_BUILD_DIR}"

    cd "${OH_BUILD_DIR}"
    echo "Creating community operator PR in ${OH_OWNER}/${OH_REPOSITORY}"
    gh pr create \
        -R "${OH_OWNER}/${OH_REPOSITORY}" \
        --base main \
        --head "${GITHUB_ACCOUNT}:${BRANCH_NAME}" \
        --body-file docs/pull_request_template.md \
        --title "operator gitlab-operator-kubernetes (${VERSION})"
}

publish_redhat_community() {
    fork_operators "${RHC_OWNER}" "${RHC_REPOSITORY}"
    pull_operators "${RHC_BUILD_DIR}"
    checkout_publish_branch "${RHC_BUILD_DIR}"
    edit_manifest
    create_bundle "${RHC_BUNDLE_DIR}"
    annotate_bundle "${RHC_BUNDLE_DIR}"
    copy_bundle "${RHC_BUILD_DIR}" "${RHC_BUNDLE_DIR}"
    commit_publish_branch "${RHC_BUILD_DIR}"

    cd "${RHC_BUILD_DIR}"
    echo "Creating community operator PR in ${RHC_OWNER}/${RHC_REPOSITORY}"
    gh pr create \
        -R "${RHC_OWNER}/${RHC_REPOSITORY}" \
        --base main \
        --head "${GITHUB_ACCOUNT}:${BRANCH_NAME}" \
        --body-file docs/pull_request_template.md \
        --title "operator gitlab-operator-kubernetes (${VERSION})"
}

publish_redhat_marketplace() {
    fork_operators "${RH_OWNER}" "${RH_REPOSITORY}"
    pull_operators "${RH_BUILD_DIR}"
    checkout_publish_branch "${RH_BUILD_DIR}"
    edit_manifest
    create_bundle "${RH_BUNDLE_DIR}"
    annotate_bundle "${RH_BUNDLE_DIR}"
    copy_bundle "${RH_BUILD_DIR}" "${RH_BUNDLE_DIR}"
    commit_publish_branch "${RH_BUILD_DIR}"

    echo "Creating certified operator PR in ${RH_OWNER}/${RH_REPOSITORY}"
    cd "${RH_BUILD_DIR}"
    gh pr create \
        -R "${RH_OWNER}/${RH_REPOSITORY}" \
        --base main \
        --head "${GITHUB_ACCOUNT}:${BRANCH_NAME}" \
        --body '' \
        --title "operator gitlab-operator-kubernetes (${VERSION})"
}

[ -z "${SKIP_CHECKS:-}" ] && check_requirements

export YQ="$(which yq)"
export TKN="$(which tkn)"

export GH_TOKEN="$(cat ${GITHUB_TOKEN_FILE})"
export OPERATOR_TAG="${VERSION}"
export OLM_PACKAGE_VERSION=${OPERATOR_TAG}
export OPERATORHUB_DIR="${BUILD_DIR}/${OH_REPOSITORY}"
export OPERATORHUB_NAME="${GITLAB_OPERATOR_DIR}"

[ -n "${SSH_KEY_FILE}" ] && export GIT_SSH_COMMAND="ssh -i ${SSH_KEY_FILE} -o IdentitiesOnly=yes"
[ -z "${KUBECONFIG:-}" ] && export KUBECONFIG="${BUILD_DIR}/kubeconfig"
[ -z "${GIT_USERNAME:-}" ] && export GIT_USERNAME="$(git config --global user.name) (Operator Release)"
[ -z "${GIT_EMAIL:-}" ] && export GIT_EMAIL="$(git config --global user.email | $SED_CMD 's/@/+operator-release@/')"

export GIT_FORK_REPO_URL="git@github.com:${GITHUB_ACCOUNT}/${RH_REPOSITORY}.git"
export GIT_BRANCH="${BRANCH_NAME}"
export OPERATOR_BUNDLE_PATH="operators/${GITLAB_OPERATOR_DIR}/${VERSION}"
export GITHUB_TOKEN_FILE
export SSH_KEY_FILE

for target in ${TARGET}; do
    "publish_${target/-/_}" || true
done

cleanup_changes
