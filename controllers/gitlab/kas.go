package gitlab

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func KasConfigMap(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(ConfigMapKind, KasComponentName)
}

func KasDeployment(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(DeploymentKind, KasComponentName)
}

func KasIngress(adapter gitlab.Adapter, template helm.Template) client.Object {
	return template.Query().ObjectByKindAndName(IngressKind, fmt.Sprintf("%s-%s", adapter.ReleaseName(), KasComponentName))
}

func KasGRPCIngress(adapter gitlab.Adapter, template helm.Template) client.Object {
	return template.Query().ObjectByKindAndName(IngressKind, fmt.Sprintf("%s-%s-grpc", adapter.ReleaseName(), KasComponentName))
}

// KasRoutes returns the default KAS HttpRoute and the KAS Workspaces HTTPRoute if enabled.
func KasRoutes(template helm.Template) []client.Object {
	return template.Query().ObjectsByKindAndLabels(HttpRouteKind, map[string]string{
		"app": KasComponentName,
	})
}

func KasService(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(ServiceKind, KasComponentName)
}

func KasServiceMonitor(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(ServiceMonitorKind, KasComponentName)
}

func KasPodMonitor(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(PodMonitorKind, KasComponentName)
}
