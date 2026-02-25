package gitlab

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

// WebserviceDeployments returns the Deployments for the Webservice component.
func WebserviceDeployments(template helm.Template) []client.Object {
	return template.Query().ObjectsByKindAndLabels(DeploymentKind, map[string]string{
		"app": WebserviceComponentName,
	})
}

// WebserviceConfigMaps returns the ConfigMaps for the Webservice component.
func WebserviceConfigMaps(template helm.Template) []client.Object {
	result := template.Query().ObjectsByKindAndLabels(ConfigMapKind, map[string]string{
		"app": WebserviceComponentName,
	})

	for _, cm := range result {
		setInstallationType(cm)
	}

	return result
}

// WebserviceServices returns the Services for the Webservice component.
func WebserviceServices(template helm.Template) []client.Object {
	return template.Query().ObjectsByKindAndLabels(ServiceKind, map[string]string{
		"app": WebserviceComponentName,
	})
}

// WebserviceServiceMonitor returns the ServiceMonitor for the Webservice component.
func WebserviceServiceMonitor(template helm.Template) client.Object {
	serviceMonitors := template.Query().ObjectsByKindAndLabels(ServiceMonitorKind, map[string]string{
		"app": WebserviceComponentName,
	})

	for _, result := range serviceMonitors {
		if !isWorkhorse(result) {
			return result
		}
	}

	return nil
}

// WebserviceWorkhorseServiceMonitor returns the ServiceMonitor for the Workhorse component.
func WebserviceWorkhorseServiceMonitor(template helm.Template) client.Object {
	serviceMonitors := template.Query().ObjectsByKindAndLabels(ServiceMonitorKind, map[string]string{
		"app": WebserviceComponentName,
	})

	for _, result := range serviceMonitors {
		if isWorkhorse(result) {
			return result
		}
	}

	return nil
}

// WebserviceIngresses returns the Ingresses for the Webservice component.
func WebserviceIngresses(template helm.Template) []client.Object {
	return template.Query().ObjectsByKindAndLabels(IngressKind, map[string]string{
		"app": WebserviceComponentName,
	})
}

func WebserviceRoute(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(HttpRouteKind, WebserviceComponentName)
}

func isWorkhorse(obj client.Object) bool {
	return strings.HasSuffix(obj.GetName(), "-workhorse")
}
