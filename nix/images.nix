# Local image cache + prebaked kind node image, so a fresh `up-dev` does ZERO
# network image pulls. Three additive, runtime-agnostic apps — run once after
# your first (slow) up-dev, then reused on every later standup:
#
#   nix run .#capture-images   record every image the running cluster pulled
#                              (containerd contents on the kind node)
#   nix run .#prewarm-images   skopeo-copy those (+ the chart-rendered component
#                              pre-seed) into a local archive cache — pull once
#   nix run .#bake-node-image  import the cached archives into a fresh copy of
#                              the base kind node image and commit it, so a
#                              fresh cluster starts with them already loaded
#
# `packages.image-list` is a best-effort PRE-SEED of the GitLab component images
# parsed from the charts flake's offline render. The CAPTURED list (real
# containerd contents) is authoritative and also covers cert-manager, Envoy
# Gateway and the external deps (CNPG/Garage/Valkey) — whose image refs live in
# the charts CI-lib scripts and so cannot be enumerated statically here.
#
# The container runtime (docker | podman | nerdctl) is taken off the host PATH,
# not pinned into the closure — kind itself needs one anyway, and this keeps the
# apps daemon-agnostic (mirrors ../gitlab-charts nix/cluster.nix).
{
  pkgs,
  lib,
  cfg,
  runtimeProbe,
  cacheEnv,
  envDefaults,
  clusterTools,
  mkScript,
  mkApp,
  chartsRendered,
}:
let
  imageTools = clusterTools ++ [
    pkgs.skopeo
    pkgs.jq
    pkgs.gnused
    pkgs.gnugrep
  ];

  # Best-effort GitLab component image list, parsed from the charts flake's
  # offline `helm template` render (single-sourced from ../gitlab-charts). Only
  # catches single `image:` lines (not repository:/tag: splits); the captured
  # list covers the rest, so this only affects the very first prewarm.
  imageList =
    pkgs.runCommand "gitlab-image-list"
      { nativeBuildInputs = [ pkgs.gnugrep pkgs.gnused pkgs.coreutils ]; }
      ''
        grep -hoE '^[[:space:]]*-?[[:space:]]*image:[[:space:]]*.+' ${chartsRendered} \
          | sed -E 's/^[[:space:]]*-?[[:space:]]*image:[[:space:]]*//' \
          | tr -d '\042\047' \
          | sed '/^[[:space:]]*$/d' \
          | sort -u > "$out"
      '';

  # Shared shell prelude for the image apps: cache env (dir + canonical chart
  # version, from nix/lib.nix) + the per-version list file + the shared
  # container-runtime probe + a ref->filename slug helper. (Apps run under
  # writeShellApplication's set -euo pipefail.)
  cachePrelude = ''
    ${cacheEnv}
    mkdir -p "$CACHE_DIR/images"
    export LIST_FILE="$CACHE_DIR/image-list-$CHART_VER.txt"
    ${runtimeProbe}
    slug() { printf '%s' "$1" | tr '/:@' '___'; }
  '';

  # Record every image containerd actually pulled onto the kind node, so prewarm
  # + bake cover the real set (deps included) rather than a static guess.
  captureImages = mkScript "capture-images" imageTools ''
    ${envDefaults}
    ${cachePrelude}
    node="''${KIND_CLUSTER_NAME}-control-plane"
    rt="$(detect_runtime)"
    if ! "$rt" inspect "$node" >/dev/null 2>&1; then
      echo "error: node container '$node' not found — run 'nix run .#up-dev' first" >&2
      exit 1
    fi
    echo "==> capturing images from containerd on $node (runtime: $rt)"
    # Tag/name refs skopeo can copy, minus sha256-only digests and the images
    # already baked into the base kind node (control plane + pause): those come
    # for free when we bake FROM kindest/node, so pulling them is wasted work.
    "$rt" exec "$node" ctr -n k8s.io images ls -q \
      | grep -vE '^sha256:|kindest/|registry\.k8s\.io/(pause|kube-|etcd|coredns)|/pause(:|@)' \
      | sort -u > "$LIST_FILE"
    echo "captured $(wc -l < "$LIST_FILE") images -> $LIST_FILE"
  '';

  # Pull each image once into a local docker-archive cache. Union of the
  # captured list (authoritative, if a prior up-dev + capture ran) and the
  # chart-rendered component pre-seed. Skips archives already present.
  prewarmImages = mkScript "prewarm-images" imageTools ''
    ${cachePrelude}
    # Prefer the captured list (authoritative — the real containerd contents).
    # Fall back to the chart-rendered component pre-seed only before a capture
    # exists, so we don't pull images the operator deploy never actually uses.
    if [ -f "$LIST_FILE" ]; then
      src="$LIST_FILE"
      echo "==> using captured image list ($(wc -l < "$LIST_FILE" | tr -d ' ') refs): $LIST_FILE"
    else
      src="${imageList}"
      echo "==> no capture yet — using chart-rendered pre-seed (run capture-images after up-dev for the full set)"
    fi

    # Pull one ref into the archive cache. docker-archive's destination ref must
    # be a TAG, not a digest, so for digest-pinned refs (repo@sha256:...)
    # synthesize a tag; the manifest digest is preserved, and bake re-tags the
    # digest name in containerd so pods referencing by digest still resolve.
    pull_one() {
      local ref="$1" tar dest
      tar="$CACHE_DIR/images/$(slug "$ref").tar"
      if [ -f "$tar" ]; then echo "cached : $ref"; return 0; fi
      dest="$ref"
      case "$ref" in
        *@sha256:*) dest="''${ref%@*}:preloaded" ;;
      esac
      echo "pulling: $ref"
      # --insecure-policy: skopeo on Nix ships no default policy.json in the
      #   system paths, so accept-anything avoids a trust-policy config file.
      # --override-os linux: on macOS skopeo runs as darwin; kind nodes are Linux.
      if skopeo --insecure-policy copy --override-os linux --retry-times 3 \
          "docker://$ref" "docker-archive:$tar:$dest" >/dev/null 2>&1; then
        printf '%s\n' "$ref" > "$tar.ref"
        echo "pulled : $ref"
      else
        echo "  warn: failed to pull $ref (skipping — will pull at runtime)"
        rm -f "$tar" "$tar.ref"
      fi
    }

    # Bounded parallelism — image pulls are network-bound. Tune with PREWARM_JOBS.
    MAXJOBS="''${PREWARM_JOBS:-6}"
    running=0
    while read -r ref; do
      [ -n "$ref" ] || continue
      pull_one "$ref" &
      running=$((running + 1))
      if [ "$running" -ge "$MAXJOBS" ]; then
        wait -n 2>/dev/null || true
        running=$((running - 1))
      fi
    done < <(sort -u "$src")
    wait || true
    echo "==> prewarm done. cache: $CACHE_DIR/images"
  '';

  # Bake the cached archives into a reusable kind node image. Boots a fresh
  # (non-kubeadm'd) node so its containerd is up, imports every archive into the
  # k8s.io namespace, then commits. NOTE: committing a booted node is the
  # fragile step — the run flags (--privileged, cgroups, init) can differ across
  # docker/podman; validate and tweak here first.
  bakeNodeImage = mkScript "bake-node-image" imageTools ''
    ${cachePrelude}
    rt="$(detect_runtime)"
    baked="${cfg.nodeImageRepo}:$CHART_VER"
    shopt -s nullglob
    archives=("$CACHE_DIR"/images/*.tar)
    shopt -u nullglob
    if [ "''${#archives[@]}" -eq 0 ]; then
      echo "no cached archives in $CACHE_DIR/images — run 'nix run .#prewarm-images' first" >&2
      exit 1
    fi
    echo "==> baking $baked from ${cfg.kindNodeImage} (+ ''${#archives[@]} images, runtime: $rt)"
    c="operator-node-bake-$$"
    # shellcheck disable=SC2064
    trap "$rt rm -f '$c' >/dev/null 2>&1 || true" EXIT
    "$rt" run -d --privileged --name "$c" "${cfg.kindNodeImage}" >/dev/null
    echo "    waiting for containerd inside the node..."
    ready=""
    for _ in $(seq 1 60); do
      if "$rt" exec "$c" ctr -n k8s.io images ls -q >/dev/null 2>&1; then
        ready=1
        break
      fi
      sleep 1
    done
    # Abort if containerd never came up: proceeding would import nothing (every
    # import warns+continues) and still commit a node, which nodeImageResolver
    # would then PREFER on the next up-dev — a silent, zero-benefit cache. The
    # EXIT trap removes the bake container.
    if [ -z "$ready" ]; then
      echo "error: containerd not ready in the bake node after 60s — aborting (refusing to commit a node with no preloaded images)" >&2
      exit 1
    fi
    for a in "''${archives[@]}"; do
      echo "    import $(basename "$a")"
      "$rt" cp "$a" "$c:/tmp/img.tar"
      # --no-unpack: register the image in containerd's content/image store but
      # do NOT unpack it into a snapshot now. Unpacking converts overlay whiteout
      # markers (mknod char 0:0), which fails on this RAW booted node when the
      # daemon runs overlay-on-overlay (Docker Desktop for Mac):
      #   "failed to convert whiteout file ... operation not permitted".
      # A real kubeadm'd kind node unpacks the same layers fine at pod-start
      # (a normal up-dev runs postgresql:17 etc.), so defer the unpack there.
      # The image still counts as present, so kubelet won't network-pull it.
      # Warn+continue (don't abort the whole bake) so one bad archive costs at
      # most a single runtime pull instead of the entire prebaked node.
      if ! "$rt" exec "$c" ctr -n k8s.io images import --no-unpack /tmp/img.tar; then
        echo "      warn: import failed for $(basename "$a") (will pull at runtime)"
      fi
      "$rt" exec "$c" rm -f /tmp/img.tar
      # A digest-pinned image was archived under a synthesized :preloaded tag
      # (see prewarm). Re-create its digest name so pods referencing by digest
      # find it locally instead of pulling.
      ref="$(cat "$a.ref" 2>/dev/null || true)"
      case "$ref" in
        *@sha256:*)
          echo "      tag digest ref $ref"
          "$rt" exec "$c" ctr -n k8s.io images tag "''${ref%@*}:preloaded" "$ref" \
            || echo "      warn: could not tag digest ref (will pull at runtime)" ;;
      esac
    done
    "$rt" commit "$c" "$baked" >/dev/null
    echo "==> baked $baked"
    echo "    up-dev will now use it (fresh clusters start with images preloaded)."
  '';
in
{
  inherit imageList;
  apps = {
    capture-images = mkApp captureImages;
    prewarm-images = mkApp prewarmImages;
    bake-node-image = mkApp bakeNodeImage;
  };
}
