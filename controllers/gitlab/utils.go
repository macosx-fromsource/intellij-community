package gitlab

const (
	// Known object kinds.
	CertificateKind             = "Certificate"
	ConfigMapKind               = "ConfigMap"
	CronJobKind                 = "CronJob"
	DaemonSetKind               = "DaemonSet"
	DeploymentKind              = "Deployment"
	HorizontalPodAutoscalerKind = "HorizontalPodAutoscaler"
	IngressKind                 = "Ingress"
	JobKind                     = "Job"
	PersistentVolumeClaimKind   = "PersistentVolumeClaim"
	PodMonitorKind              = "PodMonitor"
	SecretKind                  = "Secret"
	ServiceKind                 = "Service"
	ServiceMonitorKind          = "ServiceMonitor"
	StatefulSetKind             = "StatefulSet"

	// Kind related to the app.k8s.io Application CRD (optional).
	ApplicationKind = "Application"

	// Kinds related to Gateway API.
	HttpRouteKind        = "HTTPRoute"
	TcpRouteKind         = "TCPRoute"
	GatewayKind          = "Gateway"
	GatewayClassKind     = "GatewayClass"
	BackendTlsPolicyKind = "BackendTLSPolicy"
	// Kind related to Envoy Gateway.
	EnvoyProxyKind               = "EnvoyProxy"
	EnvoyPatchPolicyKind         = "EnvoyPatchPolicy"
	EnvoyClientTrafficPolicyKind = "ClientTrafficPolicy"
	EnvoySecurityPolicyKind      = "SecurityPolicy"

	// GitlabComponentName is the com mon name of GitLab.
	GitLabComponentName = "gitlab"

	// GitLabShellComponentName is the common name of GitLab Shell.
	GitLabShellComponentName = "gitlab-shell"

	// MigrationsComponentName is the common name of Migrations.
	MigrationsComponentName = "migrations"

	// GitLabExporterComponentName is the common name of GitLab Exporter.
	GitLabExporterComponentName = "gitlab-exporter"

	// RegistryComponentName is the common name of the Registry.
	RegistryComponentName = "registry"

	// RegistryMigrationComponentName is the name of the Registry migration job.
	RegistryMigrationComponentName = "registry-migrations"

	// WebserviceComponentName is the common name of Webservice.
	WebserviceComponentName = "webservice"

	// SharedSecretsComponentName is the common name of Shared Secrets.
	SharedSecretsComponentName = "shared-secrets"

	// GitalyComponentName is the common name of Gitaly.
	GitalyComponentName = "gitaly"

	// SidekiqComponentName is the common name of Sidekiq.
	SidekiqComponentName = "sidekiq"

	// PrometheusComponentName is the common name of Prometheus.
	PrometheusComponentName = "prometheus"

	// NGINXComponentName is the common name of NGINX Ingress.
	NGINXComponentName = "nginx-ingress"

	// NGINXGeoComponentName is the common name of NGINX Ingress for Geo traffic.
	NGINXGeoComponentName = "nginx-ingress-geo"

	// NGINXDefaultBackendComponentName is the common name of NGINX DefaultBackend.
	NGINXDefaultBackendComponentName = "defaultbackend"

	// PagesComponentName is the common name of GitLab Pages.
	PagesComponentName = "gitlab-pages"

	// PraefectComponentName is the common name of Praefect.
	PraefectComponentName = "praefect"

	// MailroomComponentName is the common name of Mailroom.
	MailroomComponentName = "mailroom"

	// KasComponentName is the common name of KAS.
	KasComponentName = "kas"

	// ToolboxComponentName is the common name of Toolbox.
	ToolboxComponentName = "toolbox"

	// ZoektComponentName is the common name of Zoekt.
	ZoektComponentName = "gitlab-zoekt"

	// GeoLogcursorComponentName is the common name of Geo Logcursor.
	GeoLogcursorComponentName = "geo-logcursor"
)

// RedisSubqueues is the array of possible Redis subqueues.
func RedisSubqueues() [5]string {
	return [5]string{"cache", "sharedState", "queues", "actioncable", "traceChunks"}
}
