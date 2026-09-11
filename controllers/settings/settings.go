package settings

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/chart/common"
)

var (
	// HelmChartsDirectory is the directory that contains all the bundled Charts.
	// The default value is "/charts". Use HELM_CHARTS environment variable to change it.
	HelmChartsDirectory = "/charts"

	// ManagerServiceAccount is the name of the ServiceAccount that is used by the manager.
	// The default value is "gitlab-manager". Use GITLAB_MANAGER_SERVICE_ACCOUNT environment
	// variable to change it.
	ManagerServiceAccount = "gitlab-manager"

	// AppNonRootServiceAccount is the name of the ServiceAccount that is used by GitLab components
	// that can run under the 'nonroot' SecurityContextConstraint.
	// The default value is "gitlab-app-nonroot". Use GITLAB_APP_NONROOT_SERVICE_ACCOUNT environment
	// variable to change it.
	AppNonRootServiceAccount = "gitlab-app-nonroot"

	// NGINXServiceAccount is the name of the ServiceAccount that is used by NGINX.
	// The default value is "gitlab-nginx-ingress". Use NGINX_SERVICE_ACCOUNT environment
	// variable to change it.
	NGINXServiceAccount = "gitlab-nginx-ingress"

	// PrometheusServiceAccount is the name of the ServiceAccount that is used by Prometheus.
	// The default value is "gitlab-prometheus-server". Use PROMETHEUS_SERVICE_ACCOUNT environment
	// variable to change it.
	PrometheusServiceAccount = "gitlab-prometheus-server"

	// WatchNamespace is the namespace the Operator is scoped to via the WATCH_NAMESPACE
	// environment variable. An empty string means the Operator runs with cluster scope,
	// in which case RBAC checks are performed at cluster scope.
	WatchNamespace = ""

	// HealthProbeBindAddress returns the address for hosting health probes.
	HealthProbeBindAddress = ":6060"

	// BridgeBindAddress is the address the bridge (backend-for-frontend) server binds
	// to. It serves the CRUD API, the OpenAPI document, and the SPA. Use
	// BRIDGE_BIND_ADDRESS environment variable to change it.
	BridgeBindAddress = ":8090"

	// EnableBridge controls whether the bridge (backend-for-frontend) server is
	// started. It is disabled by default. Set the ENABLE_BRIDGE environment
	// variable to a truthy value ("true", "1") to enable it.
	EnableBridge = false

	// LivenessEndpointName returns the endpoint name for the liveness probe.
	LivenessEndpointName = "/liveness"

	// ReadinessEndpointName returns the endpoint name for the readiness probe.
	ReadinessEndpointName = "/readiness"

	// ErrAliveStatus returns an error if not alive, and nil if alive.
	ErrAliveStatus = fmt.Errorf("not alive")

	// ErrReadyStatus returns an error if not ready, and nil if ready.
	ErrReadyStatus = fmt.Errorf("not ready")

	// HealthzCheck returns the checker.
	HealthzCheck = func(_ *http.Request) error { return ErrAliveStatus }
	ReadyzCheck  = func(_ *http.Request) error { return ErrReadyStatus }

	DefaultKubeVersion     *common.KubeVersion = nil
	DefaultKubeAPIVersions common.VersionSet   = common.VersionSet{}

	// DynamicChartPullEnabled lets the v2alpha1 GitLabCore controller pull a
	// GitLab chart version that HelmChartsDirectory does not carry from
	// DynamicChartRepository, rather than failing the render outright. It is
	// enabled by default; set the ENABLE_DYNAMIC_CHART_PULL environment
	// variable to a falsy value ("false", "0") to turn it off, e.g. for a
	// disconnected cluster that must never reach out to a chart repository. A
	// pulled chart is not signature- or provenance-verified (see
	// internal/render.PullChart). No other controller reads this setting:
	// v1beta1 always renders from HelmChartsDirectory and never reaches out to
	// a chart repository.
	DynamicChartPullEnabled = true

	// DynamicChartRepository is the Helm chart repository a dynamic chart pull
	// downloads from. The default value is "https://charts.gitlab.io/", the
	// official GitLab chart repository. Use the DYNAMIC_CHART_REPOSITORY
	// environment variable to change it.
	DynamicChartRepository = "https://charts.gitlab.io/"

	// DynamicChartCacheDirectory is where a dynamically pulled chart is cached
	// on disk, so a repeated reconcile does not download it again. The default
	// value is "<os.TempDir()>/gitlab-operator-charts". Use the
	// DYNAMIC_CHART_CACHE_DIRECTORY environment variable to change it.
	DynamicChartCacheDirectory = filepath.Join(os.TempDir(), "gitlab-operator-charts")

	// DynamicChartCacheTTL is how long a chart may sit in
	// DynamicChartCacheDirectory without being pulled or reused before
	// internal/render.PullChart prunes it. This is what keeps the cache from
	// growing forever as a long-lived controller renders more and more chart
	// versions over its lifetime; a version pruned too early is simply pulled
	// again on the next reconcile that needs it. The default value is 30
	// minutes. Use the DYNAMIC_CHART_CACHE_TTL environment variable (Go duration
	// syntax, e.g. "72h") to change it, or set it to "0" to disable pruning.
	DynamicChartCacheTTL = 30 * time.Minute

	// DynamicChartAllowHTTP overrides the default protocol restriction
	// internal/render.PullChart and internal/render.RemoteChartVersions place
	// on DynamicChartRepository, which otherwise accepts only an "https://"
	// URL. It is disabled by default: a plain "http://" repository puts a
	// chart archive on the wire unauthenticated and unencrypted. Set the
	// DYNAMIC_CHART_ALLOW_HTTP environment variable to a truthy value ("true",
	// "1") to allow it anyway, e.g. for a disconnected cluster whose only
	// reachable mirror serves plain HTTP.
	DynamicChartAllowHTTP = false
)

