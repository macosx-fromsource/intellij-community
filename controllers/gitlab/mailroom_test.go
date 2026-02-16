package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	gitLabMailroomEnabled = "gitlab.mailroom.enabled"
	incomingEmailEnabled  = "global.appConfig.incomingEmail.enabled"
	incomingEmailSecret   = "global.appConfig.incomingEmail.password.secret" //nolint:gosec
)

var _ = Describe("CustomResourceAdapter", func() {
	if namespace == "" {
		namespace = testNamespace
	}

	Context("Mailroom", func() {
		When("Mailroom is enabled", func() {
			chartValues := support.Values{}
			_ = chartValues.SetValue(gitLabMailroomEnabled, true)
			_ = chartValues.SetValue(incomingEmailEnabled, true)
			_ = chartValues.SetValue(incomingEmailSecret, "secret_value")

			mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
			adapter := CreateMockAdapter(mockGitLab)
			template, err := GetTemplate(adapter)

			enabled := adapter.WantsComponent(component.Mailroom)
			configMap := MailroomConfigMap(adapter, template)
			deployment := MailroomDeployment(template)

			It("Should render the template", func() {
				Expect(err).To(BeNil())
				Expect(template).NotTo(BeNil())
			})

			It("Should contain Mailroom resources", func() {
				Expect(enabled).To(BeTrue())
				Expect(deployment).NotTo(BeNil())
				Expect(configMap).NotTo(BeNil())
			})
		})

		When("Mailroom is disabled", func() {
			chartValues := support.Values{}
			_ = chartValues.SetValue(gitLabMailroomEnabled, false)

			mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
			adapter := CreateMockAdapter(mockGitLab)
			template, err := GetTemplate(adapter)

			enabled := adapter.WantsComponent(component.Mailroom)
			configMap := MailroomConfigMap(adapter, template)
			deployment := MailroomDeployment(template)

			It("Should render the template", func() {
				Expect(err).To(BeNil())
				Expect(template).NotTo(BeNil())
			})

			It("Should not contain Mailroom resources", func() {
				Expect(enabled).To(BeFalse())
				Expect(deployment).To(BeNil())
				Expect(configMap).To(BeNil())
			})
		})
	})
})
