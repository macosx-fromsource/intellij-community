# Pure operator-manifest + CR-overlay rendering. No cluster needed.
# Imported from flake.nix with the shared context; `chartSrc` is passed in so
# the chart path resolves at the flake root, not relative to this file.
{
  pkgs,
  lib,
  cfg,
  yaml,
  helm,
  chartSrc,
}:
let
  # Operator-manifest subcharts. `helm template` needs them present in
  # charts/, but they are not committed, so fetch them in a fixed-output
  # derivation (the one place Nix allows network access) keyed by outputHash.
  # Re-bootstrap the hash when Chart.lock changes — see nix/README.md.
  chartDeps = pkgs.stdenvNoCC.mkDerivation {
    name = "gitlab-operator-chart-deps";
    src = chartSrc;
    nativeBuildInputs = [
      helm
      pkgs.cacert
      pkgs.coreutils
      pkgs.yq-go
    ];
    dontConfigure = true;
    dontInstall = true;
    buildPhase = ''
      export HOME="$TMPDIR"
      export SSL_CERT_FILE="${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
      # We are already in the unpacked, writable chart source root.
      # `helm dependency build` needs each http(s) repo registered first.
      # Derive them from Chart.yaml so new deps work without edits here.
      yq '.dependencies[].repository' Chart.yaml 2>/dev/null \
        | while read -r url; do
            case "$url" in
              http://*|https://*)
                helm repo add "repo-$(printf '%s' "$url" | sha1sum | cut -c1-12)" \
                  "$url" >/dev/null 2>&1 || true ;;
            esac
          done
      helm dependency build
      mkdir -p "$out"
      cp charts/*.tgz "$out"/
    '';
    outputHashMode = "recursive";
    outputHashAlgo = "sha256";
    outputHash = "sha256-kWo+O9sYsV/r69IGN9DDT+StRWA8/DMHEuSMdzXX7Js=";
  };

  # The operator chart's `helm --set` flags, single-sourced so the pure
  # manifest render and the build-operator escape hatch share one key list
  # and can't drift. Callers supply the value strings: static cfg values
  # for the pure derivation, quoted shell vars for the app.
  mkOperatorSetFlags =
    {
      nameOverride,
      registry,
      repository,
      name,
      tag,
      clusterMode,
      pullPolicy ? null,
    }:
    lib.concatStringsSep " " (
      [
        "--set nameOverride=${nameOverride}"
        "--set image.registry=${registry}"
        "--set image.repository=${repository}"
        "--set image.name=${name}"
        "--set image.tag=${tag}"
        "--set watchCluster=${clusterMode}"
        "--set nginx-ingress.create=${clusterMode}"
        "--set prometheus.serviceAccount.server.create=${clusterMode}"
      ]
      ++ lib.optional (pullPolicy != null) "--set image.pullPolicy=${pullPolicy}"
    );

  # Parameterised so we can render both the default manifest (registry
  # image, tag=latest) and a dev manifest (local image, pullPolicy=Never).
  # Overlays the FOD-fetched subcharts into a writable chart copy, then
  # templates fully offline.
  mkOperatorManifest =
    {
      tag ? cfg.image.tag,
      pullPolicy ? null,
      bridgeEnabled ? false,
    }:
    pkgs.runCommand "operator.yaml" { nativeBuildInputs = [ helm ]; } ''
      export HOME="$TMPDIR"   # helm wants a writable HOME/cache
      work="$TMPDIR/chart"
      cp -r ${chartSrc} "$work" && chmod -R u+w "$work"
      mkdir -p "$work/charts"
      cp ${chartDeps}/*.tgz "$work/charts/"
      helm template "$work" --include-crds \
        --namespace ${cfg.namespace} \
        ${mkOperatorSetFlags {
          nameOverride = cfg.nameOverride;
          registry = cfg.image.registry;
          repository = cfg.image.repository;
          name = cfg.image.name;
          inherit tag pullPolicy;
          clusterMode = lib.boolToString cfg.clusterMode;
        }} \
        ${lib.optionalString bridgeEnabled "--set bridge.enabled=true"} \
        > "$out"
    '';

  # Default: published registry image, tag=latest.
  operatorManifest = mkOperatorManifest { };
  # Dev: the locally-built image (tag=dev), never pulled — kind must have
  # it loaded via `nix run .#load-image`.
  operatorManifestDev = mkOperatorManifest {
    tag = cfg.devImageTag;
    pullPolicy = "Never";
  };
  # Bridge dev: the locally-built bridge image (tag=dev-bridge), never pulled,
  # with the chart's bridge.enabled=true so the manager injects ENABLE_BRIDGE
  # and opens the bridge container port. Load the image via
  # `nix run .#load-image-bridge`. The bridge exposes only a container port (no
  # Service/Ingress) — reach it with `kubectl port-forward`.
  operatorManifestBridge = mkOperatorManifest {
    tag = "${cfg.devImageTag}-bridge";
    pullPolicy = "Never";
    bridgeEnabled = true;
  };

  # Static half of the GitLab CR (kind-sizing / ingress / pages settings)
  # as structured Nix data, serialised to YAML at build time. The two
  # runtime-dependent scalars — chart version and the <ip>.nip.io domain —
  # are injected at deploy time via `yq` `strenv`.
  #
  # Transport-independent values shared by both front-door modes below.
  commonValues = {
    global = {
      pages.enabled = true;
      appConfig.object_store = {
        enabled = true;
        proxy_download = true;
      };
      shell.port = cfg.sshPort;
    };
    gitlab = {
      "gitlab-shell" = {
        minReplicas = 1;
        maxReplicas = 1;
      };
      "gitlab-exporter".enabled = true;
      webservice = {
        minReplicas = 1;
        maxReplicas = 1;
      };
    };
    registry.hpa = {
      minReplicas = 1;
      maxReplicas = 1;
    };
  };

  # Opt-in front door (NGINX_INGRESS=1): classic Ingress served by the bundled
  # nginx-ingress. Envoy Gateway is the default (gatewayValues below).
  ingressValues = {
    global = {
      ingress = {
        enabled = true;
        configureCertmanager = false;
        tls.secretName = cfg.tlsSecretName;
      };
      gatewayApi = {
        enabled = false;
        installEnvoy = false;
        configureCertmanager = false;
      };
    };
    gitlab."gitlab-pages".ingress.tls.secretName = cfg.pagesTlsSecretName;
    "nginx-ingress" = {
      enabled = true;
      controller = {
        ingressClassResource.enabled = true;
        service.nodePorts.https = cfg.httpsNodePort;
        replicaCount = 1;
        minAvailable = 1;
      };
      defaultBackend.replicaCount = 1;
    };
  };

  # Gateway-API front door: Envoy Gateway (the chart's own 10.1.0 default).
  # nginx-ingress is off; Envoy is exposed as a NodePort service (kind has no
  # cloud LoadBalancer); listener TLS reuses the self-signed wildcard secrets
  # the provision scripts already create, so we avoid cert-manager/Let's Encrypt.
  gatewayValues = {
    global = {
      ingress.enabled = false;
      gatewayApi = {
        enabled = true;
        installEnvoy = true;
        configureCertmanager = false;
      };
    };
    "nginx-ingress".enabled = false;
    gatewayApiResources = {
      # Expose Envoy as a NodePort (kind has no cloud LoadBalancer) and pin the
      # HTTPS (:443) nodePort to 32443 so the kind host mapping 443->32443
      # reaches it — same external path nginx-ingress used. The patch is a
      # strategic merge keyed by Service port number (patchMergeKey=port), so it
      # matches port 443 regardless of Envoy's generated port name.
      envoy.proxySpec.provider = {
        type = "Kubernetes";
        kubernetes.envoyService = {
          type = "NodePort";
          patch = {
            type = "StrategicMerge";
            value.spec.ports = [
              {
                port = 443;
                nodePort = cfg.httpsNodePort;
              }
            ];
          };
        };
      };
      gateway.listeners =
        let
          webCert = [ { name = cfg.tlsSecretName; } ];
          pagesCert = [ { name = cfg.pagesTlsSecretName; } ];
        in
        {
          gitlab-web.tls.certificateRefs = webCert;
          gitlab-web-geo.tls.certificateRefs = webCert;
          gitlab-smartcard-web.tls.certificateRefs = webCert;
          registry-web.tls.certificateRefs = webCert;
          kas-web.tls.certificateRefs = webCert;
          kas-workspaces-web.tls.certificateRefs = webCert;
          openbao-web.tls.certificateRefs = webCert;
          pages-web.tls.certificateRefs = pagesCert;
        };
    };
  };

  mkCrOverlay =
    name: modeValues:
    yaml name { spec.chart.values = lib.recursiveUpdate commonValues modeValues; };
  crOverlay = mkCrOverlay "gitlab-cr-overlay.yaml" ingressValues;
  crOverlayGateway = mkCrOverlay "gitlab-cr-overlay-gateway.yaml" gatewayValues;
in
{
  inherit
    chartDeps
    mkOperatorSetFlags
    mkOperatorManifest
    operatorManifest
    operatorManifestDev
    operatorManifestBridge
    crOverlay
    crOverlayGateway
    ;
}
