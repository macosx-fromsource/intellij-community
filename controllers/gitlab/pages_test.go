package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	globalPagesEnabled = "global.pages.enabled"
)

var _ = Describe("Pages", func() {
	When("Pages is enabled", func() {
		chartValues := IngressModeValues()
		_ = chartValues.SetValue(globalPagesEnabled, true)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		enabled := adapter.WantsComponent(component.GitLabPages)
		configMap := PagesConfigMap(adapter, template)
		services := PagesServices(template)
		deployment := PagesDeployment(template)
		ingress := PagesIngress(template)

		It("Should render the template", func() {
			Expect(err).To(BeNil())
			Expect(template).NotTo(BeNil())
		})

		It("Should contain Pages resources", func() {
			Expect(enabled).To(BeTrue())
			Expect(configMap).NotTo(BeNil())
			Expect(services).NotTo(BeEmpty())
			Expect(deployment).NotTo(BeNil())
			Expect(ingress).NotTo(BeNil())
		})
	})

	When("Pages is enabled with Gateway API", func() {
		chartValues := GatewayAPIModeValues()
		_ = chartValues.SetValue(globalPagesEnabled, true)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		enabled := adapter.WantsComponent(component.GitLabPages)
		ingress := PagesIngress(template)
		route := PagesRoute(template)

		It("Should render the template", func() {
			Expect(err).To(BeNil())
			Expect(template).NotTo(BeNil())
		})

		It("Should template Pages with an HTTPRoute instead of an Ingress", func() {
			Expect(enabled).To(BeTrue())
			Expect(ingress).To(BeNil())
			Expect(route).NotTo(BeNil())
		})
	})

	When("Pages is disabled", func() {
		chartValues := support.Values{}
		_ = chartValues.SetValue(globalPagesEnabled, false)

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)

		template, err := GetTemplate(adapter)

		enabled := adapter.WantsComponent(component.GitLabPages)
		configMap := PagesConfigMap(adapter, template)
		services := PagesServices(template)
		deployment := PagesDeployment(template)
		ingress := PagesIngress(template)

		It("Should render the template", func() {
			Expect(err).To(BeNil())
			Expect(template).NotTo(BeNil())
		})

		It("Should not contain Pages resources", func() {
			Expect(enabled).To(BeFalse())
			Expect(configMap).To(BeNil())
			Expect(services).To(BeEmpty())
			Expect(deployment).To(BeNil())
			Expect(ingress).To(BeNil())
		})
	})
})
