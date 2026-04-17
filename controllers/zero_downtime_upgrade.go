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
func (r *GitLabReconciler) reconcileZeroDowntimeUpgrade(ctx context.Context, adapter gitlab.Adapter, template helm.Template, log logr.Logger) (ctrl.Result, error) {
	if adapter.WantsComponent(component.Migrations) {
		if adapter.WantsComponent(component.Webservice) || adapter.WantsComponent(component.Sidekiq) {
			// If upgrading with Migrations enabled and Webservice and/or Sidekiq enabled,
			// then follow the traditional upgrade logic.
			log.Info("ensuring pre-migrations Job has finished")

			job, err := gitlabctl.PreMigrationsJob(adapter, template)
			if err != nil {
				return requeue(err)
			}

			exists, err := r.jobExists(ctx, job)
			if err != nil {
				return requeue(err)
			}

			// Scale Webservice and Sidekiq down before running pre migrations.
			// Only scale them down before once, to avoid pause -> unpause loop.
			if !exists {
				log.Info("pre-migrations Job does not exist")

				log.Info("ensuring Webservice and/or Sidekiq are reconciled")

				if err := r.reconcileWebserviceAndSidekiqIfEnabled(ctx, adapter, template, true); err != nil {
					return requeue(err)
				}
			}

			finished, err := r.runPreMigrations(ctx, adapter, job)
			if err != nil {
				return requeue(err)
			}

			if !finished {
				log.Info("pre-migrations Job not yet finished")
				return requeueWithDefaultDelay()
			}

			log.Info("ensuring Webservice and/or Sidekiq are unpaused")

			if err := r.unpauseWebserviceAndSidekiqIfEnabled(ctx, adapter, template); err != nil {
				return requeue(err)
			}

			log.Info("ensuring Webservice and/or Sidekiq are running")

			if err := r.webserviceAndSidekiqRunningIfEnabled(ctx, adapter, template); err != nil {
				log.Info("Webservice and/or Sidekiq not yet running", "error", err)
				return requeueWithDefaultDelay()
			}

			log.Info("ensuring post-migrations Job has finished")

			finished, err = r.runAllMigrations(ctx, adapter, template)
			if err != nil {
				return requeue(err)
			}

			if !finished {
				log.Info("migrations Job not yet finished")
				return requeueWithDefaultDelay()
			}

			log.Info("ensuring rolling update of Webservice and/or Sidekiq")

			if err := r.rollingUpdateWebserviceAndSidekiqIfEnabled(ctx, adapter, template); err != nil {
				return requeue(err)
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
