{
  # Nix dev shell + build/test/deploy for the GitLab Operator.
  #
  # The flake is split into a PURE half (cacheable, cluster-free, validated by
  # `nix flake check`) and an IMPURE half (effects against a live cluster, run
  # as thin shell apps):
  #
  #   Pure derivations:
  #     packages.operator-manifest  operator manifest rendered offline with
  #                                 `helm template` in the sandbox (no cluster).
  #     packages.cr-overlay         the static half of the GitLab CR as a Nix
  #                                 attrset serialised to YAML; the two
  #                                 host-specific scalars (chart version +
  #                                 ingress domain) are injected at deploy time.
  #     packages.manager            operator binary built with buildGoModule.
  #     packages.image              operator container image (see note below).
  #
  #   Impure apps (kind, cert-manager, openssl, kubectl apply): thin
  #   `writeShellApplication` wrappers that delegate to scripts/. The shell
  #   scripts are the source of truth for the apply/provision half.
  #
  # Dev loop: `nix run .#load-image-dev` streams the locally-built image into kind,
  # and `nix run .#deploy-dev` deploys it (tag=dev, pullPolicy=Never) so
  # operator code changes can be tested without pushing to a registry.
  description = "GitLab Operator — reproducible dev shell + build/test/deploy via Nix";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    # Dev shell tools resolved from the existing mise.toml (single-sourced).
    # See https://gitlab.com/sbordei/tool2nix
    tool2nix.url = "gitlab:sbordei/tool2nix";
    tool2nix.inputs.nixpkgs.follows = "nixpkgs";

    # GitLab Helm chart flake. The operator consumes its portable `lib`
    # (render/values/version helpers) instead of vendoring that plumbing, and
    # reads its offline `rendered` render to PRE-SEED the local image cache used
    # by the prebaked kind node image (nix/images.nix). Bump with
    # `nix flake update gitlab-charts`. SSH remote (uses your local SSH agent).
    gitlab-charts.url = "git+ssh://git@gitlab.com/gitlab-org/charts/gitlab.git?ref=master";
    gitlab-charts.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      tool2nix,
      gitlab-charts,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        lib = pkgs.lib;
        yaml = (pkgs.formats.yaml { }).generate;

        # --- tools, single-sourced from mise.toml -----------------------------
        # Resolve the same mise-pinned tools the dev shell uses, so every
        # build/test/deploy operation matches the dev shell. tool2nix maps mise
        # names to nixpkgs attrs, so versions track nixpkgs (see nix/README.md).
        # NOTE: we keep pkgs.yq-go (not mise.yq): tool2nix's heuristic resolves
        # "yq" to the Python yq, but our scripts need the Go yq.
        mise = tool2nix.lib.packagesAttrsFrom pkgs ./mise.toml;
        helm = mise.helm;

        # --- tool groups ------------------------------------------------------
        goTools = [
          mise.golang
          mise.golangci-lint
          pkgs.kubernetes-controller-tools
          pkgs.setup-envtest
        ];
        helmTools = [
          helm
          pkgs.coreutils
        ];
        clusterTools = [
          pkgs.bash
          mise.kind
          pkgs.kubectl
          helm
          pkgs.openssl
          pkgs.curl
          pkgs.jq
          pkgs.coreutils
          pkgs.gnused
          pkgs.gnugrep
        ];

        mkScript =
          name: runtimeInputs: text:
          pkgs.writeShellApplication { inherit name runtimeInputs text; };
        mkApp = drv: {
          type = "app";
          program = lib.getExe drv;
          meta.description = "GitLab Operator flake app: ${drv.name}";
        };

        # --- static, single-sourced configuration -----------------------------
        # Defaults that used to live as `${VAR:-default}` fallbacks scattered
        # across the bash scripts. Centralised here so the pure manifest render
        # and the runtime apps read the same values.
        cfg = {
          namespace = "gitlab-system";
          nameOverride = "gitlab";
          clusterMode = true;
          # Scratch dir for generated artifacts + default wait timeout. Mirror
          # the provision-script fallbacks so the standalone script and the flake
          # agree on the same defaults.
          buildDir = ".build";
          k8sTimeout = "300s";
          image = {
            registry = "registry.gitlab.com";
            repository = "gitlab-org/cloud-native";
            name = "gitlab-operator";
            tag = "latest";
          };
          # Tag used for the locally-built (Nix) operator image in dev flow.
          devImageTag = "dev";
          # Prebaked kind node image: the base node with every cluster image
          # (GitLab components + prereqs + external deps) preloaded into
          # containerd, so a fresh `up-dev` performs zero network image pulls.
          # Tagged per deployed chart version by `nix run .#bake-node-image`
          # and preferred by `up-dev` via nodeImageResolver. See nix/images.nix.
          nodeImageRepo = "kindest/node-gitlab";
          kindClusterName = "gitlab";
          # kind node k8s must be >= 1.31: chart 10.1.x's Gateway API CRDs use
          # the CEL isIP() function, which older API servers can't compile.
          # cert-manager is bumped in lockstep (the script's 1.6.1 default won't
          # run on >= 1.31). Both override the provision-script defaults and stay
          # env-overridable (KIND_IMAGE / CERT_MANAGER_VERSION).
          kindNodeImage = "kindest/node:v1.33.1";
          certManagerVersion = "1.17.2";
          # Envoy Gateway is a cluster prerequisite for the Gateway-API path
          # (the operator manages the Gateway API *resources* but not the
          # controller). Pinned to the version the bundled chart's gateway-helm
          # dependency uses, so the CRDs/controller match the operator's CRs.
          envoyGatewayVersion = "1.9.0";
          # Namespace the Envoy Gateway controller installs into (gateway-deps).
          envoyGatewayNamespace = "envoy-gateway-system";
          # Front-door wiring shared by the CR overlays. The VALUES are owned by
          # the tracked scripts/.tpl that create and consume them —
          # scripts/provision_and_deploy.sh (TLS secrets) and
          # scripts/manifests/gitlab-cr-selfsigned.yaml.tpl + the upstream
          # kind-ssl host-port map (nodePorts). Single-sourced here for the Nix
          # overlays only; keep them in step with those files.
          tlsSecretName = "custom-gitlab-tls";
          pagesTlsSecretName = "custom-pages-tls";
          httpsNodePort = 32443;
          sshPort = 32022;
        };

        # k8s minor version (e.g. "1.33") parsed from cfg.kindNodeImage, so the
        # envtest control-plane assets match the kind node the controller tests
        # run against. Pinned rather than letting `setup-envtest use` resolve
        # "latest" at runtime (which could drift from the cluster's API server).
        kindK8sVersion =
          let
            tag = lib.last (lib.splitString ":" cfg.kindNodeImage); # v1.33.1
            parts = lib.splitString "." (lib.removePrefix "v" tag); # [ "1" "33" "1" ]
          in
          "${builtins.elemAt parts 0}.${builtins.elemAt parts 1}"; # 1.33

        # THE canonical chart version: the highest entry in CHART_VERSIONS by
        # true version ordering (builtins.compareVersions, NOT lexical `head`).
        # Exported as GITLAB_CHART_VERSION (via envMap) so scripts/deploy.sh's
        # own `sort -rV | head` fallback is overridden with this exact value,
        # AND reused as the key for every cache artifact (prebaked node image,
        # state snapshot, image cache) so cached state always matches the chart
        # version actually deployed. They agree today only because CHART_VERSIONS
        # happens to be sorted descending; this makes that a guarantee, not luck.
        # Override with GITLAB_CHART_VERSION.
        defaultChartVersion =
          let
            versions = lib.filter (s: s != "") (
              lib.splitString "\n" (builtins.readFile ./CHART_VERSIONS)
            );
          in
          lib.last (lib.sort (a: b: builtins.compareVersions a b < 0) versions);

        # Shared shell snippets (container-runtime probe + cache env) sourced by
        # the cache/cluster apps, so the runtime probe and cache-dir/version
        # resolution live in ONE place instead of being copy-pasted per app.
        shellLib = import ./nix/lib.nix { inherit defaultChartVersion; };

        # --- env-var defaults, single-sourced from cfg ------------------------
        # THE one place that answers "where does $FOO's default come from?".
        # Each impure app sources `envDefaults` first, which `export`s every
        # tunable below. Because these are exported before the app hands off to
        # scripts/*.sh, a cfg value here OVERRIDES the fallback baked into
        # scripts/provision_and_deploy.sh — while a user-supplied env var still
        # wins over both (the `''${VAR:-default}` form). The env-var name is the
        # exact name the scripts read, so there is one name per concept (no
        # aliases). The comments show the script's own fallback for reference.
        #
        # Runtime-resolved values that have NO static default live in
        # scripts/deploy.sh, not here: KIND_LOCAL_IP (auto-detected),
        # GITLAB_OPERATOR_DOMAIN (derived from it), GITLAB_RUNNER_TOKEN
        # (fetched from the cluster). GITLAB_ACME_EMAIL is git-derived but
        # resolved in the prelude below (not deploy.sh) because kind-up shells
        # into provision_and_deploy.sh without going through deploy.sh.
        envMap = {
          TARGET_NAMESPACE = cfg.namespace; # script default: gitlab-system
          KIND_CLUSTER_NAME = cfg.kindClusterName; # script default: gitlab
          KIND_IMAGE = cfg.kindNodeImage; # script default: kindest/node:v1.30.8
          CERT_MANAGER_VERSION = cfg.certManagerVersion; # script default: 1.6.1
          ENVOY_GATEWAY_VERSION = cfg.envoyGatewayVersion; # (Envoy is nix-only; no script default)
          GITLAB_TLSCERTNAME = cfg.tlsSecretName; # script default: custom-gitlab-tls
          BUILD_DIR = cfg.buildDir; # script default: .build
          KUBERNETES_TIMEOUT = cfg.k8sTimeout; # script default: 300s
          GITLAB_CHART_VERSION = defaultChartVersion; # script default: sort -rV CHART_VERSIONS | head -n1
        };
        # Shell prelude every impure/cluster app sources first. Order-safe:
        # KUBE_CONTEXT is derived after KIND_CLUSTER_NAME has been resolved.
        envDefaults = ''
          ${lib.concatStringsSep "\n" (
            lib.mapAttrsToList (var: default: ''export ${var}="''${${var}:-${default}}"'') envMap
          )}
          export KUBE_CONTEXT="''${KUBE_CONTEXT:-kind-''${KIND_CLUSTER_NAME}}"
          # provision_and_deploy.sh defaults GITLAB_ACME_EMAIL to
          # `git config user.email` at top level under `set -e`; when git can't
          # resolve an identity (unset, or "dubious ownership" when the repo
          # owner UID differs from the process UID — hits nix-develop users on
          # Debian) that command fails and aborts the whole script before any
          # step runs. Resolve it here, guarded so it can never abort, and fall
          # back to a placeholder so the value is always NON-EMPTY (an empty
          # value re-triggers the fragile `''${VAR:-...}` default downstream).
          # For the selfsigned local flow the address is cosmetic; set
          # GITLAB_ACME_EMAIL explicitly for a real ACME (Let's Encrypt) issuer.
          # `git` is read off the ambient PATH (as the underlying script already
          # does) — the developer's identity is inherently non-hermetic, and the
          # guard falls back to the placeholder if git is unavailable.
          export GITLAB_ACME_EMAIL="''${GITLAB_ACME_EMAIL:-$(git config user.email 2>/dev/null || true)}"
          export GITLAB_ACME_EMAIL="''${GITLAB_ACME_EMAIL:-dev@localhost}"
        '';

        # Prefer the prebaked node image (nix run .#bake-node-image) when it
        # exists in the local container runtime, so `up-dev` starts a fresh
        # cluster with all images already in containerd (no network pulls).
        # Sourced by up-dev AFTER envDefaults so it wins over the cfg default;
        # kindUp's own envDefaults then keeps the exported value
        # (''${KIND_IMAGE:-...}). Runtime-agnostic: probes docker/podman/nerdctl
        # off the host PATH, matching how the image apps discover a runtime.
        nodeImageResolver = ''
          if [ -n "''${NO_BAKED_NODE:-}" ]; then
            echo "==> NO_BAKED_NODE set — using base node image ${cfg.kindNodeImage} (fresh image pulls)"
          else
            ${shellLib.runtimeProbe}
            # GITLAB_CHART_VERSION is the canonical version exported by
            # envDefaults, which up-dev sources before this resolver — so the
            # baked-node lookup key always matches the deployed chart version.
            __baked="${cfg.nodeImageRepo}:$GITLAB_CHART_VERSION"
            __rt="$(detect_runtime 2>/dev/null || true)"
            if [ -n "$__rt" ] && "$__rt" image inspect "$__baked" >/dev/null 2>&1; then
              export KIND_IMAGE="$__baked"
              echo "==> using prebaked node image $__baked (no image pulls expected)"
            fi
          fi
        '';

        # Fully-qualified ref the chart composes from image.{registry,repository,name}.
        # We build/load the local dev image under this exact name + devImageTag
        # so the rendered manifest resolves to the image already present in kind.
        devImageName = "${cfg.image.registry}/${cfg.image.repository}/${cfg.image.name}";

        # --- modules (relative paths threaded so they resolve at flake root) --
        # One scope for every nix/ module: callPackageWith fills each module's
        # declared formals from here and ignores what a module doesn't ask for,
        # so a new SHARED arg is added once here instead of in every import.
        # `pkgs //` keeps modules that take a bare `pkgs`/`lib` working. Only
        # per-module specifics (source paths, the charts render, and apps'
        # cross-module wiring) remain as explicit override args below.
        moduleScope = pkgs // {
          inherit
            pkgs
            cfg
            yaml
            helm
            mkScript
            mkApp
            envDefaults
            clusterTools
            goTools
            helmTools
            devImageName
            defaultChartVersion
            kindK8sVersion
            nodeImageResolver
            ;
          inherit (shellLib) runtimeProbe cacheEnv;
        };
        callModule = lib.callPackageWith moduleScope;

        manifests = callModule ./nix/manifests.nix { chartSrc = ./deploy/chart; };
        imageMod = callModule ./nix/image.nix {
          repoSrc = ./.;
          chartVersionsFile = ./CHART_VERSIONS;
        };
        imagesMod = callModule ./nix/images.nix {
          # Offline chart render from the charts flake → GitLab component image
          # pre-seed for prewarm-images (see nix/images.nix).
          chartsRendered = gitlab-charts.packages.${system}.rendered;
        };
        snapshotMod = callModule ./nix/snapshot.nix { };
        # up-dev restores this instead of running dev_dependencies.sh when a
        # snapshot for the deployed chart version exists (see nix/apps.nix `up`).
        restoreDevExe = lib.getExe snapshotMod.restoreDev;
        # Executables of the individual cache steps, sequenced by apps.nix's
        # warm-cache meta-app (mirrors restoreDevExe). `.program` is each app's
        # getExe path, so warm-cache just runs them in order.
        cacheExes = {
          capture = imagesMod.apps.capture-images.program;
          prewarm = imagesMod.apps.prewarm-images.program;
          bake = imagesMod.apps.bake-node-image.program;
          snapshot = snapshotMod.apps.snapshot-dev.program;
        };
        appsMod = callModule ./nix/apps.nix {
          inherit restoreDevExe cacheExes;
          inherit (manifests)
            mkOperatorSetFlags
            operatorManifest
            operatorManifestDev
            operatorManifestBridge
            crOverlay
            crOverlayGateway
            ;
          inherit (imageMod) gitlabCharts image imageBridge;
        };
      in
      {
        devShells.default = tool2nix.lib.mkShellWith pkgs ./mise.toml {
          packages = with pkgs; [
            gh
            tektoncd-cli
            kubernetes-controller-tools # controller-gen
            setup-envtest
          ];
        };

        # Build artifacts you can inspect/cache/diff without a cluster.
        packages = {
          chart-deps = manifests.chartDeps; # FOD: fetched subcharts (bootstrap its hash first)
          gitlab-charts = imageMod.gitlabCharts; # FOD: bundled GitLab charts (bootstrap its hash)
          operator-manifest = manifests.operatorManifest;
          operator-manifest-dev = manifests.operatorManifestDev;
          operator-manifest-bridge = manifests.operatorManifestBridge; # dev-bridge image + bridge.enabled=true

          cr-overlay = manifests.crOverlay;
          cr-overlay-gateway = manifests.crOverlayGateway; # Gateway-API variant (default front door)
          manager = imageMod.manager; # the compiled operator binary
          manager-bridge = imageMod.managerBridge; # operator binary with the bridge server (-tags bridge)
          bridge-web = imageMod.bridgeWeb; # built bridge SPA (internal/bridge/web/dist)
          image = imageMod.image; # streamed operator container image
          image-bridge = imageMod.imageBridge; # streamed bridge-enabled image (tag dev-bridge)
          image-list = imagesMod.imageList; # GitLab component images parsed from the charts render (prewarm pre-seed)
          default = imageMod.manager;
        };

        # `nix flake check` — validates rendering offline, no cluster needed.
        # CI runs this on any runner; it is identical to running it locally.
        checks = {
          # Building this proves the manifest renders; the assertions prove it
          # is non-empty and structurally sane.
          operator-manifest =
            pkgs.runCommand "check-operator-manifest" { nativeBuildInputs = [ pkgs.yq-go ]; }
              ''
                test -s ${manifests.operatorManifest}
                # at least one Deployment must be present
                yq eval-all -e 'select(.kind == "Deployment") | .metadata.name' \
                  ${manifests.operatorManifest} >/dev/null
                touch "$out"
                # NOTE: to add schema validation, wire kubeconform here with an
                # offline -schema-location mirror (its default fetch is network-
                # bound and would break sandbox purity).
              '';
          # Beyond the generic non-empty/Deployment sanity above, prove the
          # bridge manifest actually WIRES the bridge: bridge.enabled=true must
          # surface ENABLE_BRIDGE=true on a manager Deployment container (the
          # whole reason operator-manifest-bridge exists). Guards against the
          # chart silently dropping the env if bridge.enabled stops taking.
          operator-manifest-bridge =
            pkgs.runCommand "check-operator-manifest-bridge" { nativeBuildInputs = [ pkgs.yq-go ]; }
              ''
                test -s ${manifests.operatorManifestBridge}
                yq eval-all -e '
                  [ select(.kind == "Deployment").spec.template.spec.containers[].env[]
                    | select(.name == "ENABLE_BRIDGE" and .value == "true") ] | length > 0
                ' ${manifests.operatorManifestBridge} >/dev/null
                touch "$out"
              '';
          cr-overlay = pkgs.runCommand "check-cr-overlay" { nativeBuildInputs = [ pkgs.yq-go ]; } ''
            yq eval -e '.spec.chart.values.global.ingress.enabled' \
              ${manifests.crOverlay} >/dev/null
            touch "$out"
          '';
        };

        # Disjoint union: a duplicate app name across modules is a build error,
        # not a silent last-wins shadow (`//` would quietly drop one).
        apps = lib.foldl' lib.attrsets.unionOfDisjoint { } [
          appsMod.apps
          imagesMod.apps
          snapshotMod.apps
        ];
      }
    );
}
