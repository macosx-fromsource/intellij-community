package gitlab

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
			Expect(KasEnabled(adapter)).To(BeFalse())
		})

		It("KAS managed resources must be nil", func() {
			Expect(KasConfigMap(template)).To(BeNil())
			Expect(KasDeployment(template)).To(BeNil())
			Expect(KasIngress(template)).To(BeNil())
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
			Expect(KasEnabled(adapter)).To(BeTrue())

			cfgMap := KasConfigMap(template)
			deployment := KasDeployment(template)
			ingress := KasIngress(template)
			svc := KasService(template)

			Expect(cfgMap).NotTo(BeNil())
			Expect(deployment).NotTo(BeNil())
			Expect(ingress).NotTo(BeNil())
			Expect(svc).NotTo(BeNil())

			Expect(cfgMap.GetName()).To(Equal("test-kas"))
			Expect(deployment.GetName()).To(Equal("test-kas"))
			Expect(ingress.GetName()).To(Equal("test-kas"))
			Expect(svc.GetName()).To(Equal("test-kas"))
		})
	})

})
