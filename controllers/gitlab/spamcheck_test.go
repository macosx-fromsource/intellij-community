package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("Spamcheck", func() {
	When("Spamcheck is disabled", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue("global.spamcheck.enabled", false)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		It("Returns the template", func() {
			Expect(err).To(BeNil())
		})

		It("SpamcheckEnabled should return false", func() {
			Expect(adapter.WantsComponent(component.Spamcheck)).To(BeFalse())
		})

		It("Spamcheck managed resources must be nil", func() {
			Expect(SpamcheckConfigMap(template)).To(BeNil())
			Expect(SpamcheckDeployment(template)).To(BeNil())
			Expect(SpamcheckService(template)).To(BeNil())
		})
	})

	When("Spamcheck is enabled", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue("global.spamcheck.enabled", true)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		It("Returns the template", func() {
			Expect(err).To(BeNil())
		})

		It("Spamcheck managed resources must be available", func() {
			Expect(adapter.WantsComponent(component.Spamcheck)).To(BeTrue())

			cfgMap := SpamcheckConfigMap(template)
			deployment := SpamcheckDeployment(template)
			svc := SpamcheckService(template)

			Expect(cfgMap).NotTo(BeNil())
			Expect(deployment).NotTo(BeNil())
			Expect(svc).NotTo(BeNil())

			Expect(cfgMap.GetName()).To(Equal("test-spamcheck"))
			Expect(deployment.GetName()).To(Equal("test-spamcheck"))
			Expect(svc.GetName()).To(Equal("test-spamcheck"))
		})
	})
})
