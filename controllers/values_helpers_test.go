package controllers

import (
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// IngressModeValues returns chart values that pin the legacy Ingress + nginx-ingress
// topology on, matching the upstream chart defaults prior to the Gateway API switch.
func IngressModeValues() support.Values {
	v := support.Values{}
	_ = v.SetValue("global.ingress.enabled", true)
	_ = v.SetValue("global.ingress.configureCertmanager", false)
	_ = v.SetValue("nginx-ingress.enabled", true)
	_ = v.SetValue("global.gatewayApi.enabled", false)
	_ = v.SetValue("global.gatewayApi.configureCertmanager", false)

	return v
}

// GatewayAPIModeValues returns chart values that explicitly enable the Gateway API
// topology with Envoy and cert-manager configured, matching the new upstream defaults.
func GatewayAPIModeValues() support.Values {
	v := support.Values{}
	_ = v.SetValue("global.ingress.enabled", false)
	_ = v.SetValue("global.ingress.tls.enabled", false) // Ensure no self-signed cert job is created.
	_ = v.SetValue("global.ingress.configureCertmanager", false)
	_ = v.SetValue("nginx-ingress.enabled", false)
	_ = v.SetValue("global.gatewayApi.enabled", true)
	_ = v.SetValue("global.gatewayApi.installEnvoy", true)
	_ = v.SetValue("global.gatewayApi.configureCertmanager", false)

	return v
}

// WithOverrides returns a new Values with base as the starting point and
// overrides layered on top (overrides win).
func WithOverrides(base, overrides support.Values) support.Values {
	out := support.Values{}
	_ = out.Merge(base)
	_ = out.Merge(overrides)

	return out
}
