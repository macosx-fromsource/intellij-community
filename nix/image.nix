# Operator binary (buildGoModule) + container image. `repoSrc` and
# `chartVersionsFile` are passed in so paths resolve at the flake root.
{
  pkgs,
  lib,
  cfg,
  devImageName,
  helm,
  repoSrc,
  chartVersionsFile,
}:
let
  # Only the Go inputs, so edits elsewhere (docs, charts, manifests, deploy/,
  # config/, nix/, scripts/) don't change the derivation and force a recompile.
  # Whole directories (not just *.go) so the //go:embed *.tpl assets under
  # pkg/ come along. Keep in sync if a new top-level Go dir or embed is added.
  goSrc = lib.fileset.toSource {
    root = repoSrc;
    fileset = lib.fileset.unions [
      (repoSrc + "/go.mod")
      (repoSrc + "/go.sum")
      (repoSrc + "/cmd/manager")
      (repoSrc + "/api")
      (repoSrc + "/controllers")
      (repoSrc + "/helm")
      (repoSrc + "/pkg")
      # Bridge: only what compilation needs from internal/ — the Go sources and
      # the //go:embed placeholder dir (internal/bridge/static.go embeds
      # `all:web/dist`). The SPA sources under internal/bridge/web/ have their
      # own derivation (bridgeWeb) and are overlaid at preBuild, so excluding
      # them here keeps the Go build cache stable when only the SPA changes (no
      # recompile of the non-bridge manager). The entrypoint tag files live in
      # cmd/manager/ and come along with cmd/ above.
      (lib.fileset.fileFilter (f: f.hasExt "go") (repoSrc + "/internal"))
      (repoSrc + "/internal/bridge/web/dist")
    ];
  };

  # Bridge SPA (Vue 3 + Vite), built to `dist/` for the manager's go:embed
  # (internal/bridge/static.go embeds `all:web/dist`). Committed only as a
  # `.gitkeep`, so a bridge-tagged build needs this built first. schema.d.ts is
  # committed, so `npm run build` is fully offline — we skip the Dockerfile's
  # `npx openapi-typescript` codegen step (network-bound, breaks sandbox purity).
  # Re-bootstrap `npmDepsHash` when web/package-lock.json changes:
  #   nix run nixpkgs#prefetch-npm-deps -- internal/bridge/web/package-lock.json
  bridgeWeb = pkgs.buildNpmPackage {
    pname = "gitlab-operator-bridge-web";
    version = "dev";
    src = repoSrc + "/internal/bridge/web";
    npmDepsHash = "sha256-48eV3eGDe0ctyAMif1ODtlud3NXxY1ClLp6Ze7C2FMQ=";
    # vite.config.ts reads CHART_VERSIONS at build time and bakes it into the
    # bundle, failing the build if it is absent. The SPA src is the only input
    # here, so stage the repo-root file beside it (the `./CHART_VERSIONS`
    # candidate), the way the Dockerfile's webbuilder stage copies it next to the
    # SPA. chartVersionsFile is already passed into this module for gitlabCharts.
    postPatch = ''
      cp ${chartVersionsFile} CHART_VERSIONS
    '';
    installPhase = ''
      runHook preInstall
      cp -r dist "$out"
      runHook postInstall
    '';
  };

  # Operator binary. CGO is off so it is static and drops into a minimal
  # image; the entrypoint package is cmd/manager, so the output is named
  # `manager` and matches `bin/manager`.
  # Re-bootstrap `vendorHash` when go.mod/go.sum change — see nix/README.md.
  # `bridge = true` compiles with the `bridge` build tag (wires the
  # backend-for-frontend server, see cmd/manager/bridge.go) and overlays the built
  # SPA into the go:embed dir. The bridge is absent from the default build
  # (cmd/manager/bridge_stub.go).
  # vendorHash is shared: `go mod vendor` pulls the bridge's imports (Huma) in
  # via internal/bridge regardless of the build tag, so both variants vendor the
  # same tree. Re-bootstrap when go.mod/go.sum (or the pinned Go) change.
  mkManager =
    {
      goPkgs ? pkgs,
      bridge ? false,
    }:
    goPkgs.buildGoModule {
      pname = "gitlab-operator${lib.optionalString bridge "-bridge"}";
      version = "dev";
      src = goSrc;
      vendorHash = "sha256-gAL5Vn+iGxh6W946f09eCGpUUDwGkpu51lmYuSOtI9k=";
      # main lives in cmd/manager (moved there in 6e4d37ca); building "." would
      # compile the non-main module root and install NO binary → empty $out.
      subPackages = [ "cmd/manager" ];
      env.CGO_ENABLED = 0;
      tags = lib.optionals bridge [ "bridge" ];
      # Fill the (otherwise `.gitkeep`-only) embed dir with the built SPA so
      # `//go:embed all:web/dist` resolves to a real UI. The sandbox src copy is
      # writable, so this in-place overlay is safe.
      preBuild = lib.optionalString bridge ''
        cp -r ${bridgeWeb}/* internal/bridge/web/dist/
      '';
      doCheck = false; # tests need envtest assets; run them via `nix run .#test`
    };

  # Host binary — used by `.#manager` and local runs.
  manager = mkManager { };
  # Bridge-enabled host binary (`.#manager-bridge`).
  managerBridge = mkManager { bridge = true; };

  # Linux binary for the container image: kind nodes run Linux, so a macOS
  # (Mach-O) binary fails with "exec /manager: exec format error". On a
  # Linux host this is just `manager`; on macOS we cross-compile to the
  # kind node's arch. CGO is off, so pure-Go cross-compilation needs no C
  # toolchain and stays cheap.
  crossGoPkgs =
    if pkgs.stdenv.hostPlatform.isAarch64 then
      pkgs.pkgsCross.aarch64-multiplatform
    else
      pkgs.pkgsCross.gnu64;
  managerLinux =
    if pkgs.stdenv.hostPlatform.isLinux then manager else mkManager { goPkgs = crossGoPkgs; };
  # Linux bridge binary for the bridge image, cross-compiled on macOS like above.
  managerBridgeLinux =
    if pkgs.stdenv.hostPlatform.isLinux then
      managerBridge
    else
      mkManager {
        goPkgs = crossGoPkgs;
        bridge = true;
      };

  # GitLab chart tarballs the operator serves at runtime (the image's
  # /charts; cmd/manager/main.go exits if empty). charts/ is generated, not committed,
  # so fetch the CHART_VERSIONS in an FOD like chartDeps. Re-bootstrap the
  # hash when CHART_VERSIONS changes — see nix/README.md.
  chartVersions = lib.filter (s: s != "") (
    lib.splitString "\n" (builtins.readFile chartVersionsFile)
  );
  gitlabCharts = pkgs.stdenvNoCC.mkDerivation {
    name = "gitlab-bundled-charts";
    dontUnpack = true;
    nativeBuildInputs = [
      helm
      pkgs.cacert
    ];
    buildPhase = ''
      export HOME="$TMPDIR"
      export SSL_CERT_FILE="${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
      helm repo add gitlab https://charts.gitlab.io/ >/dev/null
      helm repo update gitlab >/dev/null
      mkdir -p "$out"
      ${lib.concatMapStringsSep "\n" (v: ''
        helm fetch gitlab/gitlab --version "${v}" --destination "$out"
      '') chartVersions}
    '';
    dontInstall = true;
    outputHashMode = "recursive";
    outputHashAlgo = "sha256";
    outputHash = "sha256-G5Mo6mfjl9S62Mz0P8KtyBiIAO9xlOn7npuTfewueqY=";
  };

  # Image filesystem laid out to match the upstream Dockerfile contract:
  #   /manager        — the binary (chart hardcodes `command: [/manager]`)
  #   /charts/*.tgz    — bundled GitLab charts (HELM_CHARTS default /charts)
  #   /tmp             — helm/controller-runtime scratch space
  mkImageRoot =
    managerBin:
    pkgs.runCommand "operator-image-root" { } ''
      mkdir -p "$out" "$out/charts" "$out/tmp"
      # Locate the binary rather than hardcoding bin/manager: a cross-compiled
      # buildGoModule can emit it under bin/<goos>_<goarch>/manager instead of
      # bin/manager, so find it either way (host or cross).
      manager_bin="$(find ${managerBin}/bin -type f -name manager | head -n1)"
      if [ -z "$manager_bin" ]; then
        echo "no 'manager' binary under ${managerBin}/bin" >&2
        exit 1
      fi
      cp "$manager_bin" "$out/manager"
      cp ${gitlabCharts}/*.tgz "$out/charts/"
      chmod 1777 "$out/tmp"
    '';

  # NOTE: `nix build .#image` produces a *streamer script*, not a tarball —
  # run the result to emit the image archive on stdout (`nix run .#load-image-dev`
  # does this and pipes it into `kind load`). CA certs are bundled because
  # the operator talks to the kube API and chart repos over TLS.
  mkImage =
    { managerBin, tag }:
    pkgs.dockerTools.streamLayeredImage {
      name = devImageName;
      inherit tag;
      contents = [
        (mkImageRoot managerBin)
        pkgs.cacert
      ];
      config = {
        # The chart overrides this with `command: [/manager]`; set it anyway
        # so the image is runnable on its own.
        Entrypoint = [ "/manager" ];
        Env = [
          "HELM_CHARTS=/charts"
          "SSL_CERT_FILE=/etc/ssl/certs/ca-bundle.crt"
        ];
      };
    };

  image = mkImage {
    managerBin = managerLinux;
    tag = cfg.devImageTag;
  };
  # Bridge-enabled dev image. Distinct tag mirrors CI's `-bridge` suffix and
  # keeps it from colliding with the default `dev` image in kind.
  imageBridge = mkImage {
    managerBin = managerBridgeLinux;
    tag = "${cfg.devImageTag}-bridge";
  };
in
{
  inherit
    manager
    managerLinux
    managerBridge
    bridgeWeb
    gitlabCharts
    image
    imageBridge
    ;
}
