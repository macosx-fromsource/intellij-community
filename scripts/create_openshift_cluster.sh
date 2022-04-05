#!/bin/bash -e

# Environment variables
CLUSTER_NAME="${CLUSTER_NAME:-ocp-$USER}"
CLUSTER_VERSION="${CLUSTER_VERSION:-4.8.21}"
BASE_DOMAIN="${BASE_DOMAIN:-k8s-ft.win}"
FIPS_ENABLED="${FIPS_ENABLED:-false}"

GCP_PROJECT_ID="${GCP_PROJECT_ID:-cloud-native-182609}"
GCP_REGION="${GCP_REGION:-us-central1}"
GOOGLE_APPLICATION_CREDENTIALS="${GOOGLE_APPLICATION_CREDENTIALS:-gcloud.json}"
GOOGLE_CREDENTIALS="${GOOGLE_CREDENTIALS:-$(cat $GOOGLE_APPLICATION_CREDENTIALS)}"

PULL_SECRET_FILE="${PULL_SECRET_FILE:-pull_secret}"
PULL_SECRET="${PULL_SECRET:-$(cat $PULL_SECRET_FILE)}"

SSH_PUBLIC_KEY_FILE="${SSH_PUBLIC_KEY_FILE:-$HOME/.ssh/id_rsa.pub}"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-$(cat $SSH_PUBLIC_KEY_FILE)}"

LOG_LEVEL="${LOG_LEVEL:-info}"

INSTALL_DIR="install-${CLUSTER_NAME}"
TOOL_DIR="bin"
TMP_DIR="tmp-cluster"

main() {
  export GOOGLE_CREDENTIALS  # needed for openshift-install to see them
  local scripts_dir="$(dirname "$0")"

  if [ -d "$INSTALL_DIR" ]; then
    echo "$INSTALL_DIR exists, skipping cluster creation"
  else
    verify_requirements
    render_config_file
    create_cluster
  fi

  export KUBECONFIG="install-$CLUSTER_NAME/auth/kubeconfig"
  . "${scripts_dir}/install_certmanager.sh" 'openshift'

  echo "If this is a cluster meant to run CI pipelines, run"
  echo "./ci/scripts/install_external_dns.sh to finish network configuration"
}

verify_requirements() {
  echo "Verifying requirements for OpenShift $CLUSTER_VERSION"

  _verify_installed "openshift-install"
  _verify_installed "oc"
}

render_config_file() {
  echo 'Rendering install-config file'
  template_data="$(cat scripts/install-config.template.yaml)"

  template_data="$(echo "${template_data//CLUSTER_NAME/$CLUSTER_NAME}")"
  template_data="$(echo "${template_data//PULL_SECRET/$PULL_SECRET}")"
  template_data="$(echo "${template_data//SSH_PUBLIC_KEY/$SSH_PUBLIC_KEY}")"
  template_data="$(echo "${template_data//BASE_DOMAIN/$BASE_DOMAIN}")"
  template_data="$(echo "${template_data//GCP_PROJECT_ID/$GCP_PROJECT_ID}")"
  template_data="$(echo "${template_data//GCP_REGION/$GCP_REGION}")"
  template_data="$(echo "${template_data//FIPS_ENABLED/$FIPS_ENABLED}")"

  mkdir -p $INSTALL_DIR
  echo "$template_data" > "$INSTALL_DIR/install-config.yaml"
}

create_cluster() {
  echo "Creating cluster '$CLUSTER_NAME'"
  $TOOL_DIR/openshift-install-$CLUSTER_VERSION create cluster \
    --dir "$INSTALL_DIR" --log-level "$LOG_LEVEL"
}

_verify_installed() {
  local tool="$1"
  if ! type "$TOOL_DIR/$tool-$CLUSTER_VERSION" > /dev/null; then
    _download_openshift_tool "$tool"
  fi
}

_download_openshift_tool() {
  local tool="$1"
  local mirrortool="$tool"
  local platform="$(uname -s | tr '[:upper:]' '[:lower:]')"
  local mirrorplatform="$platform"

  if [ "$tool" == "oc" ]; then
    mirrortool="openshift-client"
  fi

  if [ "$platform" == "darwin" ]; then
    mirrorplatform="mac"
  fi

  mkdir -p $TMP_DIR
  wget -O "$TMP_DIR/$tool.tar.gz" \
    "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/$CLUSTER_VERSION/$mirrortool-$mirrorplatform.tar.gz"

  tar -xzf "$TMP_DIR/$tool.tar.gz" -C "$TMP_DIR"

  mkdir -p $TOOL_DIR
  mv "$TMP_DIR/$tool" "$TOOL_DIR/$tool-$CLUSTER_VERSION"
}

main
