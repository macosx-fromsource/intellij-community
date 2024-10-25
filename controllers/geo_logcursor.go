package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileGeoLogcursor(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.createOrPatch(ctx, gitlabctl.GeoLogcursorConfigMap(template), adapter); err != nil {
		return err
	}

	if err := r.createOrPatch(ctx, gitlabctl.GeoLogcursorDeployment(template), adapter); err != nil {
		return err
	}

	return nil
}
