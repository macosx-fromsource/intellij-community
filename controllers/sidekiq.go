package controllers

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/internal"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileSidekiqConfigMaps(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, cm := range gitlabctl.SidekiqConfigMaps(template) {
		if err := r.createOrPatch(ctx, cm, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileSidekiqDeployments(ctx context.Context, adapter gitlab.Adapter, template helm.Template, pause bool) error {
	sidekiqs := gitlabctl.SidekiqDeployments(template)

	for _, sidekiq := range sidekiqs {
		sq := sidekiq.DeepCopyObject().(client.Object)

		if err := r.setDeploymentReplica(ctx, sq); err != nil {
			return err
		}

		if err := r.annotateSecretsChecksum(ctx, adapter, sq); err != nil {
			return err
		}

		if err := internal.ToggleDeploymentPause(sq, pause); err != nil {
			return err
		}

		if err := r.createOrPatch(ctx, sq, adapter); err != nil {
			return err
		}
	}

	return nil
}
