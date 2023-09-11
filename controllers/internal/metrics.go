package internal

import (
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

// PrometheusCluster returns a prometheus cluster object.
func PrometheusCluster(adapter gitlab.Adapter) *monitoringv1.Prometheus {
	labels := ResourceLabels(adapter.Name().Name, "prometheus", GitlabType)

	var replicas int32 = 2

	return &monitoringv1.Prometheus{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Prometheus",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitlab-prometheus",
			Namespace: adapter.Name().Namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.PrometheusSpec{
			Alerting: &monitoringv1.AlertingSpec{
				Alertmanagers: []monitoringv1.AlertmanagerEndpoints{
					{
						Name:      "alertmanager-main",
						Namespace: adapter.Name().Namespace,
						Port:      intstr.FromString("web"),
					},
				},
			},
			ServiceAccountName:     "prometheus-k8s",
			Replicas:               &replicas,
			ServiceMonitorSelector: &metav1.LabelSelector{},
		},
	}
}

// ExposePrometheusCluster creates a service for Prometheus.
func ExposePrometheusCluster(adapter gitlab.Adapter) *corev1.Service {
	labels := ResourceLabels(adapter.Name().Name, "prometheus", GitlabType)

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitlab-prometheus",
			Namespace: adapter.Name().Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "prometheus",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       9090,
					TargetPort: intstr.FromInt(9090),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// ExporterServiceMonitor returns the GitLab exporter service monitor.
func ExporterServiceMonitor(adapter gitlab.Adapter) *monitoringv1.ServiceMonitor {
	labels := ResourceLabels(adapter.Name().Name, "gitlab-exporter", GitlabType)

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      labels["app.kubernetes.io/instance"],
			Namespace: adapter.Name().Namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Path:     "/metrics",
					Port:     "gitlab-exporter",
					Interval: "30s",
				},
			},
		},
	}
}

// WebserviceServiceMonitor returns the Webservice service monitor.
func WebserviceServiceMonitor(adapter gitlab.Adapter) *monitoringv1.ServiceMonitor {
	labels := ResourceLabels(adapter.Name().Name, "webservice", GitlabType)

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      labels["app.kubernetes.io/instance"],
			Namespace: adapter.Name().Namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Path:     "/-/metrics",
					Port:     "http-workhorse",
					Interval: "30s",
				},
			},
		},
	}
}

// PostgresqlServiceMonitor returns the Postgres service monitor.
func PostgresqlServiceMonitor(adapter gitlab.Adapter) *monitoringv1.ServiceMonitor {
	labels := ResourceLabels(adapter.Name().Name, "postgresql", GitlabType)

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      labels["app.kubernetes.io/instance"],
			Namespace: adapter.Name().Namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Path:     "/metrics",
					Port:     "postgres-metrics",
					Interval: "30s",
				},
			},
		},
	}
}

// RedisServiceMonitor returns the Redis service monitor.
func RedisServiceMonitor(adapter gitlab.Adapter) *monitoringv1.ServiceMonitor {
	labels := ResourceLabels(adapter.Name().Name, "redis", GitlabType)

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      labels["app.kubernetes.io/instance"],
			Namespace: adapter.Name().Namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Path:     "/metrics",
					Port:     "redis-metrics",
					Interval: "30s",
				},
			},
		},
	}
}

// GitalyServiceMonitor returns the Gitaly service monitor.
func GitalyServiceMonitor(adapter gitlab.Adapter) *monitoringv1.ServiceMonitor {
	labels := ResourceLabels(adapter.Name().Name, "gitaly", GitlabType)

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      labels["app.kubernetes.io/instance"],
			Namespace: adapter.Name().Namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Path:     "/metrics",
					Port:     "gitaly-metrics",
					Interval: "30s",
				},
			},
		},
	}
}
