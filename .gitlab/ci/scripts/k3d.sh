#!/bin/bash

# k3d cluster lifecycle for per-job isolated cluster tests.
# Each CI job creates its own single-use k3d cluster inside the job's
# Docker-in-Docker environment, eliminating shared-cluster contention and
# standing infrastructure. Ported from gitlab-org/charts/gitlab
# scripts/ci/k3d.sh and adapted for the operator CI conventions.
# See: https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/98

# Configurable variables. Override any of these from the CI job environment.
K3D_VERSION="${K3D_VERSION:-5.8.3}"
DOCKER_VERSION="${DOCKER_VERSION:-28.0.1}"
# Path where the cluster's kubeconfig is written.
K3D_KUBECONFIG="${K3D_KUBECONFIG:-/tmp/k3d-kubeconfig.yaml}"

function k3d_cluster_name() {
  echo -n "gitlab"
}

function k3d_install() {
  local arch docker_arch
  arch=$(uname -m)
  case "$arch" in
    x86_64)  arch="amd64"; docker_arch="x86_64" ;;
    aarch64) arch="arm64"; docker_arch="aarch64" ;;
    *) echo "Unsupported architecture: $arch"; exit 1 ;;
  esac

  # Install the Docker CLI if not present — needed by k3d and gitlab-qa. The
  # operator build-base image is Alpine-based and ships podman only; the
  # static binary requires no package repo configuration.
  if ! command -v docker &>/dev/null; then
    echo "Installing Docker CLI v${DOCKER_VERSION} (${docker_arch})"
    curl -fLo /tmp/docker.tgz \
      "https://download.docker.com/linux/static/stable/${docker_arch}/docker-${DOCKER_VERSION}.tgz"
    tar -xz -C /usr/local/bin --strip-components=1 -f /tmp/docker.tgz docker/docker
    echo "Docker CLI $(docker --version) installed"
  else
    echo "Docker CLI already available: $(docker --version)"
  fi

  if command -v k3d &>/dev/null; then
    local installed
    installed=$(k3d version 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
    if [ "${installed}" = "${K3D_VERSION}" ]; then
      echo "k3d ${K3D_VERSION} already installed"
      return
    fi
    echo "k3d ${installed} installed but expected ${K3D_VERSION}"
  fi

  echo "Installing k3d v${K3D_VERSION} (${arch})"
  curl -fLo /tmp/k3d "https://github.com/k3d-io/k3d/releases/download/v${K3D_VERSION}/k3d-linux-${arch}"
  install -c -m 0755 /tmp/k3d /usr/local/bin/k3d
  echo "k3d $(k3d version) installed"
}

function k3d_docker_host_ip() {
  # When running with the .dind service, Docker is at tcp://docker:2375. The
  # DinD host's IP is what we need: k3d exposes ports on it, and nip.io
  # routes back to it from the CI job container.
  if echo "${DOCKER_HOST:-}" | grep -q "docker"; then
    getent hosts docker | awk '{print $1; exit}'
  else
    # Parse by the "src" keyword: field position varies with routing topology.
    ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}'
  fi
}

function k3d_create() {
  local cluster_name docker_ip
  cluster_name=$(k3d_cluster_name)
  docker_ip=$(k3d_docker_host_ip)

  echo "Creating k3d cluster '${cluster_name}' (image: ${K3D_K8S_IMAGE})"
  echo "DinD/Docker host IP for port mapping: ${docker_ip}"

  # Traefik is disabled so the GitLab chart's bundled NGINX controller can
  # bind ports 80/443 through the k3s service load balancer. Port 22 is
  # mapped for Git-over-SSH.
  k3d cluster create "${cluster_name}" \
    --image "${DOCKERHUB_PREFIX:-docker.io}/${K3D_K8S_IMAGE}" \
    --api-port "${docker_ip}:6443" \
    --port "22:22@loadbalancer" \
    --port "80:80@loadbalancer" \
    --port "443:443@loadbalancer" \
    --k3s-arg "--disable=traefik@server:0" \
    --wait \
    --timeout 120s

  k3d kubeconfig get "${cluster_name}" > "${K3D_KUBECONFIG}"
  export KUBECONFIG="${K3D_KUBECONFIG}"

  # nip.io domain: *.IP.nip.io resolves to IP — zero-config DNS for the job.
  export DOMAIN="${docker_ip}.nip.io"
  echo "Using domain: ${DOMAIN}"

  # nip.io also parses dash-separated digit groups anywhere in the name as an
  # IP in dash notation: gitlab-98736478--10-1-2.172.17.0.2.nip.io resolves
  # to 2.172.17.0, not 172.17.0.2. Chart-version pipelines append the chart
  # version to HOSTSUFFIX, producing exactly that pattern. Neutralize every
  # dash directly followed by a digit so hostnames resolve to the intended
  # IP. HOSTSUFFIX only feeds hostnames and manifest file names, so the
  # rewrite is safe.
  if echo "${HOSTSUFFIX:-}" | grep -qE -- '-[0-9]'; then
    local safe_hostsuffix
    safe_hostsuffix=$(echo "${HOSTSUFFIX}" | sed -E 's/-([0-9])/x\1/g')
    echo "Sanitizing HOSTSUFFIX for nip.io: ${HOSTSUFFIX} -> ${safe_hostsuffix}"
    export HOSTSUFFIX="${safe_hostsuffix}"
  fi

  kubectl wait --for=condition=Ready nodes --all --timeout=120s
}

