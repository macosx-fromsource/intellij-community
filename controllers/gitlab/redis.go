package gitlab

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

// RedisConfigMaps returns the ConfigMaps of the Redis component.
func RedisConfigMaps(adapter gitlab.Adapter, template helm.Template) []client.Object {
	nameOverride := RedisComponentName(adapter)

	componentLabel := gitlabComponentLabel
	componentName := DefaultRedisComponentName

	if IsChartVersionOlderThan(adapter.DesiredVersion(), ChartVersion7) {
		componentLabel = appLabel
		componentName = nameOverride
	}

	result := template.Query().ObjectsByKindAndLabels(ConfigMapKind, map[string]string{
		componentLabel: componentName,
	})

	for _, c := range result {
		updateCommonLabels(adapter.ReleaseName(), nameOverride, c.GetLabels())
	}

	return result
}

// RedisServices returns the Services of the Redis component.
func RedisServices(adapter gitlab.Adapter, template helm.Template) []client.Object {
	nameOverride := RedisComponentName(adapter)

	componentLabel := gitlabComponentLabel
	componentName := DefaultRedisComponentName

	if IsChartVersionOlderThan(adapter.DesiredVersion(), ChartVersion7) {
		componentLabel = appLabel
		componentName = nameOverride
	}

	results := template.Query().ObjectsByKindAndLabels(ServiceKind, map[string]string{
		componentLabel: componentName,
	})

	for _, s := range results {
		updateCommonLabels(adapter.ReleaseName(), RedisComponentName(adapter), s.GetLabels())
	}

	return results
}

// RedisMasterService returns the Master Service of the Redis component.
func RedisMasterService(adapter gitlab.Adapter, template helm.Template) client.Object {
	redisServices := RedisServices(adapter, template)

	for _, s := range redisServices {
		if strings.HasSuffix(s.GetName(), "-master") {
			return s
		}
	}

	return nil
}

// RedisStatefulSet returns the Statefulset of the Redis component.
func RedisStatefulSet(adapter gitlab.Adapter, template helm.Template) client.Object {
	nameOverride := RedisComponentName(adapter)

	componentLabel := gitlabComponentLabel
	componentName := DefaultRedisComponentName

	if IsChartVersionOlderThan(adapter.DesiredVersion(), ChartVersion7) {
		componentLabel = appLabel
		componentName = nameOverride
	}

	results := template.Query().ObjectsByKindAndLabels(StatefulSetKind, map[string]string{
		componentLabel: componentName,
	})

	if len(results) == 0 {
		return nil
	}

	return results[0]
}
