package gitlab

import (
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

// ZoektStatefulSet returns the StatefulSet for the Zoekt component.
// Zoekt chart currently sets labels differently than GitLab Charts,
// so we need to manually adjust the selector and template labels here.
func ZoektStatefulSet(template helm.Template) client.Object {
	obj := template.Query().ObjectByKindAndComponent(StatefulSetKind, ZoektComponentName)

	if obj == nil {
		return nil
	}

	sts := obj.(*appsv1.StatefulSet)
	sts = sts.DeepCopy()

	name := sts.Spec.Selector.MatchLabels["app.kubernetes.io/name"]
	sts.Labels["app"] = name
	sts.Spec.Selector.MatchLabels = map[string]string{
		"app":     name,
		"release": sts.Labels["release"],
	}
	sts.Spec.Template.Labels["app"] = name
	sts.Spec.Template.Labels["release"] = sts.Labels["release"]
	sts.Spec.Template.Labels["app.kubernetes.io/name"] = name

	return sts
}

// ZoektDeployment returns the Deployment for the Zoekt component.
// Zeokt chart currently sets labels differently than GitLab Charts,
// so we need to manually adjust the selector and template labels here.
func ZoektDeployment(template helm.Template) client.Object {
	obj := template.Query().ObjectByKindAndComponent(DeploymentKind, ZoektComponentName)

	if obj == nil {
		return nil
	}

	deployment := obj.(*appsv1.Deployment)
	deployment = deployment.DeepCopy()

	name := deployment.Spec.Selector.MatchLabels["app.kubernetes.io/name"]
	deployment.Labels["app"] = name
	deployment.Spec.Selector.MatchLabels = map[string]string{
		"app":     name,
		"release": deployment.Labels["release"],
	}
	deployment.Spec.Template.Labels["app"] = name
	deployment.Spec.Template.Labels["release"] = deployment.Labels["release"]
	deployment.Spec.Template.Labels["app.kubernetes.io/name"] = name

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
