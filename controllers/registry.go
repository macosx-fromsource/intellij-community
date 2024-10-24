package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileRegistry(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileRegistryConfigMap(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileRegistryMigrationsJob(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileRegistryDeployment(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileRegistryService(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileRegistryIngress(ctx, adapter, template); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileRegistryConfigMap(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.RegistryConfigMap(adapter, template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileRegistryDeployment(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	registry := gitlabctl.RegistryDeployment(template)

	if err := r.setDeploymentReplica(ctx, registry); err != nil {
		return err
	}

	if err := r.annotateSecretsChecksum(ctx, adapter, registry); err != nil {
		return err
	}

	err := r.createOrPatch(ctx, registry, adapter)

	return err
}

func (r *GitLabReconciler) reconcileRegistryService(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.RegistryService(template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileRegistryIngress(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileIngress(ctx, gitlabctl.RegistryIngress(template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileRegistryMigrationsJob(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	migrationsJob := gitlabctl.RegistryMigrationsJob(template)
	if migrationsJob == nil {
		return nil
	}

	if exists, err := r.jobExists(ctx, migrationsJob); err != nil {
		return err
	} else if exists {
		r.Log.V(2).Info("registry migrations Job already exists")
		return nil
	}

	return r.createOrPatch(ctx, migrationsJob, adapter)
}
