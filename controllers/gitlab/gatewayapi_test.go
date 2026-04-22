package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

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
		for _, r := range gatewayResources {
			gatewayKinds = append(gatewayKinds, r.GetObjectKind().GroupVersionKind().Kind)
		}
	})

	When("Gateway API is disabled (default)", func() {
		BeforeEach(func() {
			if IsChartVersionOlderThan(helm.GetChartVersion(), ChartVersion97) {
				Skip("Gateway API not supported in this Chart version")
			}

			chartValues = support.Values{}
		})

		It("Templates no Gateway API resources", func() {
			Expect(gatewayResources).To(BeEmpty())
		})
	})

	When("Only Gateway is enabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue("global.gatewayApi.enabled", true)
		})

		It("Templates the non-vendor specific Gateway API resources only", func() {
			if IsChartVersionOlderThan(helm.GetChartVersion(), ChartVersion97) {
				Skip("Gateway API not supported in this Chart version")
			}

			Expect(gatewayResources).To(HaveLen(1))
			Expect(gatewayKinds).To(ContainElement(GatewayKind))
		})
	})

	When("Gateway API and Envoy resources are enabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue("global.gatewayApi.enabled", true)
			_ = chartValues.SetValue("global.gatewayApi.installEnvoy", true)

			if IsChartVersionOlderThan(helm.GetChartVersion(), ChartVersion99) {
				_ = chartValues.SetValue("global.gatewayApi.class.create", true)
			}
		})

		It("Templates the default and Envoy-specific Gateway API resources", func() {
			if IsChartVersionOlderThan(helm.GetChartVersion(), ChartVersion97) {
				Skip("Gateway API not supported in this Chart version")
			}

			Expect(gatewayKinds).To(ContainElement(GatewayKind))
			Expect(gatewayKinds).To(ContainElement(GatewayClassKind))
			Expect(gatewayKinds).To(ContainElement(EnvoyProxyKind))
		})
	})
})
