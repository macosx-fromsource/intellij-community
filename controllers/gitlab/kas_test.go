package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("KAS", func() {
	When("KAS is disabled", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue("global.kas.enabled", false)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		It("Returns the template", func() {
			Expect(err).To(BeNil())
		})

		It("KasEnabled should return false", func() {
			Expect(adapter.WantsComponent(component.GitLabKAS)).To(BeFalse())
		})

		It("KAS managed resources must be nil", func() {
			Expect(KasConfigMap(template)).To(BeNil())
			Expect(KasDeployment(template)).To(BeNil())
			Expect(KasIngress(adapter, template)).To(BeNil())
			Expect(KasGRPCIngress(adapter, template)).To(BeNil())
			Expect(KasService(template)).To(BeNil())
		})
	})

	When("KAS is enabled", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue("global.kas.enabled", true)
		_ = chartValues.SetValue("global.kas.service.apiExternalPort", 8153)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		It("Returns the template", func() {
			Expect(err).To(BeNil())
		})

		It("KAS managed resources must be available", func() {
			Expect(adapter.WantsComponent(component.GitLabKAS)).To(BeTrue())

			cfgMap := KasConfigMap(template)
			deployment := KasDeployment(template)
			ingress := KasIngress(adapter, template)
			svc := KasService(template)

			Expect(cfgMap).NotTo(BeNil())
			Expect(deployment).NotTo(BeNil())
			Expect(ingress).NotTo(BeNil())
			Expect(KasGRPCIngress(adapter, template)).To(BeNil())
			Expect(svc).NotTo(BeNil())

			Expect(cfgMap.GetName()).To(Equal("test-kas"))
			Expect(deployment.GetName()).To(Equal("test-kas"))
			Expect(ingress.GetName()).To(Equal("test-kas"))
			Expect(svc.GetName()).To(Equal("test-kas"))
		})
	})

	When("KAS GRPC Ingress is enabled", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue("global.kas.enabled", true)
		_ = chartValues.SetValue("gitlab.kas.ingress.grpc.enabled", true)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		It("Returns the template", func() {
			Expect(err).To(BeNil())
		})

		It("KAS GRPC Ingress resource must be available", func() {
			Expect(adapter.WantsComponent(component.GitLabKAS)).To(BeTrue())

			grpcIngress := KasGRPCIngress(adapter, template)
			Expect(grpcIngress).NotTo(BeNil())
			Expect(grpcIngress.GetName()).To(Equal("test-kas-grpc"))
		})
	})

	When("KAS GRPC Ingress is enabled and relativeUrlRoot is set", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue("global.kas.enabled", true)
		_ = chartValues.SetValue("gitlab.kas.ingress.grpc.enabled", true)
		_ = chartValues.SetValue("global.appConfig.relativeUrlRoot", "/gitlab")

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		It("Returns the template", func() {
			Expect(err).To(BeNil())
		})

		It("KAS GRPC Ingress resource must be nil", func() {
			Expect(adapter.WantsComponent(component.GitLabKAS)).To(BeTrue())
			Expect(KasGRPCIngress(adapter, template)).To(BeNil())
		})
	})

	When("Gateway API is enabled", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue("global.kas.enabled", true)
		_ = chartValues.SetValue("global.gatewayApi.enabled", true)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		It("Returns the template", func() {
			Expect(err).To(BeNil())
		})

		It("Templates the HTTPRoute", func() {
			Expect(KasRoutes(template)).To(HaveLen(1))
		})
	})
})
