package gitlab

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

// GitalyStatefulSet returns the StatefulSet of Gitaly component.
func GitalyStatefulSet(template helm.Template) client.Object {
	results := template.Query().ObjectsByKindAndLabels(StatefulSetKind, map[string]string{AppLabel: GitalyComponentName})

	for _, result := range results {
		if _, hasStorageLabel := result.GetLabels()["storage"]; !hasStorageLabel {
			return result
		}
	}

	return nil
}

// GitalyConfigMap returns the ConfigMap of Gitaly component.
func GitalyConfigMap(template helm.Template) client.Object {
	results := template.Query().ObjectsByKindAndLabels(ConfigMapKind, map[string]string{AppLabel: GitalyComponentName})

	for _, result := range results {
		if hasPraefectSuffix := strings.HasSuffix(result.GetName(), "-praefect"); !hasPraefectSuffix {
			return result
		}
	}

	return nil
}

// GitalyService returns the Service of Gitaly component.
func GitalyService(template helm.Template) client.Object {
	results := template.Query().ObjectsByKindAndLabels(ServiceKind, map[string]string{AppLabel: GitalyComponentName})

	for _, result := range results {
		if _, hasStorageLabel := result.GetLabels()["storage"]; !hasStorageLabel {
			return result
		}
	}

	return nil
}

// GitalyServiceMonitor returns the Service of Gitaly component.
func GitalyServiceMonitor(template helm.Template) client.Object {
	results := template.Query().ObjectsByKindAndLabels(ServiceMonitorKind, map[string]string{AppLabel: GitalyComponentName})

	for _, result := range results {
		if _, hasStorageLabel := result.GetLabels()["storage"]; !hasStorageLabel {
			return result
		}
	}

	return nil
}
