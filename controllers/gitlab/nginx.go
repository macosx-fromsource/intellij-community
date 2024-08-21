package gitlab

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

// NGINXConfigMaps returns the ConfigMaps of the NGINX component.
func NGINXConfigMaps(adapter gitlab.Adapter, template helm.Template, isGeo bool) []client.Object {
	result := template.Query().ObjectsByKindAndLabels(ConfigMapKind, map[string]string{
		"app": nginxComponentName(isGeo),
	})

	// Namespaces are properly set on NGINX objects in Chart version 5.6.0.
	// When all of Operator's supported CHART_VERSIONS are at or above 5.6.0,
	// we can remove this override.
	for _, cm := range result {
		cm.SetNamespace(adapter.Name().Namespace)
	}

	return result
}

// NGINXServices returns the Services of the NGINX Component.
func NGINXServices(adapter gitlab.Adapter, template helm.Template, isGeo bool) []client.Object {
	result := template.Query().ObjectsByKindAndLabels(ServiceKind, map[string]string{
		"app": nginxComponentName(isGeo),
	})

	// Namespaces are properly set on NGINX objects in Chart version 5.6.0.
	// When all of Operator's supported CHART_VERSIONS are at or above 5.6.0,
	// we can remove this override.
	for _, svc := range result {
		svc.SetNamespace(adapter.Name().Namespace)
	}

	return result
}

// NGINXServiceMonitor returns the ServiceMonitor for the NGINX Ingress component.
func NGINXServiceMonitor(template helm.Template, isGeo bool) client.Object {
	result := template.Query().ObjectByKindAndComponent(ServiceMonitorKind, nginxComponentName(isGeo))

	if result != nil {
		result.SetNamespace(template.Namespace())
	}

	return result
}

// NGINXDeployments returns the Deployments of the NGINX Component.
func NGINXDeployments(adapter gitlab.Adapter, template helm.Template, isGeo bool) []client.Object {
	result := template.Query().ObjectsByKindAndLabels(DeploymentKind, map[string]string{
		"app": nginxComponentName(isGeo),
	})

	// Namespaces are properly set on NGINX objects in Chart version 5.6.0.
	// When all of Operator's supported CHART_VERSIONS are at or above 5.6.0,
	// we can remove this override.
	for _, dep := range result {
		dep.SetNamespace(adapter.Name().Namespace)
	}

	return result
}

// NGINXAnnotations returns the annotations for Ingress objects.
func NGINXAnnotations() map[string]string {
	return map[string]string{
		"nginx.ingress.kubernetes.io/proxy-body-size":         "0",
		"nginx.ingress.kubernetes.io/proxy-buffering":         "off",
		"nginx.ingress.kubernetes.io/proxy-read-timeout":      "900",
		"nginx.ingress.kubernetes.io/proxy-request-buffering": "off",
	}
}

func nginxComponentName(isGeo bool) string {
	if isGeo {
		return NGINXGeoComponentName
	}

	return NGINXComponentName
}