function k3d_delete() {
  if ! command -v k3d &>/dev/null; then
    echo "k3d not installed, skipping cluster delete"
    return 0
  fi
  local cluster_name
  cluster_name=$(k3d_cluster_name)
  k3d cluster delete "${cluster_name}" || true
}

# Collect debug artifacts (GitLab CR status, pod events, logs) into
# ${CI_PROJECT_DIR}/k3d-debug/ before the cluster is destroyed. Called from
# after_script — all commands are best-effort and must never fail the job.
function k3d_collect_debug() {
  local debug_dir="${CI_PROJECT_DIR:-$(pwd)}/k3d-debug"
  local ns="${TESTS_NAMESPACE:-gitlab-system}"
  mkdir -p "${debug_dir}/failed-pod-logs" "${debug_dir}/operator-logs"

  # after_script runs in a fresh shell: restore the kubeconfig exported by
  # k3d_create if it exists.
  [ -f "${K3D_KUBECONFIG}" ] && export KUBECONFIG="${K3D_KUBECONFIG}"

  echo "k3d_collect_debug: namespace='${ns}' output='${debug_dir}'"

  if command -v kubectl &>/dev/null; then
    kubectl get gitlab -n "${ns}" -o yaml > "${debug_dir}/gitlab-cr.yaml" 2>&1 || true
    kubectl get pods -A -o wide > "${debug_dir}/pods.txt" 2>&1 || true
    kubectl describe pods -n "${ns}" > "${debug_dir}/pods-describe.txt" 2>&1 || true
    kubectl get events -A --sort-by=.lastTimestamp > "${debug_dir}/events.txt" 2>&1 || true
    { kubectl get nodes -o wide; echo "---"; kubectl describe nodes; } > "${debug_dir}/nodes.txt" 2>&1 || true

    # Operator controller logs.
    local deploy
    for deploy in $(kubectl get deployments -n "${ns}" -o name 2>/dev/null | grep controller-manager); do
      kubectl logs -n "${ns}" "${deploy}" -c manager --tail=1000 \
        > "${debug_dir}/operator-logs/${deploy##*/}.log" 2>&1 || true
    done

    # Logs from pods not Running in the test namespace.
    local not_running
    not_running=$(kubectl get pods -n "${ns}" \
      -o jsonpath='{range .items[?(@.status.phase!="Running")]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)
    while IFS= read -r pod; do
      [ -z "${pod}" ] && continue
      kubectl logs -n "${ns}" "${pod}" --all-containers --tail=500 \
        > "${debug_dir}/failed-pod-logs/${pod}.log" 2>&1 || true
      kubectl logs -n "${ns}" "${pod}" --all-containers --previous --tail=500 \
        > "${debug_dir}/failed-pod-logs/${pod}.previous.log" 2>&1 || true
    done <<< "${not_running}"
  fi

  if command -v helm &>/dev/null; then
    helm list -A > "${debug_dir}/helm-releases.txt" 2>&1 || true
  fi

  echo "k3d_collect_debug: collected artifacts:"
  ls -la "${debug_dir}" 2>/dev/null || true
}

function k3d_info() {
  echo "k3d cluster: $(k3d_cluster_name)"
  echo "  K8s image:  ${K3D_K8S_IMAGE}"
  echo "  KUBECONFIG: ${KUBECONFIG}"
  echo "  Domain:     ${DOMAIN}"
}
