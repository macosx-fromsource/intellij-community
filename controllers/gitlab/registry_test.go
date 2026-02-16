package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("Registry", func() {
	var (
		values        support.Values
		migrationsJob client.Object
	)

	JustBeforeEach(func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, values)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)
		Expect(err).ToNot(HaveOccurred())

		migrationsJob = RegistryMigrationsJob(template)
	})

	When("Migrations are enabled", func() {
		BeforeEach(func() {
			values = support.Values{}
			_ = values.SetValue("registry.database.enabled", true)
			_ = values.SetValue("registry.database.migrations.enabled", true)
		})

		It("renders the migrations job", func() {
			Expect(migrationsJob).ToNot(BeNil())
		})
	})

	When("Migrations are disabled", func() {
		BeforeEach(func() {
			values = support.Values{}
			_ = values.SetValue("gitlab.registry.database.enabled", true)
			_ = values.SetValue("gitlab.registry.migrations.enabled", false)
		})

		It("renders the migrations job", func() {
			Expect(migrationsJob).To(BeNil())
		})
	})
})
