package controllers

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileMigrationsConfigMap(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.MigrationsConfigMap(adapter, template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) runMigrationsJob(ctx context.Context, adapter gitlab.Adapter, job *batchv1.Job) (bool, error) {
	if err := r.createOrPatch(ctx, job, adapter); err != nil {
		return false, err
	}

	return r.jobFinished(ctx, adapter, job)
}

func (r *GitLabReconciler) runPreMigrations(ctx context.Context, adapter gitlab.Adapter, job *batchv1.Job) (bool, error) {
	return r.runMigrationsJob(ctx, adapter, job)
}

func (r *GitLabReconciler) runAllMigrations(ctx context.Context, adapter gitlab.Adapter, template helm.Template) (bool, error) {
	job, err := gitlabctl.MigrationsJob(adapter, template)
	if err != nil {
		return false, err
	}

	return r.runMigrationsJob(ctx, adapter, job)
}
