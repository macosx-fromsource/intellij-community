package feature

import (
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/internal/v1beta1"
)

var (
	BackupCronJob                = v1beta1.BackupCronJob
	BackupCronJobPersistence     = v1beta1.BackupCronJobPersistence
	ConfigureCertManager         = v1beta1.ConfigureCertManager
	ReplaceGitalyWithPraefect    = v1beta1.ReplaceGitalyWithPraefect
	RestoreDeploymentPersistence = v1beta1.RestoreDeploymentPersistence
	ConfigureGatewayCertManager  = v1beta1.ConfigureGatewayCertManager
)
