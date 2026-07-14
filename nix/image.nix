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
      (repoSrc + "/main.go")
      (repoSrc + "/api")
      (repoSrc + "/controllers")
      (repoSrc + "/helm")
      (repoSrc + "/pkg")
    ];
  };

  # Operator binary. CGO is off so it is static and drops into a minimal
  # image; the output is renamed to `manager` to match `bin/manager`.
  # Re-bootstrap `vendorHash` when go.mod/go.sum change — see nix/README.md.
  mkManager =
    goPkgs:
    goPkgs.buildGoModule {
      pname = "gitlab-operator";
      version = "dev";
      src = goSrc;
      vendorHash = "sha256-V6I0BPOfFDq9jUu2gLDmsQey8kG2ctCIXXMHgSHCGBQ=";
      subPackages = [ "." ];
      env.CGO_ENABLED = 0;
      # Name the binary `manager` regardless of the module's base name.
      postInstall = ''
        if [ -e "$out/bin/gitlab-operator" ]; then
          mv "$out/bin/gitlab-operator" "$out/bin/manager"
        fi
      '';
      doCheck = false; # tests need envtest assets; run them via `nix run .#test`
    };

  # Host binary — used by `.#manager` and local runs.
  manager = mkManager pkgs;

  # Linux binary for the container image: kind nodes run Linux, so a macOS
  # (Mach-O) binary fails with "exec /manager: exec format error". On a
  # Linux host this is just `manager`; on macOS we cross-compile to the
  # kind node's arch. CGO is off, so pure-Go cross-compilation needs no C
  # toolchain and stays cheap.
  managerLinux =
    if pkgs.stdenv.hostPlatform.isLinux then
      manager
    else
      mkManager (
        if pkgs.stdenv.hostPlatform.isAarch64 then
          pkgs.pkgsCross.aarch64-multiplatform
        else
          pkgs.pkgsCross.gnu64
      );

  # GitLab chart tarballs the operator serves at runtime (the image's
  # /charts; main.go exits if empty). charts/ is generated, not committed,
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
    outputHash = "sha256-bKsV3eWhm128/VCAKTHQfWO4Soe4Kq14E81wCA4KQGY=";
  };

  # Image filesystem laid out to match the upstream Dockerfile contract:
  #   /manager        — the binary (chart hardcodes `command: [/manager]`)
  #   /charts/*.tgz    — bundled GitLab charts (HELM_CHARTS default /charts)
  #   /tmp             — helm/controller-runtime scratch space
  imageRoot = pkgs.runCommand "operator-image-root" { } ''
    mkdir -p "$out" "$out/charts" "$out/tmp"
    cp ${managerLinux}/bin/manager "$out/manager"
    cp ${gitlabCharts}/*.tgz "$out/charts/"
    chmod 1777 "$out/tmp"
  '';

  # NOTE: `nix build .#image` produces a *streamer script*, not a tarball —
  # run the result to emit the image archive on stdout (`nix run .#load-image-dev`
  # does this and pipes it into `kind load`). CA certs are bundled because
  # the operator talks to the kube API and chart repos over TLS.
  image = pkgs.dockerTools.streamLayeredImage {
    name = devImageName;
    tag = cfg.devImageTag;
    contents = [
      imageRoot
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
in
{
  inherit
    manager
    managerLinux
    gitlabCharts
    image
    ;
}
