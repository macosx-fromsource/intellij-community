package gitlab

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

// PostgresServices returns the Services of the Postgres component.
func PostgresServices(adapter gitlab.Adapter, template helm.Template) []client.Object {
	nameOverride := PostgresComponentName(adapter)

	componentLabel := gitlabComponentLabel
	componentName := DefaultPostgresComponentName

	if IsChartVersionOlderThan(adapter.DesiredVersion(), ChartVersion7) {
		componentLabel = appLabel
		componentName = nameOverride
	}

	results := template.Query().ObjectsByKindAndLabels(ServiceKind, map[string]string{
		componentLabel: componentName,
	})

	for _, s := range results {
		updateCommonLabels(adapter.ReleaseName(), nameOverride, s.GetLabels())

		// Temporary fix: patch in the namespace because the version of the PostgreSQL chart
		// we use does not specify `namespace` in the template.
		s.SetNamespace(adapter.Name().Namespace)
	}

	return results
}

// PostgresStatefulSet returns the StatefulSet of the PostgreSQL component.
func PostgresStatefulSet(adapter gitlab.Adapter, template helm.Template) client.Object {
	nameOverride := PostgresComponentName(adapter)

	componentLabel := gitlabComponentLabel
	componentName := DefaultPostgresComponentName

	if IsChartVersionOlderThan(adapter.DesiredVersion(), ChartVersion7) {
		componentLabel = appLabel
		componentName = nameOverride
	}

	objects := template.Query().ObjectsByKindAndLabels(StatefulSetKind, map[string]string{
		componentLabel: componentName,
	})

	// This should panic when the StatefulSet does not exist. This needs a better
	// error handling.
	// See: https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305
	result := objects[0]

	// Temporary fix: patch in the namespace because the version of the PostgreSQL chart
	// we use does not specify `namespace` in the template.
	result.SetNamespace(adapter.Name().Namespace)

	updateCommonLabels(adapter.ReleaseName(), nameOverride, result.GetLabels())

	return result
}

// PostgresConfigMap returns the ConfigMap of the PostgreSQL component.
func PostgresConfigMap(adapter gitlab.Adapter, template helm.Template) client.Object {
	// InitDB Configmap's name currently does not respect name overrides
	initDBConfigMap := template.Query().ObjectByKindAndName(ConfigMapKind,
		fmt.Sprintf("%s-%s-init-db", adapter.ReleaseName(), DefaultPostgresComponentName))

	updateCommonLabels(adapter.ReleaseName(), PostgresComponentName(adapter), initDBConfigMap.GetLabels())

	return initDBConfigMap
}

// PostgresService returns the common Service of the PostgreSQL component.
func PostgresService(adapter gitlab.Adapter, template helm.Template) client.Object {
	pgServices := PostgresServices(adapter, template)

	for _, s := range pgServices {
		if !strings.HasSuffix(s.GetName(), "-headless") &&
			!strings.HasSuffix(s.GetName(), "-metrics") &&
			!strings.HasSuffix(s.GetName(), "-read") {
			return s
		}
	}

	return nil
}