const (
	envHelmChartsDirectory      = "HELM_CHARTS"
	envManagerServiceAccount    = "GITLAB_MANAGER_SERVICE_ACCOUNT"
	envAppNonRootServiceAccount = "GITLAB_APP_NONROOT_SERVICE_ACCOUNT"
	envNGINXServiceAccount      = "NGINX_SERVICE_ACCOUNT"
	envPrometheusServiceAccount = "PROMETHEUS_SERVICE_ACCOUNT"
	envKubeVersion              = "GITLAB_OPERATOR_KUBERNETES_VERSION"
	envKubeAPIVersions          = "GITLAB_OPERATOR_KUBERNETES_API_VERSIONS"
	envWatchNamespace           = "WATCH_NAMESPACE"
	envBridgeBindAddress        = "BRIDGE_BIND_ADDRESS"
	envEnableBridge             = "ENABLE_BRIDGE"
	envEnableDynamicChartPull   = "ENABLE_DYNAMIC_CHART_PULL"
	envDynamicChartRepository   = "DYNAMIC_CHART_REPOSITORY"
	envDynamicChartCacheDir     = "DYNAMIC_CHART_CACHE_DIRECTORY"
	envDynamicChartCacheTTL     = "DYNAMIC_CHART_CACHE_TTL"
	envDynamicChartAllowHTTP    = "DYNAMIC_CHART_ALLOW_HTTP"
)

