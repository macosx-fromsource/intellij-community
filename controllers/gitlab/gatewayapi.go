package gitlab

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/internal"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	feature "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/features"
)

func gateway(template helm.Template, adapter gitlab.Adapter) client.Object {
	if gateway := template.Query().ObjectByKindAndComponent(GatewayKind, GitLabComponentName); gateway != nil {
		if adapter.WantsFeature(feature.ConfigureGatewayCertManager) {
			annotations := gateway.GetAnnotations()
			annotations["cert-manager.io/issuer"] = internal.CertificateGatewayIssuerName(adapter)
			gateway.SetAnnotations(annotations)
		}

		return gateway
	}

	return nil
}

func gatewayClass(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(GatewayClassKind, GitLabComponentName)
}

func envoyPatchPolicy(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(EnvoyPatchPolicyKind, GitLabComponentName)
}

func envoyClientTrafficPolicy(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(EnvoyClientTrafficPolicyKind, GitLabComponentName)
}

func envoySecurityPolicy(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(EnvoySecurityPolicyKind, GitLabComponentName)
}

func envoyProxy(template helm.Template) client.Object {
	return template.Query().ObjectByKindAndComponent(EnvoyProxyKind, GitLabComponentName)
}

// WantedGatewayApiResources returns shared Gateway API resources not specific to certain workloads/components.
func WantedGatewayApiResources(template helm.Template, adapter gitlab.Adapter) []client.Object {
	var res = []client.Object{}

	if gw := gateway(template, adapter); gw != nil {
		res = append(res, gw)
	}

	if gwc := gatewayClass(template); gwc != nil {
		res = append(res, gwc)
	}

	if ep := envoyProxy(template); ep != nil {
		res = append(res, ep)
	}

	if pp := envoyPatchPolicy(template); pp != nil {
		res = append(res, pp)
	}

	if ctp := envoyClientTrafficPolicy(template); ctp != nil {
		res = append(res, ctp)
	}

	if sp := envoySecurityPolicy(template); sp != nil {
		res = append(res, sp)
	}

	return res
}
