package controllers

import (
	"context"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) runSharedSecretsJob(ctx context.Context, adapter gitlab.Adapter, template helm.Template) (bool, error) {
	cfgMap, job, err := gitlabctl.SharedSecretsResources(adapter, template)
	if err != nil {
		return false, err
	}

	if cfgMap == nil || job == nil {
		r.Log.V(2).Info("shared secrets Job skipped, not needed per configuration", "gitlab", adapter.Name())

		return true, nil
	}

	if err := r.createOrPatch(ctx, cfgMap, adapter); err != nil {
		return false, err
	}

	if err := r.createOrPatch(ctx, job, adapter); err != nil {
		return false, err
	}

	return r.jobFinished(ctx, adapter, job)
}

func (r *GitLabReconciler) runSelfSignedCertsJob(ctx context.Context, adapter gitlab.Adapter, template helm.Template) (bool, error) {
	job, err := gitlabctl.SelfSignedCertsJob(adapter, template)
	if err != nil {
		return false, err
	}

	if job == nil {
		r.Log.V(2).Info("self-signed certificates Job skipped, not needed per configuration", "gitlab", adapter.Name())

		return true, nil
	}

	if err := r.createOrPatch(ctx, job, adapter); err != nil {
		return false, err
	}

	return r.jobFinished(ctx, adapter, job)
}
