package gitlab

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

func GeoLogcursorDeployment(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(DeploymentKind, GeoLogcursorComponentName)
}

func GeoLogcursorConfigMap(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(ConfigMapKind, GeoLogcursorComponentName)
}
