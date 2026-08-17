# Shared shell snippets for the cache/cluster apps. Kept as plain strings (not
# packages) so each writeShellApplication can source them inline under its own
# `set -euo pipefail`, and defined in ONE place instead of being copy-pasted per
# app (the container-runtime probe and the cache-dir/chart-version resolution
# were previously duplicated across images.nix, snapshot.nix and flake.nix).
#
# Parameterised only by `defaultChartVersion` (the canonical version computed in
# flake.nix), which cacheEnv bakes in as the fallback for apps that don't source
# envDefaults.
{ defaultChartVersion }:
{
  # Define `detect_runtime`: print the first working container runtime
  # (docker|podman|nerdctl) on PATH, or print an error to stderr + return 1 if
  # none. Two call styles:
  #   REQUIRED  rt="$(detect_runtime)"                 # set -e aborts if none
  #   TOLERATED rt="$(detect_runtime 2>/dev/null || true)" ; [ -n "$rt" ] || ...
  # (Runtime taken off the host PATH, not pinned into the closure — kind needs
  # one anyway, and this keeps the apps daemon-agnostic.)
  runtimeProbe = ''
    detect_runtime() {
      local __rt
      for __rt in docker podman nerdctl; do
        if command -v "$__rt" >/dev/null 2>&1 && "$__rt" info >/dev/null 2>&1; then
          printf '%s' "$__rt"
          return 0
        fi
      done
      echo "error: no working container runtime (docker/podman/nerdctl) on PATH" >&2
      return 1
    }
  '';

  # Export CACHE_DIR + CHART_VER. CHART_VER resolves to the same canonical
  # version envDefaults exports as GITLAB_CHART_VERSION, with the Nix default as
  # a fallback for apps that don't source envDefaults (prewarm/bake) — so every
  # cache key matches the deployed chart version.
  cacheEnv = ''
    export CACHE_DIR="''${GITLAB_OPERATOR_CACHE:-''${XDG_CACHE_HOME:-$HOME/.cache}/gitlab-operator}"
    CHART_VER="''${GITLAB_CHART_VERSION:-${defaultChartVersion}}"
    export CHART_VER
  '';
}
