package gitlab

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/resource"
)

var _ = Describe("CustomResourceAdapter", func() {

	if namespace == "" {
		namespace = "default"
	}

	Context("Toolbox", func() {
		When("Toolbox CronJob is disabled", func() {
			chartValues := resource.Values{}

			mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
			adapter := CreateMockAdapter(mockGitLab)
			template, err := GetTemplate(adapter)

			enabled := ToolboxCronJobEnabled(adapter)
			cronJob := ToolboxCronJob(adapter)

			It("Should render the template", func() {
				Expect(err).To(BeNil())
				Expect(template).NotTo(BeNil())
			})

			It("Should not contain Toolbox CronJob resources", func() {
				Expect(enabled).To(BeFalse())
				Expect(cronJob).To(BeNil())
			})
		})

		When("Toolbox CronJob is enabled", func() {
			key := fmt.Sprintf(gitlabToolboxCronJobEnabled, ToolboxComponentName(helm.GetChartVersion()))

			chartValues := resource.Values{}
			_ = chartValues.SetValue(key, true)

			mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
			adapter := CreateMockAdapter(mockGitLab)
			template, err := GetTemplate(adapter)

			enabled := ToolboxCronJobEnabled(adapter)
			cronJob := ToolboxCronJob(adapter)

			persistenceEnabled := ToolboxCronJobPersistenceEnabled(adapter)
			cronJobPersistentVolumeClaim := ToolboxCronJobPersistentVolumeClaim(adapter)

			It("Should render the template", func() {
				Expect(err).To(BeNil())
				Expect(template).NotTo(BeNil())
			})

			It("Should contain Toolbox CronJob resources", func() {
				Expect(enabled).To(BeTrue())
				Expect(cronJob).NotTo(BeNil())
			})

			It("Should not contain Toolbox CronJob Persistence resources", func() {
				Expect(persistenceEnabled).To(BeFalse())
				Expect(cronJobPersistentVolumeClaim).To(BeNil())
			})
		})

		When("Toolbox CronJob and CronJob Persistence is enabled", func() {
			gitlabToolboxCronJobEnabled := fmt.Sprintf(gitlabToolboxCronJobEnabled, ToolboxComponentName(helm.GetChartVersion()))
			gitlabToolboxCronJobPersistenceEnabled := fmt.Sprintf(gitlabToolboxCronJobPersistenceEnabled, ToolboxComponentName(helm.GetChartVersion()))

			chartValues := resource.Values{}
			_ = chartValues.SetValue(gitlabToolboxCronJobEnabled, true)
			_ = chartValues.SetValue(gitlabToolboxCronJobPersistenceEnabled, true)

			mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
			adapter := CreateMockAdapter(mockGitLab)
			template, err := GetTemplate(adapter)

			enabled := ToolboxCronJobEnabled(adapter)
			cronJob := ToolboxCronJob(adapter)

			persistenceEnabled := ToolboxCronJobPersistenceEnabled(adapter)
			cronJobPersistentVolumeClaim := ToolboxCronJobPersistentVolumeClaim(adapter)

			It("Should render the template", func() {
				Expect(err).To(BeNil())
				Expect(template).NotTo(BeNil())
			})

			It("Should contain Toolbox CronJob resources", func() {
				Expect(enabled).To(BeTrue())
				Expect(cronJob).NotTo(BeNil())
			})

			It("Should contain Toolbox CronJob Persistence resources", func() {
				Expect(persistenceEnabled).To(BeTrue())
				Expect(cronJobPersistentVolumeClaim).NotTo(BeNil())
			})
		})
	})
})
