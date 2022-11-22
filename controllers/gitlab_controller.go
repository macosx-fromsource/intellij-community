/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"

	apiv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/internal"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/adapter"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	feature "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/features"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/status"
	rt "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/runtime"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube/apply"
)

const (
	defaultRequeueDelay = 10 * time.Second
)

// GitLabReconciler reconciles a GitLab object.
type GitLabReconciler struct {
	client.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

// Reconcile triggers when an event occurs on the watched resource.
//
//nolint:gocognit,gocyclo,nestif // The complexity of this method will be addressed in #260.
func (r *GitLabReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gitlab", req.NamespacedName)

	log.Info("Reconciling GitLab")

	rtCtx := rt.NewContext(ctx,
		rt.WithLogger(log),
		rt.WithClient(r.Client),
		rt.WithEventRecorder(r.Recorder))

	gitlab := &apiv1beta1.GitLab{}
	if err := r.Get(ctx, req.NamespacedName, gitlab); err != nil {
		if errors.IsNotFound(err) {
			return doNotRequeue()
		}

		// could not get GitLab resource
		return requeue(err)
	}

	adapter, err := adapter.NewV1Beta1(rtCtx, gitlab)
	if err != nil {
		return requeue(err)
	}

	isUpgrade := adapter.IsUpgrade()
	log.V(1).Info("version information", "upgrade", isUpgrade, "current version", adapter.CurrentVersion(), "desired version", adapter.DesiredVersion())

	if err := r.setStatusCondition(ctx, adapter, status.ConditionInitialized, false, "GitLab is initializing"); err != nil {
		return requeue(err)
	}

	template, err := gitlabctl.GetTemplate(adapter)
	if err != nil {
		r.Recorder.Event(adapter.Origin(), "Warning", "ConfigError",
			fmt.Sprintf("Configuration error detected: %v", err))
		return doNotRequeue() // prevent further reconcile loops
	}

	if err := r.setStatusCondition(ctx, adapter, status.ConditionInitialized, true, "GitLab is initialized"); err != nil {
		return requeue(err)
	}

	if err := r.setStatusCondition(ctx, adapter, status.ConditionAvailable, false, "GitLab is starting but not yet available"); err != nil {
		return requeue(err)
	}

	if adapter.WantsComponent(component.NginxIngress) {
		if err := r.reconcileNGINX(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	finished, err := r.runSharedSecretsJob(ctx, adapter, template)
	if err != nil {
		return requeue(err)
	}

	if !finished {
		return requeueWithDelay()
	}

	finished, err = r.runSelfSignedCertsJob(ctx, adapter, template)
	if err != nil {
		return requeue(err)
	}

	if !finished {
		return requeueWithDelay()
	}

	if adapter.WantsComponent(component.PostgreSQL) {
		if err := r.reconcilePostgres(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	} else {
		if err := r.validateExternalPostgresConfiguration(ctx, adapter); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Redis) {
		if err := r.reconcileRedis(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	} else {
		if err := r.validateExternalRedisConfiguration(ctx, adapter); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Gitaly) {
		if !adapter.WantsComponent(component.Praefect) || !adapter.WantsFeature(feature.ReplaceGitalyWithPraefect) {
			if err := r.reconcileGitaly(ctx, adapter, template); err != nil {
				return requeue(err)
			}
		}
	}

	if adapter.WantsComponent(component.Praefect) {
		if err := r.reconcilePraefect(ctx, adapter, template); err != nil {
			return requeue(err)
		}

		if adapter.WantsComponent(component.Gitaly) {
			if err := r.reconcileGitalyPraefect(ctx, adapter, template); err != nil {
				return requeue(err)
			}
		}
	}

	if adapter.WantsComponent(component.MinIO) {
		if err := r.reconcileMinioInstance(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Mailroom) {
		if err := r.reconcileMailroom(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Spamcheck) {
		if err := r.reconcileSpamcheck(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if internal.RequiresCertManagerCertificate(adapter).Any() {
		if err := r.reconcileCertManagerCertificates(ctx, adapter); err != nil {
			return requeue(err)
		}
	}

	if !r.ifCoreServicesReady(ctx, adapter, template) {
		log.Info("Core services are not ready. Waiting and retrying", "interval", defaultRequeueDelay)
		return requeueWithDelay()
	}

	if adapter.WantsComponent(component.GitLabShell) {
		if err := r.reconcileGitLabShell(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Registry) {
		if err := r.reconcileRegistry(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Toolbox) {
		if err := r.reconcileToolbox(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.GitLabExporter) {
		if err := r.reconcileGitLabExporter(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.GitLabPages) {
		if err := r.reconcilePages(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.GitLabKAS) {
		if err := r.reconcileKas(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Migrations) {
		if err := r.reconcileMigrationsConfigMap(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if err := r.reconcileSidekiqConfigMaps(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Webservice) {
		if err := r.reconcileWebserviceExceptDeployments(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if isUpgrade {
		if err := r.setStatusCondition(ctx, adapter, status.ConditionUpgrading, true, fmt.Sprintf("GitLab is upgrading from %s to %s", adapter.CurrentVersion(), adapter.DesiredVersion())); err != nil {
			return requeue(err)
		}

		if adapter.WantsComponent(component.Migrations) {
			if adapter.WantsComponent(component.Webservice) || adapter.WantsComponent(component.Sidekiq) {
				// If upgrading with Migrations enabled and Webservice and/or Sidekiq enabled,
				// then follow the traditional upgrade logic.
				if err := r.reconcileWebserviceAndSidekiqIfEnabled(ctx, adapter, template, true, log); err != nil {
					return requeue(err)
				}

				log.Info("reconciling pre migrations")

				finished, err := r.runPreMigrations(ctx, adapter, template)
				if err != nil {
					return requeue(err)
				}

				if !finished {
					return requeueWithDelay()
				}

				if err := r.unpauseWebserviceAndSidekiqIfEnabled(ctx, adapter, template, log); err != nil {
					return requeueWithDelay()
				}

				if err := r.webserviceAndSidekiqRunningIfEnabled(ctx, adapter, template, log); err != nil {
					return requeueWithDelay()
				}

				log.Info("reconciling post migrations")

				finished, err = r.runAllMigrations(ctx, adapter, template)
				if err != nil {
					return requeue(err)
				}

				if !finished {
					return requeueWithDelay()
				}

				if err := r.rollingUpdateWebserviceAndSidekiqIfEnabled(ctx, adapter, template, log); err != nil {
					return requeue(err)
				}
			} else {
				// If upgrading with Migrations enabled but neither Webservice nor Sidekiq are enabled,
				// then just run all migrations.
				log.Info("running all migrations")

				finished, err := r.runAllMigrations(ctx, adapter, template)
				if err != nil {
					return requeue(err)
				}

				if !finished {
					return requeueWithDelay()
				}
			}
		} else {
			// If upgrading with Migrations disabled, then just reconcile enabled Deployments.
			if err := r.reconcileWebserviceAndSidekiqIfEnabled(ctx, adapter, template, false, log); err != nil {
				return requeue(err)
			}
		}
	} else {
		// If not upgrading, then run all migrations (if enabled) and reconcile enabled Deployments.
		if err := r.setStatusCondition(ctx, adapter, status.ConditionUpgrading, false, "GitLab is not currently upgrading"); err != nil {
			return requeue(err)
		}

		if adapter.WantsComponent(component.Migrations) {
			log.Info("running all migrations")

			finished, err := r.runAllMigrations(ctx, adapter, template)
			if err != nil {
				return requeue(err)
			}

			if !finished {
				return requeueWithDelay()
			}
		}

		if err := r.reconcileWebserviceAndSidekiqIfEnabled(ctx, adapter, template, false, log); err != nil {
			return requeue(err)
		}
	}

	if err := r.setupAutoscaling(ctx, adapter, template); err != nil {
		return requeue(err)
	}

	if settings.IsGroupVersionSupported("monitoring.coreos.com", "v1") {
		// Deploy a prometheus service monitor
		if err := r.reconcileServiceMonitor(ctx, adapter); err != nil {
			return requeue(err)
		}
	}

	result, err := r.reconcileGitLabStatus(ctx, adapter, template)

	return result, err
}

// SetupWithManager configures the custom resource watched resources.
func (r *GitLabReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta1.GitLab{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&batchv1.Job{}).
		Owns(&networkingv1.Ingress{}).
		WithEventFilter(predicate.GenerationChangedPredicate{})

	if settings.IsGroupVersionKindSupported("batch/v1", "CronJob") {
		r.Log.Info("Using batch/v1 for CronJob")
		builder.Owns(&batchv1.CronJob{})
	}

	if settings.IsGroupVersionKindSupported("batch/v1beta1", "CronJob") {
		r.Log.Info("Using batch/v1beta1 for CronJob")
		builder.Owns(&batchv1beta1.CronJob{})
	}

	if settings.IsGroupVersionSupported("monitoring.coreos.com", "v1") {
		r.Log.Info("Using monitoring.coreos.com/v1")
		builder.Owns(&monitoringv1.ServiceMonitor{})
	}

	if settings.IsGroupVersionSupported("cert-manager.io", "v1") {
		r.Log.Info("Using cert-manager.io/v1")
		builder.
			Owns(&certmanagerv1.Issuer{}).
			Owns(&certmanagerv1.Certificate{})
	}

	return builder.Complete(r)
}

// jobFinished checks the status of a specified Job.
// - Returns `true` and `nil` if the Job is finished and has a status of Succeeded.
// - Returns `true` and an error if the Job is finished and has a status of Failed.
// - Returns `false` and an error if the Job Status cannot be found.
// - Returns `false` and `nil` in any other case (meaning the Job is still running with no errors yet).
func (r *GitLabReconciler) jobFinished(ctx context.Context, adapter gitlab.Adapter, job client.Object) (bool, error) {
	logger := r.Log.WithValues("gitlab", adapter.Name(), "job", job.GetName(), "namespace", job.GetNamespace())

	logger.V(2).Info("Checking the status of Job")

	lookupKey := types.NamespacedName{
		Name:      job.GetName(),
		Namespace: job.GetNamespace(),
	}

	lookupVal := &batchv1.Job{}

	if err := r.Get(ctx, lookupKey, lookupVal); err != nil {
		logger.V(2).Info("failed to check the status of Job", "error", err)
		return false, err
	}

	if lookupVal.Status.Succeeded > 0 {
		logger.V(2).Info("Job succeeded")
		return true, nil
	}

	if lookupVal.Status.Failed > 0 {
		err := errors.NewInternalError(fmt.Errorf("job %s has failed", lookupKey))
		logger.Error(err, "Job failed")

		return true, err
	}

	return false, nil
}

func (r *GitLabReconciler) reconcileServiceMonitor(ctx context.Context, adapter gitlab.Adapter) error {
	var servicemonitors []*monitoringv1.ServiceMonitor

	gitaly := internal.GitalyServiceMonitor(adapter)

	gitlab := internal.ExporterServiceMonitor(adapter)

	workhorse := internal.WebserviceServiceMonitor(adapter)

	servicemonitors = append(servicemonitors,
		gitlab,
		gitaly,
		workhorse,
	)

	if adapter.WantsComponent(component.Redis) {
		redis := internal.RedisServiceMonitor(adapter)
		servicemonitors = append(servicemonitors, redis)
	}

	if adapter.WantsComponent(component.PostgreSQL) {
		postgres := internal.PostgresqlServiceMonitor(adapter)
		servicemonitors = append(servicemonitors, postgres)
	}

	for _, sm := range servicemonitors {
		if err := r.createOrPatch(ctx, sm, adapter); err != nil {
			return err
		}
	}

	if adapter.WantsComponent(component.Prometheus) {
		service := internal.ExposePrometheusCluster(adapter)
		if err := r.createOrPatch(ctx, service, adapter); err != nil {
			return err
		}

		prometheus := internal.PrometheusCluster(adapter)
		if err := r.createOrPatch(ctx, prometheus, adapter); err != nil {
			return err
		}
	}

	return nil
}

// The boolean return parameter is unused at the moment, but may be useful in the future.
func (r *GitLabReconciler) createOrPatch(ctx context.Context, templateObject client.Object, adapter gitlab.Adapter) error {
	if templateObject == nil {
		r.Log.Info("Controller is not able to delete managed resources. This is a known issue",
			"gitlab", adapter.Name())
		return nil
	}

	key := client.ObjectKeyFromObject(templateObject)

	logger := r.Log.WithValues(
		"gitlab", adapter.Name(),
		"type", fmt.Sprintf("%T", templateObject),
		"reference", key)

	logger.V(2).Info("Setting controller reference")

	obj := templateObject.DeepCopyObject().(client.Object)

	if err := controllerutil.SetControllerReference(adapter.Origin(), obj, r.Scheme); err != nil {
		return err
	}

	outcome, err := kube.ApplyObject(obj, apply.WithContext(ctx),
		apply.WithClient(r.Client), apply.WithLogger(logger))

	if err != nil {
		return err
	}

	if outcome != kube.ObjectUnchanged {
		logger.V(1).Info("CreateOrPatch", "outcome", outcome)
	}

	return nil
}

func (r *GitLabReconciler) createOrUpdate(ctx context.Context, templateObject client.Object, adapter gitlab.Adapter) (bool, bool, error) {
	if templateObject == nil {
		r.Log.Info("Controller is not able to delete managed resources. This is a known issue",
			"gitlab", adapter.Name())
		return false, false, nil
	}

	key := client.ObjectKeyFromObject(templateObject)

	logger := r.Log.WithValues(
		"gitlab", adapter.Name(),
		"type", fmt.Sprintf("%T", templateObject),
		"reference", key)

	logger.V(2).Info("Setting controller reference")

	if err := controllerutil.SetControllerReference(adapter.Origin(), templateObject, r.Scheme); err != nil {
		return false, false, err
	}

	existing := templateObject.DeepCopyObject().(client.Object)

	if err := r.Get(ctx, key, existing); err != nil {
		if !errors.IsNotFound(err) {
			return false, false, err
		}

		if err := r.Create(ctx, existing); err != nil {
			return false, false, err
		}

		logger.V(1).Info("createOrUpdate result", "result", "created")

		return true, false, nil
	}

	templateObject.SetResourceVersion(existing.GetResourceVersion())

	if err := r.Update(ctx, templateObject); err != nil {
		logger.Error(err, "unable to update object", "object", templateObject)
		return false, false, err
	}

	logger.V(1).Info("createOrUpdate result", "result", "updated")

	return false, true, nil
}

func (r *GitLabReconciler) reconcileIngress(ctx context.Context, templateObject client.Object, adapter gitlab.Adapter) error {
	if templateObject == nil {
		r.Log.V(2).Info("Controller received a nil templateObject",
			"type", "Ingress",
			"gitlab", adapter.Name())

		return nil
	}

	ingress, err := internal.AsIngress(templateObject)
	if err != nil {
		return err
	}

	logger := r.Log.WithValues("gitlab", adapter.Name())

	found := &networkingv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: adapter.Name().Namespace}, found)

	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("creating ingress", "ingress", ingress.Name)
			return r.Create(ctx, ingress)
		}

		return err
	}

	// If resource is an Ingress and has an ACME challenge path, skip the patch.
	// This ensures that CertManager can add a path to existing ingresses for the ACME challenge without
	// the Operator immediately removing it before the challenge can be completed.
	doPatch := true
	regex := regexp.MustCompile("/.well-known/acme-challenge/+")

	for _, path := range found.Spec.Rules[0].IngressRuleValue.HTTP.Paths {
		if regex.MatchString(path.Path) {
			logger.V(1).Info("ingress contains ACME challenge path, skipping patch for now", "ingress", found.Name)

			doPatch = false
		}
	}

	if doPatch {
		if err := r.createOrPatch(ctx, ingress, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileCertManagerCertificates(ctx context.Context, adapter gitlab.Adapter) error {
	issuer := internal.CertificateIssuer(adapter)

	_, _, err := r.createOrUpdate(ctx, issuer, adapter)

	return err
}

func (r *GitLabReconciler) setupAutoscaling(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, hpa := range template.Query().ObjectsByKind(gitlabctl.HorizontalPodAutoscalerKind) {
		if err := r.createOrPatch(ctx, hpa, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) isEndpointReady(ctx context.Context, service string, adapter gitlab.Adapter) bool {
	var addresses []corev1.EndpointAddress

	ep := &corev1.Endpoints{}
	err := r.Get(ctx, types.NamespacedName{Name: service, Namespace: adapter.Name().Namespace}, ep)

	if err != nil && errors.IsNotFound(err) {
		return false
	}

	for _, subset := range ep.Subsets {
		addresses = append(addresses, subset.Addresses...)
	}

	return len(addresses) > 0
}

func (r *GitLabReconciler) ifCoreServicesReady(ctx context.Context, adapter gitlab.Adapter, template helm.Template) bool {
	if adapter.WantsComponent(component.PostgreSQL) {
		if !r.isEndpointReady(ctx, adapter.ReleaseName()+"-postgresql", adapter) {
			return false
		}
	}

	if adapter.WantsComponent(component.Redis) {
		if !r.isEndpointReady(ctx, adapter.ReleaseName()+"-redis-master", adapter) {
			return false
		}
	}

	if adapter.WantsComponent(component.Gitaly) {
		if !adapter.WantsComponent(component.Praefect) || !adapter.WantsFeature(feature.ReplaceGitalyWithPraefect) {
			if !r.isEndpointReady(ctx, gitlabctl.GitalyService(template).GetName(), adapter) {
				return false
			}
		}
	}

	if adapter.WantsComponent(component.Praefect) {
		if !r.isEndpointReady(ctx, gitlabctl.PraefectService(template).GetName(), adapter) {
			return false
		}

		if adapter.WantsComponent(component.Gitaly) {
			for _, gitalyPraefectService := range gitlabctl.GitalyPraefectServices(template) {
				if !r.isEndpointReady(ctx, gitalyPraefectService.GetName(), adapter) {
					return false
				}
			}
		}
	}

	return true
}

// If a Deployment has an HPA attached to it consult its Status to set the replica count.
func (r *GitLabReconciler) setDeploymentReplica(ctx context.Context, obj client.Object) error {
	deployment, err := internal.AsDeployment(obj)
	if err != nil {
		return err
	}

	// Get the Deployment's HPA so we can check the desired number of replicas.
	// Finds the Deployment's HPA using the Deployment's name (since they are defined the same way in the Helm chart).
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, hpa); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	replicas := hpa.Status.DesiredReplicas
	if replicas == 0 {
		return nil
	}

	if deployment.Spec.Replicas == nil || *(deployment.Spec.Replicas) != replicas {
		r.Log.V(1).Info("Changing replica count of deployment with HPA",
			"deployment", types.NamespacedName{
				Namespace: deployment.Namespace,
				Name:      deployment.Name,
			},
			"replicas", replicas)

		deployment.Spec.Replicas = &replicas
	}

	return nil
}

func (r *GitLabReconciler) annotateSecretsChecksum(ctx context.Context, adapter gitlab.Adapter, obj client.Object) error {
	template, err := internal.GetPodTemplateSpec(obj)
	if err != nil {
		return err
	}

	secretsInfo := internal.PopulateAttachedSecrets(*template)
	for secretName, secretKeys := range secretsInfo {
		secret := &corev1.Secret{}
		lookupKey := types.NamespacedName{Name: secretName, Namespace: adapter.Name().Namespace}

		if err := r.Get(ctx, lookupKey, secret); err != nil {
			if errors.IsNotFound(err) {
				// Skip this Secret. Do not overreact to it being missing.
				continue
			}

			return err
		}

		hash := internal.SecretChecksum(*secret, secretKeys)
		if hash == "" {
			continue
		}

		if template.ObjectMeta.Annotations == nil {
			template.ObjectMeta.Annotations = map[string]string{}
		}

		template.ObjectMeta.Annotations[fmt.Sprintf("checksum/secret-%s", secretName)] = hash
	}

	return nil
}

func (r *GitLabReconciler) ensureSecret(ctx context.Context, adapter gitlab.Adapter, secretName string) error {
	secret := &corev1.Secret{}
	lookupKey := types.NamespacedName{Name: secretName, Namespace: adapter.Name().Namespace}
	err := r.Get(ctx, lookupKey, secret)

	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Secret '%s' not found", lookupKey)
		}

		return err
	}

	return nil
}

func doNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func requeue(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}

func requeueWithDelay() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: defaultRequeueDelay}, nil
}
