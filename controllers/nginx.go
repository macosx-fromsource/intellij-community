package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileNGINX(ctx context.Context, adapter gitlab.Adapter, template helm.Template, isGeo bool) error {
	for _, cm := range gitlabctl.NGINXConfigMaps(adapter, template, isGeo) {
		if err := r.createOrPatch(ctx, cm, adapter); err != nil {
			return err
		}
	}

	for _, svc := range gitlabctl.NGINXServices(adapter, template, isGeo) {
		if err := r.createOrPatch(ctx, svc, adapter); err != nil {
			return err
		}
	}

	for _, dep := range gitlabctl.NGINXDeployments(adapter, template, isGeo) {
		if err := r.createOrPatch(ctx, dep, adapter); err != nil {
			return err
		}
	}

	return nil
}