// Load reads Operator settings from environment variables.
func Load() {
	helmChartsDirectory := os.Getenv(envHelmChartsDirectory)
	if helmChartsDirectory != "" {
		HelmChartsDirectory = helmChartsDirectory
	}

	mgrServiceAccount := os.Getenv(envManagerServiceAccount)
	if mgrServiceAccount != "" {
		ManagerServiceAccount = mgrServiceAccount
	}

	appNonRootServiceAccount := os.Getenv(envAppNonRootServiceAccount)
	if appNonRootServiceAccount != "" {
		AppNonRootServiceAccount = appNonRootServiceAccount
	}

	nginxServiceAccount := os.Getenv(envNGINXServiceAccount)
	if nginxServiceAccount != "" {
		NGINXServiceAccount = nginxServiceAccount
	}

	prometheusServiceAccount := os.Getenv(envPrometheusServiceAccount)
	if prometheusServiceAccount != "" {
		PrometheusServiceAccount = prometheusServiceAccount
	}

	kubeVersionStr := os.Getenv(envKubeVersion)
	if kubeVersionStr != "" {
		DefaultKubeVersion, _ = common.ParseKubeVersion(kubeVersionStr)
	}

	kubeAPIVersionsStr := os.Getenv(envKubeAPIVersions)
	if kubeAPIVersionsStr != "" {
		DefaultKubeAPIVersions = strings.Split(kubeAPIVersionsStr, ",")
	}

	WatchNamespace = os.Getenv(envWatchNamespace)

	bridgeBindAddress := os.Getenv(envBridgeBindAddress)
	if bridgeBindAddress != "" {
		BridgeBindAddress = bridgeBindAddress
	}

	if enableBridge, err := strconv.ParseBool(os.Getenv(envEnableBridge)); err == nil {
		EnableBridge = enableBridge
	}

	if enableDynamicChartPullStr := os.Getenv(envEnableDynamicChartPull); enableDynamicChartPullStr != "" {
		if enableDynamicChartPull, err := strconv.ParseBool(enableDynamicChartPullStr); err == nil {
			DynamicChartPullEnabled = enableDynamicChartPull
		} else {
			warnInvalidEnv(envEnableDynamicChartPull, enableDynamicChartPullStr, err, DynamicChartPullEnabled)
		}
	}

	if dynamicChartRepository := os.Getenv(envDynamicChartRepository); dynamicChartRepository != "" {
		DynamicChartRepository = dynamicChartRepository
	}

	if dynamicChartCacheDir := os.Getenv(envDynamicChartCacheDir); dynamicChartCacheDir != "" {
		DynamicChartCacheDirectory = dynamicChartCacheDir
	}

	if dynamicChartCacheTTLStr := os.Getenv(envDynamicChartCacheTTL); dynamicChartCacheTTLStr != "" {
		if dynamicChartCacheTTL, err := time.ParseDuration(dynamicChartCacheTTLStr); err == nil {
			DynamicChartCacheTTL = dynamicChartCacheTTL
		} else {
			// A plausible typo, e.g. "30" for "30m": time.ParseDuration requires
			// a unit, and silently keeping the default here would leave no sign
			// anything was wrong.
			warnInvalidEnv(envDynamicChartCacheTTL, dynamicChartCacheTTLStr, err, DynamicChartCacheTTL)
		}
	}

	if dynamicChartAllowHTTPStr := os.Getenv(envDynamicChartAllowHTTP); dynamicChartAllowHTTPStr != "" {
		if dynamicChartAllowHTTP, err := strconv.ParseBool(dynamicChartAllowHTTPStr); err == nil {
			DynamicChartAllowHTTP = dynamicChartAllowHTTP
		} else {
			warnInvalidEnv(envDynamicChartAllowHTTP, dynamicChartAllowHTTPStr, err, DynamicChartAllowHTTP)
		}
	}
}

// warnInvalidEnv reports, on stderr, that the named environment variable
// carried a value Load could not parse, so it kept the given default instead
// of failing outright. Load runs before a structured logger exists (main's
// init, ahead of ctrl.SetLogger), and this package takes no logging
// dependency of its own, so stderr is what makes a bad setting visible in
// the Pod's own logs rather than silently ignored.
func warnInvalidEnv(name, value string, err error, kept any) {
	fmt.Fprintf(os.Stderr, "settings: %s=%q is invalid (%v), keeping %v\n", name, value, err, kept)
}
