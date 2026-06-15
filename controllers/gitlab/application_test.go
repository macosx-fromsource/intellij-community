package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("Application", func() {
	var (
		chartValues          support.Values
		applicationResources []client.Object
	)

	JustBeforeEach(func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		Expect(err).To(BeNil())
		Expect(template).NotTo(BeNil())

		applicationResources = WantedApplicationResources(template)
	})

	When("global.application.create is false", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue("global.application.create", false)
		})

		It("returns no Application resources", func() {
			Expect(applicationResources).To(BeEmpty())
		})
	})

	When("global.application.create is true", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue("global.application.create", true)
		})

		It("returns one Application resource with the correct GVK", func() {
			Expect(applicationResources).To(HaveLen(1))
			Expect(applicationResources[0].GetObjectKind().GroupVersionKind().Kind).To(Equal(ApplicationKind))
			Expect(applicationResources[0].GetObjectKind().GroupVersionKind().Group).To(Equal("app.k8s.io"))
			Expect(applicationResources[0].GetObjectKind().GroupVersionKind().Version).To(Equal("v1beta1"))
		})
	})
})
