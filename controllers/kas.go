package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileKas(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileKasConfigMap(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileKasDeployment(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileKasService(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileKasIngress(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileKasRoutes(ctx, adapter, template); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileKasConfigMap(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.KasConfigMap(template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileKasDeployment(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	kas := gitlabctl.KasDeployment(template)

	if err := r.setDeploymentReplica(ctx, kas); err != nil {
		return err
	}

	if err := r.annotateSecretsChecksum(ctx, adapter, kas); err != nil {
		return err
	}

	err := r.createOrPatch(ctx, kas, adapter)

	return err
}

func (r *GitLabReconciler) reconcileKasService(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.KasService(template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileKasIngress(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileIngress(ctx, gitlabctl.KasIngress(adapter, template), adapter); err != nil {
		return err
	}

	if err := r.reconcileIngress(ctx, gitlabctl.KasGRPCIngress(adapter, template), adapter); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileKasRoutes(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, route := range gitlabctl.KasRoutes(template) {
		if err := r.createOrPatch(ctx, route, adapter); err != nil {
			return err
		}
	}

	return nil
}
