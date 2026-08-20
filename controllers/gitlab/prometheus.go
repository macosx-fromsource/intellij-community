package gitlab

import (
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var prometheusSubcharts = []string{"prometheus", "alertmanager", "prometheus-node-exporter", "prometheus-pushgateway"}

func prometheusObjectsByKind(template helm.Template, kind string) []client.Object {
	objects := template.Query().ObjectsByKindAndLabels(kind, map[string]string{
		AppLabel: PrometheusComponentName,
	})

	if len(objects) == 0 {
		for _, chart := range prometheusSubcharts {
			chartObjs := template.Query().ObjectsByKindAndLabels(kind, map[string]string{"app.kubernetes.io/name": chart})
			objects = append(objects, chartObjs...)
		}
	}

	return objects
}

func PrometheusDeployments(template helm.Template) []client.Object {
	return prometheusObjectsByKind(template, DeploymentKind)
}

func PrometheusStatefulSets(template helm.Template) []client.Object {
	return prometheusObjectsByKind(template, StatefulSetKind)
}

func PrometheusDaemonSets(template helm.Template) []client.Object {
	return prometheusObjectsByKind(template, DaemonSetKind)
}

func PrometheusServices(template helm.Template) []client.Object {
	return prometheusObjectsByKind(template, ServiceKind)
}

func PrometheusConfigMaps(template helm.Template) []client.Object {
	return prometheusObjectsByKind(template, ConfigMapKind)
}

func PrometheusPersistentVolumeClaims(template helm.Template) []client.Object {
	return prometheusObjectsByKind(template, PersistentVolumeClaimKind)
}

func PrometheusIngresses(template helm.Template) []client.Object {
	return prometheusObjectsByKind(template, IngressKind)
}
