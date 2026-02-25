package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileGitLabShell(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileShellConfigMaps(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileShellDeployment(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileShellService(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileShellRoute(ctx, adapter, template); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileShellDeployment(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	shell := gitlabctl.ShellDeployment(template)

	if err := r.setDeploymentReplica(ctx, shell); err != nil {
		return err
	}

	if err := r.annotateSecretsChecksum(ctx, adapter, shell); err != nil {
		return err
	}

	err := r.createOrPatch(ctx, shell, adapter)

	return err
}

func (r *GitLabReconciler) reconcileShellConfigMaps(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, cm := range gitlabctl.ShellConfigMaps(adapter, template) {
		if err := r.createOrPatch(ctx, cm, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileShellService(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.ShellService(template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileShellRoute(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.ShellRoute(template), adapter); err != nil {
		return err
	}

	return nil
}
