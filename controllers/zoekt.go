package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileZoekt(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileZoektConfigMaps(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileZoektCertificate(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileZoektDeployment(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileZoektStatefulSet(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileZoektServices(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileZoektIngress(ctx, adapter, template); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileZoektConfigMaps(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	configMaps := gitlabctl.ZoektConfigMaps(template)

	for _, configMap := range configMaps {
		if err := r.createOrPatch(ctx, configMap, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileZoektStatefulSet(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	return r.createOrPatch(ctx, gitlabctl.ZoektStatefulSet(template), adapter)
}

func (r *GitLabReconciler) reconcileZoektServices(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	services := gitlabctl.ZoektServices(template)

	for _, svc := range services {
		if err := r.createOrPatch(ctx, svc, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileZoektIngress(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if ing := gitlabctl.ZoektIngress(template); ing != nil {
		return r.createOrPatch(ctx, gitlabctl.ZoektIngress(template), adapter)
	}

	return nil
}

func (r *GitLabReconciler) reconcileZoektCertificate(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if cert := gitlabctl.ZoektCertificate(template); cert != nil {
		return r.createOrPatch(ctx, cert, adapter)
	}

	return nil
}

func (r *GitLabReconciler) reconcileZoektDeployment(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	return r.createOrPatch(ctx, gitlabctl.ZoektDeployment(template), adapter)
}
