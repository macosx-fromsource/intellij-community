package controllers

import (
	"context"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
)

//nolint:nestif,gocognit
func (r *GitLabReconciler) reconcileNonZeroDowntimeUpgrade(ctx context.Context, adapter gitlab.Adapter, template helm.Template, log logr.Logger) (ctrl.Result, error) {
	if adapter.WantsComponent(component.Migrations) {
		if adapter.WantsComponent(component.Webservice) || adapter.WantsComponent(component.Sidekiq) {
			// If upgrading with Migrations enabled and Webservice and/or Sidekiq enabled,
			// scale them down before running migrations, then restore them after.
			log.Info("ensuring migrations Job has finished (upgrade with downtime)")

			job, err := gitlabctl.MigrationsJob(adapter, template)
			if err != nil {
				return requeue(err)
			}

			exists, err := r.jobExists(ctx, job)
			if err != nil {
				return requeue(err)
			}

			// Scale Webservice and Sidekiq down before running migrations.
			// Only scale them down once (before the migrations Job is created),
			// to avoid scale-down/scale-up loop.
			if !exists {
				log.Info("migrations Job does not exist")

				log.Info("scaling down Webservice and/or Sidekiq")

				if err := r.scaleDownWebserviceAndSidekiqIfEnabled(ctx, adapter, template); err != nil {
					return requeue(err)
				}

				if !r.webserviceAndSidekiqScaledDownIfEnabled(ctx, adapter, template) {
					log.Info("Webservice and/or Sidekiq not yet fully scaled down")
					return requeueWithDefaultDelay()
				}
			}

			finished, err := r.runAllMigrations(ctx, adapter, template)
			if err != nil {
				return requeue(err)
			}

			if !finished {
				log.Info("migrations Job not yet finished")
				return requeueWithDefaultDelay()
			}

			log.Info("ensuring Webservice and/or Sidekiq are reconciled with new version")

			if err := r.reconcileWebserviceAndSidekiqIfEnabled(ctx, adapter, template, false); err != nil {
				return requeue(err)
			}

			log.Info("restoring Webservice and/or Sidekiq replica counts")

			if err := r.restoreWebserviceAndSidekiqReplicasIfEnabled(ctx, adapter, template); err != nil {
				return requeue(err)
			}

			if !r.webserviceAndSidekiqHaveReadyReplicaIfEnabled(ctx, adapter, template) {
				log.Info("Webservice and/or Sidekiq do not yet have a ready replica")
				return requeueWithDefaultDelay()
			}
		} else {
			// If upgrading with Migrations enabled but neither Webservice nor Sidekiq are enabled,
			// then just run all migrations.
			log.Info("ensuring migrations Job has finished")

			finished, err := r.runAllMigrations(ctx, adapter, template)
			if err != nil {
				return requeue(err)
			}

			if !finished {
				log.Info("migrations Job not yet finished")
				return requeueWithDefaultDelay()
			}
		}
	} else {
		// If upgrading with Migrations disabled, then just reconcile enabled Deployments.
		log.Info("ensuring Webservice and/or Sidekiq are reconciled if enabled")

		if err := r.reconcileWebserviceAndSidekiqIfEnabled(ctx, adapter, template, false); err != nil {
			return requeue(err)
		}
	}

	return ctrl.Result{}, nil
}
