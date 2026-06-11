package settings

import (
	"fmt"
	"net/http"
	"os"
	"strings"

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
}
