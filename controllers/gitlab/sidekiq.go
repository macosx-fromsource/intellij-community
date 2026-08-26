package gitlab

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

// SidekiqDeployments returns the Deployments of the Sidekiq component.
func SidekiqDeployments(template helm.Template) []client.Object {
	return template.Query().ObjectsByKindAndLabels(DeploymentKind, map[string]string{
		AppLabel: SidekiqComponentName,
	})
}

// SidekiqConfigMaps returns the ConfigMaps of the Sidekiq component.
func SidekiqConfigMaps(template helm.Template) []client.Object {
	result := template.Query().ObjectsByKindAndLabels(ConfigMapKind, map[string]string{
		AppLabel: SidekiqComponentName,
	})

	for _, cm := range result {
		setInstallationType(cm)
	}

	return result
}

// SidekiqPodMonitor returns the PodMonitor of the Sidekiq component.
func SidekiqPodMonitor(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(PodMonitorKind, SidekiqComponentName)
}
