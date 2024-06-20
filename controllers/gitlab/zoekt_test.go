package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const zoektEnabled = "gitlab-zoekt.install"

var _ = Describe("Zoekt resources", func() {
	var values support.Values
	var wantsZoekt bool
	var statefulSet, ingress, certificate, deployment client.Object
	var services, configMaps []client.Object

	JustBeforeEach(func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, values)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)
		Expect(err).To(BeNil())

		wantsZoekt = adapter.WantsComponent(component.Zoekt)
		statefulSet = ZoektStatefulSet(template)
		deployment = ZoektDeployment(template, adapter)
		services = ZoektServices(template)
		ingress = ZoektIngress(template)
		certificate = ZoektCertificate(template)
		configMaps = ZoektConfigMaps(template)
	})

	When("Zoekt is enabled", func() {
		BeforeEach(func() {
			values = support.Values{}
			_ = values.SetValue(zoektEnabled, true)
			_ = values.SetValue("gitlab-zoekt.gateway.tls.certificate.create", true)
			_ = values.SetValue("gitlab-zoekt.ingress.enabled", true)
		})

		It("Should contain Zoekt resources", func() {
			Expect(wantsZoekt).To(BeTrue())
			Expect(statefulSet).NotTo(BeNil())
			Expect(deployment).NotTo(BeNil())
			Expect(services).To(HaveLen(2))
			Expect(ingress).NotTo(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(configMaps).To(HaveLen(2))
		})
	})

	When("Zoekt is disabled", func() {
		BeforeEach(func() {
			values = support.Values{}
			_ = values.SetValue(zoektEnabled, false)
		})

		It("Should not contain Zoekt resources", func() {
			Expect(wantsZoekt).To(BeFalse())
			Expect(statefulSet).To(BeNil())
			Expect(deployment).To(BeNil())
			Expect(services).To(HaveLen(0))
			Expect(ingress).To(BeNil())
			Expect(certificate).To(BeNil())
			Expect(configMaps).To(HaveLen(0))
		})
	})
})
