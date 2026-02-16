package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	feature "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/features"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	gitlabToolboxPersistenceEnabled        = "gitlab.toolbox.persistence.enabled"
	gitlabToolboxCronJobEnabled            = "gitlab.toolbox.backups.cron.enabled"
	gitlabToolboxCronJobPersistenceEnabled = "gitlab.toolbox.backups.cron.persistence.enabled"
)

var _ = Describe("Toolbox", func() {
	var (
		chartValues                                         support.Values
		cronJobEnabled                                      bool
		cronJob                                             client.Object
		restorePersistenceEnabled, backupPersistenceEnabled bool
		restorePVC, backupPVC                               client.Object
	)

	JustBeforeEach(func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		Expect(err).To(BeNil())
		Expect(template).NotTo(BeNil())

		cronJobEnabled = adapter.WantsFeature(feature.BackupCronJob)
		cronJob = ToolboxCronJob(adapter, template)

		restorePersistenceEnabled = adapter.WantsFeature(feature.RestoreDeploymentPersistence)
		restorePVC = ToolboxDeploymentPersistentVolumeClaim(adapter, template)

		backupPersistenceEnabled = adapter.WantsFeature(feature.BackupCronJobPersistence)
		backupPVC = ToolboxCronJobPersistentVolumeClaim(adapter, template)
	})

	When("Toolbox CronJob is disabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue(gitlabToolboxCronJobEnabled, false)
		})

		It("Should not contain Toolbox CronJob resources", func() {
			Expect(cronJobEnabled).To(BeFalse())
			Expect(cronJob).To(BeNil())
		})
	})

	When("Toolbox CronJob is enabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue(gitlabToolboxCronJobEnabled, true)
		})

		It("Should not contain restore persistence resources", func() {
			Expect(restorePersistenceEnabled).To(BeFalse())
			Expect(restorePVC).To(BeNil())
		})

		It("Should contain Toolbox CronJob resources", func() {
			Expect(cronJobEnabled).To(BeTrue())
			Expect(cronJob).NotTo(BeNil())
		})

		It("Should not contain Toolbox CronJob persistence resources", func() {
			Expect(backupPersistenceEnabled).To(BeFalse())
			Expect(backupPVC).To(BeNil())
		})
	})

	When("Toolbox persistence, CronJob and CronJob persistence is enabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue(gitlabToolboxPersistenceEnabled, true)
			_ = chartValues.SetValue(gitlabToolboxCronJobEnabled, true)
			_ = chartValues.SetValue(gitlabToolboxCronJobPersistenceEnabled, true)
		})

		It("Should contain restore persistence resources", func() {
			Expect(restorePersistenceEnabled).To(BeTrue())
			Expect(restorePVC).NotTo(BeNil())
		})

		It("Should contain Toolbox CronJob resources", func() {
			Expect(cronJobEnabled).To(BeTrue())
			Expect(cronJob).NotTo(BeNil())
		})

		It("Should contain Toolbox CronJob persistence resources", func() {
			Expect(backupPersistenceEnabled).To(BeTrue())
			Expect(backupPVC).NotTo(BeNil())
		})
	})
})
