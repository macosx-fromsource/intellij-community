#!/usr/bin/env bash
# Deploy orchestration for the local kind dev flow.
#
# Invoked by `nix run .#deploy`, which exports the Nix-built artifact paths
# below before exec'ing this script. It can also be run standalone if those
# vars are provided. The impure cluster steps themselves live in
# provision_and_deploy.sh; this script only sequences them and applies the
# GitLab CR (static overlay from Nix + the two host-specific scalars).
#
# Inputs (exported by the flake env prelude; have sensible standalone fallbacks):
#   OPERATOR_MANIFEST          rendered operator manifest (registry image)
#   OPERATOR_MANIFEST_DEV      rendered operator manifest (local dev image)
#   CR_OVERLAY                 static GitLab CR overlay (YAML)
#   DEV_IMAGE_NAME             fully-qualified local dev image name
#   DEV_IMAGE_TAG              local dev image tag
#   KIND_CLUSTER_NAME          kind cluster name (single-sourced from cfg)
#   TARGET_NAMESPACE           target namespace (single-sourced from cfg)
#   KUBE_CONTEXT               kubectl context (defaults to kind-$KIND_CLUSTER_NAME)
set -ueo pipefail

# Manifest selection:
#   - GITLAB_OPERATOR_MANIFEST set -> use it verbatim (escape hatch).
#   - DEV_IMAGE=1                  -> dev manifest (local image, pullPolicy=Never).
#   - otherwise                    -> default manifest (registry image, tag=latest).
if [ -z "${GITLAB_OPERATOR_MANIFEST:-}" ]; then
  if [ "${DEV_IMAGE:-0}" = "1" ]; then
    GITLAB_OPERATOR_MANIFEST="${OPERATOR_MANIFEST_DEV}"
    echo "DEV_IMAGE=1 — using locally-built operator image (${DEV_IMAGE_NAME}:${DEV_IMAGE_TAG})."
    echo "Make sure you ran 'nix run .#load-image' against this cluster."
  else
    GITLAB_OPERATOR_MANIFEST="${OPERATOR_MANIFEST}"
  fi
fi
export GITLAB_OPERATOR_MANIFEST

# provision_and_deploy.sh requires KIND_LOCAL_IP (used to build a <ip>.nip.io
# domain for ingress). Auto-detect the host's LAN IP if unset. The host network
# tools live outside the nix runtime PATH, so add the usual system dirs just for
# this lookup.
if [ -z "${KIND_LOCAL_IP:-}" ]; then
  syspath="/usr/sbin:/sbin:/usr/bin:/bin"
  case "$(uname -s)" in
    Linux)
      KIND_LOCAL_IP="$(PATH="$PATH:$syspath" ip route get 1.1.1.1 2>/dev/null \
        | awk '{for (i=1;i<=NF;i++) if ($i=="src") {print $(i+1); exit}}')"
      ;;
    Darwin)
      iface="$(PATH="$PATH:$syspath" route -n get default 2>/dev/null \
        | awk '/interface:/{print $2}')"
      if [ -n "$iface" ]; then
        KIND_LOCAL_IP="$(PATH="$PATH:$syspath" ipconfig getifaddr "$iface" 2>/dev/null || true)"
      fi
      ;;
  esac
fi
if [ -z "${KIND_LOCAL_IP:-}" ]; then
  echo "Could not auto-detect KIND_LOCAL_IP; set it manually and re-run." >&2
  exit 1
fi
export KIND_LOCAL_IP
echo "Using KIND_LOCAL_IP=$KIND_LOCAL_IP"

GITLAB_OPERATOR_DOMAIN="${GITLAB_OPERATOR_DOMAIN:-${KIND_LOCAL_IP}.nip.io}"
export GITLAB_OPERATOR_DOMAIN

GITLAB_CHART_VERSION="${GITLAB_CHART_VERSION:-$(sort -rV CHART_VERSIONS | head -n1)}"
export GITLAB_CHART_VERSION

# Context safety: pin kubectl to the local kind cluster so deploy can never
# touch a remote cluster, whatever the current-context is. These normally
# arrive pre-set from the flake env prelude (single-sourced from cfg); the
# fallbacks here match provision_and_deploy.sh so standalone runs still work.
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-gitlab}"
TARGET_NAMESPACE="${TARGET_NAMESPACE:-gitlab-system}"
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-${KIND_CLUSTER_NAME}}"
export KIND_CLUSTER_NAME TARGET_NAMESPACE
export KUBECTL="kubectl --context ${KUBE_CONTEXT}"
kubectl_ctx=(kubectl --context "${KUBE_CONTEXT}")

secret_missing() {
  ! "${kubectl_ctx[@]}" -n "${TARGET_NAMESPACE}" get secret "$1" >/dev/null 2>&1
}

steps=()
cluster_exists=false
if kind get clusters 2>/dev/null | grep -qx "${KIND_CLUSTER_NAME}"; then
  cluster_exists=true
  echo "kind cluster ${KIND_CLUSTER_NAME} already exists; skipping creation"
else
  steps+=(create_kind_cluster)
fi
steps+=(create_namespace install_certmanager wait_for_certmanager)
if [ "$cluster_exists" != true ] || secret_missing custom-gitlab-tls; then
  steps+=(create_gitlab_cert deploy_gitlab_cert)
fi
if [ "$cluster_exists" != true ] || secret_missing custom-pages-tls; then
  steps+=(create_pages_cert deploy_pages_cert)
fi
steps+=(deploy_operator wait_for_operator)

bash ./scripts/provision_and_deploy.sh "${steps[@]}"

if [ ! -f external-deps.yaml ]; then
  echo "external-deps.yaml not found — run 'nix run .#deps-dev' first." >&2
  exit 1
fi

# Merge the runtime-generated external-deps wiring (doc 0) with the Nix-built
# static CR overlay (doc 1), then inject the two host-specific scalars.
yq eval-all '
  select(fileIndex == 0) * select(fileIndex == 1)
  | .spec.chart.version = strenv(GITLAB_CHART_VERSION)
  | .spec.chart.values.global.hosts.domain = strenv(GITLAB_OPERATOR_DOMAIN)
' external-deps.yaml "${CR_OVERLAY}" \
  | "${kubectl_ctx[@]}" -n "${TARGET_NAMESPACE}" apply -f -
