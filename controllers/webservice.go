package controllers

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/internal"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) reconcileWebserviceExceptDeployments(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if err := r.reconcileWebserviceConfigMaps(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileWebserviceServices(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileWebserviceIngresses(ctx, adapter, template); err != nil {
		return err
	}

	if err := r.reconcileWebserviceRoute(ctx, adapter, template); err != nil {
		return err
	}

	return nil
}

func (r *GitLabReconciler) reconcileWebserviceConfigMaps(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, cm := range gitlabctl.WebserviceConfigMaps(template) {
		if err := r.createOrPatch(ctx, cm, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileWebserviceServices(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, svc := range gitlabctl.WebserviceServices(template) {
		if err := r.createOrPatch(ctx, svc, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileWebserviceDeployments(ctx context.Context, adapter gitlab.Adapter, template helm.Template, pause bool) error {
	logger := r.Log.WithValues("gitlab", adapter.Name())

	webservices := gitlabctl.WebserviceDeployments(template)

	if internal.IsOpenshift() && len(webservices) > 1 {
		logger.V(2).Info("Multiple Webservice Ingresses detected, which is not supported on OpenShift when using NGINX Ingress Operator. See https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/160")
	}

	for _, webservice := range webservices {
		ws := webservice.DeepCopyObject().(client.Object)

		if err := r.setDeploymentReplica(ctx, ws); err != nil {
			return err
		}

		if err := r.annotateSecretsChecksum(ctx, adapter, ws); err != nil {
			return err
		}

		if err := internal.ToggleDeploymentPause(ws, pause); err != nil {
			return err
		}

		if err := r.createOrPatch(ctx, ws, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileWebserviceIngresses(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, ingress := range gitlabctl.WebserviceIngresses(template) {
		if err := r.reconcileIngress(ctx, ingress, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileWebserviceRoute(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if route := gitlabctl.WebserviceRoute(template); route != nil {
		if err := r.createOrPatch(ctx, route, adapter); err != nil {
			return err
		}
	}

	return nil
}
