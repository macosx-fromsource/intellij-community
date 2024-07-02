package gitlab

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

// ZoektStatefulSet returns the StatefulSet for the Zoekt component.
func ZoektStatefulSet(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(StatefulSetKind, ZoektComponentName)
}

// ZoektDeployment returns the Deployment for the Zoekt component.
func ZoektDeployment(template helm.Template, adapter gitlab.Adapter) client.Object {
	deployment := template.Query().ObjectByKindAndComponent(DeploymentKind, ZoektComponentName)

	// Zoekt chart currently sets no explicit namespace.
	if deployment != nil {
		deployment.SetNamespace(adapter.Name().Namespace)
	}

	return deployment
}

// ZoektConfigMaps returns the ConfigMap for the Zoekt component.
func ZoektConfigMaps(template helm.Template) []client.Object {
	return template.Query().ObjectsByKindAndLabels(ConfigMapKind, map[string]string{"app": ZoektComponentName})
}

// ZoektIngress returns the Ingress for the Zoekt component.
func ZoektIngress(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(IngressKind, ZoektComponentName)
}

// ZoektServices returns the Services for the Zoekt component.
func ZoektServices(template helm.Template) []client.Object {
	return template.Query().ObjectsByKindAndLabels(ServiceKind, map[string]string{"app": ZoektComponentName})
}

// ZoektCertificate returns the Certificate for the Zoekt component.
func ZoektCertificate(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(CertificateKind, ZoektComponentName)
}
