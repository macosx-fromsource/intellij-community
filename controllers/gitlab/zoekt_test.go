package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const zoektEnabled = "gitlab-zoekt.install"

var _ = Describe("Zoekt resources", func() {
	var (
		values                                        support.Values
		wantsZoekt                                    bool
		statefulSet, ingress, certificate, deployment client.Object
		services, configMaps                          []client.Object
	)

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

			// zoekt chart sets labels/selectors differently than gitlab chart
			// we override them in the controller to be consistent
			// So we verify that labels/selectors match between pods and controllers/services
			de := deployment.(*appsv1.Deployment)
			Expect(matchLabels(de.Spec.Template.Labels, de.Spec.Selector.MatchLabels)).To(BeTrue())

			sts := statefulSet.(*appsv1.StatefulSet)
			Expect(matchLabels(sts.Spec.Template.Labels, sts.Spec.Selector.MatchLabels)).To(BeTrue())

			svc1 := services[0].(*corev1.Service)
			svc2 := services[1].(*corev1.Service)

			// Ensure we have exactly 1 service matching 1 pod labels exclusively
			matchingSvc1De := matchLabels(de.Spec.Template.Labels, svc1.Spec.Selector)
			matchingSvc2De := matchLabels(de.Spec.Template.Labels, svc2.Spec.Selector)
			Expect(matchingSvc1De != matchingSvc2De).To(BeTrue())

			matchingSvc1Sts := matchLabels(sts.Spec.Template.Labels, svc1.Spec.Selector)
			matchingSvc2Sts := matchLabels(sts.Spec.Template.Labels, svc2.Spec.Selector)
			Expect(matchingSvc1Sts != matchingSvc2Sts).To(BeTrue())

			Expect(matchingSvc1De != matchingSvc1Sts).To(BeTrue())
			Expect(matchingSvc2De != matchingSvc2Sts).To(BeTrue())
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

func matchLabels(oLabels, qLabels map[string]string) bool {
	for k, v := range qLabels {
		if w, ok := oLabels[k]; !ok || v != w {
			return false
		}
	}

	return true
}
