package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	feature "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/features"
)

func (r *GitLabReconciler) reconcileToolbox(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileToolboxConfigMap(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileToolboxPersistentVolumeClaims(ctx, adapter, template); err != nil {
		return err
	}

	if adapter.WantsFeature(feature.BackupCronJob) {
		if err := r.reconcileToolboxCronJob(ctx, adapter, template); err != nil {
			return err
		}
	}

	if err := r.reconcileToolboxDeployment(ctx, adapter, template); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileToolboxConfigMap(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.ToolboxConfigMap(adapter, template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileToolboxCronJob(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.ToolboxCronJob(adapter, template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileToolboxDeployment(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	deployment := gitlabctl.ToolboxDeployment(adapter, template)

	if err := r.annotateSecretsChecksum(ctx, adapter, deployment); err != nil {
		return err
	}

	err := r.createOrPatch(ctx, deployment, adapter)

	return err
}

func (r *GitLabReconciler) reconcileToolboxPersistentVolumeClaims(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if adapter.WantsFeature(feature.RestoreDeploymentPersistence) {
		if err := r.createOrPatch(ctx, gitlabctl.ToolboxDeploymentPersistentVolumeClaim(adapter, template), adapter); err != nil {
			return err
		}
	}

	if adapter.WantsFeature(feature.BackupCronJobPersistence) {
		if err := r.createOrPatch(ctx, gitlabctl.ToolboxCronJobPersistentVolumeClaim(adapter, template), adapter); err != nil {
			return err
		}
	}

	return nil
}
