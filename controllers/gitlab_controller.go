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
	"slices"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	envoy "github.com/envoyproxy/gateway/api/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayalpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

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
	maxKeyLength        = 63
)

// GitLabReconciler reconciles a GitLab object.
type GitLabReconciler struct {
	client.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch;update

// Reconcile triggers when an event occurs on the watched resource.
//
//nolint:gocognit,gocyclo,nestif // The complexity of this method will be addressed in #260.
func (r *GitLabReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gitlab", req.NamespacedName)

	log.Info("reconciling GitLab")

	rtCtx := rt.NewContext(ctx,
		rt.WithLogger(log),
		rt.WithClient(r.Client),
		rt.WithEventRecorder(r.Recorder))

	gitlab := &apiv1beta1.GitLab{}
	if err := r.Get(ctx, req.NamespacedName, gitlab); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "GitLab custom resource not found, exiting")
			return doNotRequeue()
		}

		return requeue(err)
	}

	adapter, err := adapter.NewV1Beta1(rtCtx, gitlab)
	if err != nil {
		return requeue(err)
	}

	zduDisabled := apiv1beta1.IsZeroDowntimeUpgradeDisabled(gitlab)

	if zduDisabled && adapter.IsUpgrade() {
		log.Info("zero downtime upgrade is disabled via annotation",
			"annotation", apiv1beta1.DisableZDUAnnotationKey)
	}

	isZeroDowntimeUpgrade := adapter.IsUpgrade() && !zduDisabled
	operation := "install"

	if adapter.IsUpgrade() {
		operation = "upgrade"
	}

	log.Info("GitLab is initializing", "operation", operation, "current version", adapter.CurrentVersion(), "desired version", adapter.DesiredVersion())

	if err := r.setStatusCondition(ctx, adapter, status.ConditionInitialized, false, "GitLab is initializing"); err != nil {
		return requeue(err)
	}

	template, err := gitlabctl.GetTemplate(adapter)
	if err != nil {
		r.Recorder.Eventf(adapter.Origin(), nil, "Warning", "ConfigError", "GetTemplate",
			"Configuration error detected: %v", err)

		log.Error(err, "configuration error detected, check GitLab custom resource events")

		if err := r.setStatusCondition(ctx, adapter, status.ConditionInitialized, false, "There is a configuration error, check GitLab custom resource values"); err != nil {
			return requeue(err)
		}

		return doNotRequeue()
	}

	if err := r.setStatusCondition(ctx, adapter, status.ConditionInitialized, true, "GitLab is initialized"); err != nil {
		return requeue(err)
	}

	if err := r.setStatusCondition(ctx, adapter, status.ConditionAvailable, false, "GitLab is starting but not yet available"); err != nil {
		return requeue(err)
	}

	if err := r.reconcileGatewayApiResources(ctx, adapter, template); err != nil {
		return requeue(err)
	}

	if adapter.WantsComponent(component.NginxIngress) {
		if err := r.reconcileNGINX(ctx, adapter, template, false); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.NginxGeo) {
		if err := r.reconcileNGINX(ctx, adapter, template, true); err != nil {
			return requeue(err)
		}
	}

	finished, err := r.runSharedSecretsJob(ctx, adapter, template)
	if err != nil {
		return requeue(err)
	}

	if !finished {
		log.Info("shared secrets Job not yet finished")
		return requeueWithDefaultDelay()
	}

	finished, err = r.runSelfSignedCertsJob(ctx, adapter, template)
	if err != nil {
		return requeue(err)
	}

	if !finished {
		log.Info("self-signed certificates Job not yet finished")
		return requeueWithDefaultDelay()
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

	if adapter.WantsComponent(component.PostgreSQL) {
		if err := r.reconcilePostgres(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	} else {
		if err := r.validateExternalPostgresConfiguration(ctx, adapter); err != nil {
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

	if adapter.WantsComponent(component.Zoekt) {
		if err := r.reconcileZoekt(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if internal.RequiresCertManagerCertificate(adapter).Any() {
		if err := r.reconcileCertManagerCertificates(ctx, adapter); err != nil {
			return requeue(err)
		}
	}

	ready, serviceName := r.ifCoreServicesReady(ctx, adapter, template)
	if !ready {
		log.Info("core services not ready, waiting and retrying", "service name", serviceName)
		return requeueWithDefaultDelay()
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

	if adapter.WantsComponent(component.GeoLogcursor) {
		if err := r.reconcileGeoLogcursor(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if isZeroDowntimeUpgrade {
		if err := r.setStatusCondition(ctx, adapter, status.ConditionUpgrading, true, fmt.Sprintf("GitLab is upgrading from %s to %s", adapter.CurrentVersion(), adapter.DesiredVersion())); err != nil {
			return requeue(err)
		}

		result, err := r.reconcileZeroDowntimeUpgrade(ctx, adapter, template, log)
		if !result.IsZero() {
			return result, err
		}
	} else if adapter.IsUpgrade() {
		if err := r.setStatusCondition(ctx, adapter, status.ConditionUpgrading, true, fmt.Sprintf("GitLab is upgrading from %s to %s (with downtime)", adapter.CurrentVersion(), adapter.DesiredVersion())); err != nil {
			return requeue(err)
		}

		result, err := r.reconcileNonZeroDowntimeUpgrade(ctx, adapter, template, log)
		if !result.IsZero() {
			return result, err
		}
	} else {
		if err := r.setStatusCondition(ctx, adapter, status.ConditionUpgrading, false, "GitLab is not currently upgrading"); err != nil {
			return requeue(err)
		}

		if adapter.WantsComponent(component.Migrations) {
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

		log.Info("ensuring Webservice and Sidekiq are reconciled if enabled")

		if err := r.reconcileWebserviceAndSidekiqIfEnabled(ctx, adapter, template, false); err != nil {
			return requeue(err)
		}
	}

	if err := r.setupAutoscaling(ctx, adapter, template); err != nil {
		return requeue(err)
	}

	if settings.IsGroupVersionKindSupported("monitoring.coreos.com/v1", "ServiceMonitor") {
		if err := r.reconcileServiceMonitors(ctx, adapter, template); err != nil {
			return requeue(err)
		}

		if adapter.WantsComponent(component.PostgreSQL) {
			if err := r.createOrPatch(ctx, internal.PostgresqlServiceMonitor(adapter), adapter); err != nil {
				return requeue(err)
			}
		}
	}

	if settings.IsGroupVersionKindSupported("monitoring.coreos.com/v1", "PodMonitor") {
		if err := r.reconcilePodMonitors(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	if adapter.WantsComponent(component.Prometheus) {
		if err := r.reconcilePrometheus(ctx, adapter, template); err != nil {
			return requeue(err)
		}
	}

	currentManagedObjects, err := adapter.CurrentObjects(rtCtx)
	if err != nil {
		log.Error(err, "unable to discover the managed resources for GitLab instance")
		return requeue(err)
	}

	targetManagedObjects := adapter.TargetObjects()
	deletePropagation := metav1.DeletePropagationBackground

	for _, obj := range currentManagedObjects.Difference(targetManagedObjects) {
		objLog := log.WithValues("kind", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName())

		canBeDeleted, err := isSafeToDelete(rtCtx, obj)
		if err != nil {
			objLog.V(2).Error(err, "unable to determine if it is safe to delete the object")
			continue
		}

		if !canBeDeleted {
			objLog.Info("unable to safely delete the object, skipping its deletion")
			continue
		}

		if err := r.Delete(ctx, obj, &client.DeleteOptions{PropagationPolicy: &deletePropagation}); err == nil {
			objLog.Info("object deleted")
		} else if errors.IsNotFound(err) {
			objLog.V(2).Info("object not found, skipping its deletion")
		} else {
			objLog.V(2).Error(err, "unable to delete the object")
		}
	}

	return r.reconcileGitLabStatus(ctx, adapter, template, log)
}

func isSafeToDelete(ctx context.Context, obj client.Object) (bool, error) {
	c := rt.ClientFromContext(ctx)
	if c == nil {
		// This should not never happen
		panic("Can not extract Client from runtime context")
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	if !slices.Contains([]string{"Job", "CronJob"}, gvk.Kind) {
		return true, nil
	}

	existing := unstructured.Unstructured{}
	existing.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &existing); err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	}

	if gvk.Kind == "Job" {
		numActive, _, err := unstructured.NestedInt64(existing.Object, "status", "active")
		if err != nil {
			return false, fmt.Errorf("can not find number of active Pods for Job %s", obj.GetName())
		}

		return numActive == 0, nil
	}

	if gvk.Kind == "CronJob" {
		lstActive, _, err := unstructured.NestedSlice(existing.Object, "status", "active")
		if err != nil {
			return false, fmt.Errorf("can not find list of active Pods for CronJob %s", obj.GetName())
		}

		return len(lstActive) == 0, nil
	}

	return true, nil
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
		Owns(&appsv1.DaemonSet{}).
		Owns(&batchv1.Job{}).
		Owns(&networkingv1.Ingress{}).
		WithEventFilter(predicate.GenerationChangedPredicate{})

	if settings.IsGroupVersionKindSupported("batch/v1", "CronJob") {
		r.Log.Info("using batch/v1 for CronJob")
		builder.Owns(&batchv1.CronJob{})
	}

	if settings.IsGroupVersionKindSupported("batch/v1beta1", "CronJob") {
		r.Log.Info("using batch/v1beta1 for CronJob")
		builder.Owns(&batchv1beta1.CronJob{})
	}

	if settings.IsGroupVersionKindSupported("monitoring.coreos.com/v1", "ServiceMonitor") {
		r.Log.Info("using monitoring.coreos.com/v1 for ServiceMonitor")
		builder.Owns(&monitoringv1.ServiceMonitor{})
	}

	if settings.IsGroupVersionKindSupported("monitoring.coreos.com/v1", "PodMonitor") {
		r.Log.Info("using monitoring.coreos.com/v1 for PodMonitor")
		builder.Owns(&monitoringv1.PodMonitor{})
	}

	if settings.IsGroupVersionKindSupported("monitoring.coreos.com/v1", "Prometheus") {
		r.Log.Info("using monitoring.coreos.com/v1/Prometheus")
		builder.Owns(&monitoringv1.Prometheus{})
	}

	if settings.IsGroupVersionSupported("cert-manager.io", "v1") {
		r.Log.Info("using cert-manager.io/v1")
		builder.
			Owns(&certmanagerv1.Issuer{}).
			Owns(&certmanagerv1.Certificate{})
	}

	if settings.IsGroupVersionSupported("gateway.networking.k8s.io", "v1") {
		r.Log.Info("using gateway.networking.k8s.io/v1")
		builder.
			Owns(&gatewayv1.Gateway{}).
			Owns(&gatewayv1.GatewayClass{}).
			Owns(&gatewayv1.HTTPRoute{})
	}

	if settings.IsGroupVersionSupported("gateway.networking.k8s.io", "v1alpha2") {
		r.Log.Info("using gateway.networking.k8s.io/v1alpha2")
		builder.Owns(&gatewayalpha2.TCPRoute{})
	}

	if settings.IsGroupVersionSupported("gateway.envoyproxy.io", "v1alpha1") {
		r.Log.Info("using gateway.envoyproxy.io/v1alpha1")
		builder.
			Owns(&envoy.EnvoyPatchPolicy{}).
			Owns(&envoy.SecurityPolicy{}).
			Owns(&envoy.ClientTrafficPolicy{}).
			Owns(&envoy.EnvoyProxy{})
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

	logger.V(2).Info("checking the status of Job")

	lookup, err := r.lookupJob(ctx, job)
	if err != nil {
		logger.V(2).Info("failed to check the status of Job", "error", err)
		return false, err
	}

	if lookup.Status.Succeeded > 0 {
		logger.V(2).Info("Job succeeded")
		return true, nil
	}

	if lookup.Status.Failed > 0 {
		err := errors.NewInternalError(fmt.Errorf("job %s has failed", lookup))
		logger.Error(err, "Job failed")

		return true, err
	}

	return false, nil
}

func (r *GitLabReconciler) lookupJob(ctx context.Context, job client.Object) (*batchv1.Job, error) {
	lookupKey := types.NamespacedName{
		Name:      job.GetName(),
		Namespace: job.GetNamespace(),
	}

	lookup := &batchv1.Job{}

	if err := r.Get(ctx, lookupKey, lookup); err != nil {
		return nil, err
	}

	return lookup, nil
}

func (r *GitLabReconciler) jobExists(ctx context.Context, job client.Object) (bool, error) {
	_, err := r.lookupJob(ctx, job)
	if err == nil {
		return true, nil
	}

	if errors.IsNotFound(err) {
		return false, nil
	}

	return false, err
}

func (r *GitLabReconciler) reconcileServiceMonitors(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, sm := range gitlabctl.WantedServiceMonitors(adapter, template) {
		if err := r.createOrPatch(ctx, sm, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcilePodMonitors(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, pm := range gitlabctl.WantedPodMonitors(adapter, template) {
		if err := r.createOrPatch(ctx, pm, adapter); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileGatewayApiResources(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	for _, o := range gitlabctl.WantedGatewayApiResources(template, adapter) {
		if err := r.createOrPatch(ctx, o, adapter); err != nil {
			return err
		}
	}

	return nil
}

// The boolean return parameter is unused at the moment, but may be useful in the future.
func (r *GitLabReconciler) createOrPatch(ctx context.Context, templateObject client.Object, adapter gitlab.Adapter) error {
	if templateObject == nil {
		r.Log.Info("controller unable to delete managed resources, this is a known issue",
			"gitlab", adapter.Name())

		return nil
	}
	// NOTE: This keeps track of the managed objects. It will be removed once we
	//       migrate to the new framework.
	if err := adapter.PopulateManagedObjects(templateObject); err != nil {
		return err
	}

	key := client.ObjectKeyFromObject(templateObject)

	logger := r.Log.WithValues(
		"gitlab", adapter.Name(),
		"type", fmt.Sprintf("%T", templateObject),
		"reference", key)

	logger.V(2).Info("setting controller reference")

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
		logger.V(1).Info("create or patch outcome is changed", "outcome", outcome)
	}

	return nil
}

func (r *GitLabReconciler) reconcileIngress(ctx context.Context, templateObject client.Object, adapter gitlab.Adapter) error {
	if templateObject == nil {
		r.Log.V(2).Info("controller received a nil templateObject",
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
			logger.V(1).Info("creating Ingress", "Ingress", ingress.Name)
			return r.createOrPatch(ctx, ingress, adapter)
		}

		return err
	}

	// If resource is an Ingress and has an ACME challenge path, skip the patch.
	// This ensures that CertManager can add a path to existing ingresses for the ACME challenge without
	// the Operator immediately removing it before the challenge can be completed.
	doPatch := true
	regex := regexp.MustCompile("/.well-known/acme-challenge/+")

	for _, path := range found.Spec.Rules[0].HTTP.Paths {
		if regex.MatchString(path.Path) {
			logger.V(1).Info("Ingress contains ACME challenge path, skipping patch for now", "Ingress", found.Name)

			doPatch = false
		}
	}

	if doPatch {
		if err := r.createOrPatch(ctx, ingress, adapter); err != nil {
			return err
		}
	} else {
		// Always populate Ingress to make sure it does not get deleted.
		if err := adapter.PopulateManagedObjects(ingress); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) reconcileCertManagerCertificates(ctx context.Context, adapter gitlab.Adapter) error {
	if issuer := internal.CertificateIngressIssuer(adapter); issuer != nil {
		if err := r.createOrPatch(ctx, issuer, adapter); err != nil {
			return err
		}
	}

	if issuer := internal.CertificateGatewayIssuer(adapter); issuer != nil {
		if err := r.createOrPatch(ctx, issuer, adapter); err != nil {
			return err
		}
	}

	return nil
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
	slices := &discoveryv1.EndpointSliceList{}

	err := r.List(ctx, slices,
		client.MatchingLabels(map[string]string{
			discoveryv1.LabelServiceName: service,
		}),
		client.InNamespace(adapter.Name().Namespace),
	)
	if err != nil {
		r.Log.Error(err, "unable to list EndpointSlices for Service", "service", service, "gitlab", adapter.Name())

		return false
	} else if len(slices.Items) == 0 {
		r.Log.V(1).Info("no EndpointSlices exist for the Service", "service", service, "gitlab", adapter.Name())

		return false
	}

	return true
}

func (r *GitLabReconciler) ifCoreServicesReady(ctx context.Context, adapter gitlab.Adapter, template helm.Template) (bool, string) {
	serviceNames := []string{}

	if adapter.WantsComponent(component.PostgreSQL) {
		serviceNames = append(serviceNames, gitlabctl.PostgresService(adapter, template).GetName())
	}

	if adapter.WantsComponent(component.Redis) {
		serviceNames = append(serviceNames, gitlabctl.RedisMasterService(adapter, template).GetName())
	}

	if adapter.WantsComponent(component.Gitaly) {
		if !adapter.WantsComponent(component.Praefect) || !adapter.WantsFeature(feature.ReplaceGitalyWithPraefect) {
			serviceNames = append(serviceNames, gitlabctl.GitalyService(template).GetName())
		}
	}

	if adapter.WantsComponent(component.Praefect) {
		serviceNames = append(serviceNames, gitlabctl.PraefectService(template).GetName())

		if adapter.WantsComponent(component.Gitaly) {
			for _, gitalyPraefectService := range gitlabctl.GitalyPraefectServices(template) {
				serviceNames = append(serviceNames, gitalyPraefectService.GetName())
			}
		}
	}

	for _, serviceName := range serviceNames {
		if !r.isEndpointReady(ctx, serviceName, adapter) {
			return false, serviceName
		}
	}

	return true, ""
}

// If a Deployment has an HPA attached to it consult its Status to set the replica count.
func (r *GitLabReconciler) setDeploymentReplica(ctx context.Context, obj client.Object) error {
	deployment, err := internal.AsDeployment(obj)
	if err != nil {
		return err
	}

	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
		return nil
	}

	// Finds the Deployment's HPA using the Deployment's name (since they are defined the same way in the Helm chart).
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, hpa); err == nil {
		// Replica count is controlled by HPA and should not be patched by the GitLab controller.
		r.Log.V(1).Info("not setting replicas for Deployment controlled by HPA",
			"deployment", types.NamespacedName{
				Namespace: deployment.Namespace,
				Name:      deployment.Name,
			})

		deployment.Spec.Replicas = nil

		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	// Find the Deployment current replica count. If it's scaled to zero, do not override it.
	liveDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, liveDeployment); err == nil {
		if liveDeployment.Spec.Replicas != nil && *liveDeployment.Spec.Replicas == 0 {
			r.Log.V(1).Info("Deployment is scaled down, not overriding the replica count",
				"deployment", types.NamespacedName{
					Namespace: deployment.Namespace,
					Name:      deployment.Name,
				})

			*deployment.Spec.Replicas = 0

			return nil
		}
	} else if !errors.IsNotFound(err) {
		return err
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

		if template.Annotations == nil {
			template.Annotations = map[string]string{}
		}

		key := fmt.Sprintf("checksum/secret-%s", secretName)

		truncatedKey, err := internal.Truncate(key, maxKeyLength)
		if err != nil {
			return err
		}

		template.Annotations[truncatedKey] = hash
	}

	return nil
}

func (r *GitLabReconciler) ensureSecret(ctx context.Context, adapter gitlab.Adapter, secretName string) error {
	secret := &corev1.Secret{}
	lookupKey := types.NamespacedName{Name: secretName, Namespace: adapter.Name().Namespace}

	err := r.Get(ctx, lookupKey, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("secret '%s' not found", lookupKey)
		}

		return err
	}

	return nil
}

func doNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func requeue(err error) (ctrl.Result, error) {
	return ctrl.Result{Requeue: true}, err
}

func requeueWithDelay(delay time.Duration) (ctrl.Result, error) {
	if delay == 0 {
		delay = defaultRequeueDelay
	}

	return ctrl.Result{RequeueAfter: delay}, nil
}

func requeueWithDefaultDelay() (ctrl.Result, error) {
	return requeueWithDelay(defaultRequeueDelay)
}
