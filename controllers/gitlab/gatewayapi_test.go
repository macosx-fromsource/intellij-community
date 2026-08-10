package gitlab

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// backendTlsValues turns on the service-level TLS of every component that the
// chart renders a BackendTLSPolicy for. Registry additionally requires its host
// protocol to be https, which the chart asserts on.
func backendTlsValues() support.Values {
	v := support.Values{}
	_ = v.SetValue("global.hosts.registry.protocol", "https")
	_ = v.SetValue("registry.tls.enabled", true)
	_ = v.SetValue("registry.tls.caSecretName", "registry-ca")
	_ = v.SetValue("global.workhorse.tls.enabled", true)
	_ = v.SetValue("gitlab.webservice.workhorse.tls.caSecretName", "workhorse-ca")
	_ = v.SetValue("global.kas.enabled", true)
	_ = v.SetValue("global.kas.tls.enabled", true)
	_ = v.SetValue("global.kas.tls.caSecretName", "kas-ca")

	return v
}

// caSecretName returns the single CA Secret the given BackendTLSPolicy validates
// the backend certificate against.
func caSecretName(policy client.Object) string {
	btp, ok := policy.(*gatewayv1.BackendTLSPolicy)
	Expect(ok).To(BeTrue(), "%T is not a BackendTLSPolicy", policy)
	Expect(btp.Spec.Validation.CACertificateRefs).To(HaveLen(1))

	return string(btp.Spec.Validation.CACertificateRefs[0].Name)
}

var _ = Describe("Gateway API", func() {
	var chartValues support.Values
	var gatewayResources []client.Object
	var gatewayKinds []string

	JustBeforeEach(func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		Expect(err).To(BeNil())
		Expect(template).NotTo(BeNil())

		gatewayResources = WantedGatewayApiResources(template, adapter)
		gatewayKinds = nil
		for _, r := range gatewayResources {
			gatewayKinds = append(gatewayKinds, r.GetObjectKind().GroupVersionKind().Kind)
		}
	})

	When("Gateway API is disabled (ingress mode)", func() {
		BeforeEach(func() {
			chartValues = IngressModeValues()
		})

		It("Templates no Gateway API resources", func() {
			Expect(gatewayResources).To(BeEmpty())
		})
	})

	When("Only Gateway is enabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue("global.gatewayApi.enabled", true)
			_ = chartValues.SetValue("global.gatewayApi.installEnvoy", false)
			_ = chartValues.SetValue("global.ingress.enabled", false)
			_ = chartValues.SetValue("nginx-ingress.enabled", false)
		})

		It("Templates the non-vendor specific Gateway API resources only", func() {
			Expect(gatewayResources).To(HaveLen(1))
			Expect(gatewayKinds).To(ContainElement(GatewayKind))
		})
	})

	Describe("BackendTLSPolicies", func() {
		var registryPolicy, kasPolicy client.Object
		var webservicePolicies []client.Object

		JustBeforeEach(func() {
			mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
			adapter := CreateMockAdapter(mockGitLab)
			template, err := GetTemplate(adapter)

			Expect(err).To(BeNil())

			registryPolicy = RegistryBackendTlsPolicy(template)
			kasPolicy = KasBackendTlsPolicy(template)
			webservicePolicies = WebserviceBackendTlsPolicies(template)
		})

		When("the components serve TLS behind the Gateway", func() {
			BeforeEach(func() {
				chartValues = WithOverrides(GatewayAPIModeValues(), backendTlsValues())
			})

			It("Templates the BackendTLSPolicy of every component", func() {
				Expect(registryPolicy).NotTo(BeNil())
				Expect(registryPolicy.GetName()).To(Equal(fmt.Sprintf("%s-registry", releaseName)))

				Expect(kasPolicy).NotTo(BeNil())
				Expect(kasPolicy.GetName()).To(Equal(fmt.Sprintf("%s-kas", releaseName)))

				// Webservice renders one policy per entry of `webservice.deployments`.
				Expect(webservicePolicies).To(HaveLen(1))
				Expect(webservicePolicies[0].GetName()).To(Equal(fmt.Sprintf("%s-webservice-default", releaseName)))
			})

			It("Points every BackendTLSPolicy at the configured CA Secret", func() {
				Expect(caSecretName(registryPolicy)).To(Equal("registry-ca"))
				Expect(caSecretName(kasPolicy)).To(Equal("kas-ca"))
				Expect(caSecretName(webservicePolicies[0])).To(Equal("workhorse-ca"))
			})
		})

		When("the components do not serve TLS", func() {
			BeforeEach(func() {
				chartValues = GatewayAPIModeValues()
			})

			It("Templates no BackendTLSPolicy", func() {
				Expect(registryPolicy).To(BeNil())
				Expect(kasPolicy).To(BeNil())
				Expect(webservicePolicies).To(BeEmpty())
			})
		})
	})

	When("Gateway API and Envoy resources are enabled", func() {
		BeforeEach(func() {
			chartValues = GatewayAPIModeValues()
		})

		It("Templates the default and Envoy-specific Gateway API resources", func() {
			Expect(gatewayKinds).To(ContainElement(GatewayKind))
			Expect(gatewayKinds).To(ContainElement(GatewayClassKind))
			Expect(gatewayKinds).To(ContainElement(EnvoyProxyKind))
		})

		It("Orders EnvoyProxy before GatewayClass before Gateway", func() {
			envoyProxyIdx := slices.Index(gatewayKinds, EnvoyProxyKind)
			gatewayClassIdx := slices.Index(gatewayKinds, GatewayClassKind)
			gatewayIdx := slices.Index(gatewayKinds, GatewayKind)

			Expect(envoyProxyIdx).To(BeNumerically(">=", 0), "EnvoyProxy not found in gateway kinds")
			Expect(gatewayClassIdx).To(BeNumerically(">=", 0), "GatewayClass not found in gateway kinds")
			Expect(gatewayIdx).To(BeNumerically(">=", 0), "Gateway not found in gateway kinds")
			Expect(envoyProxyIdx).To(BeNumerically("<", gatewayClassIdx))
			Expect(gatewayClassIdx).To(BeNumerically("<", gatewayIdx))
		})
	})
})
