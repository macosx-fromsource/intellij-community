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

// Package gitlabcore reconciles the apps.gitlab.com/v2alpha1 GitLabCore
// resource, the resource Bridge configures a GitLab instance through.
//
// It renders the GitLab umbrella chart with internal/render, runs the chart
// hooks with internal/render/hookexec, and applies the rendered objects.
//
// It lives under internal/controller, where the v2 controllers go. The
// deprecated controllers/ and helm/ packages reconcile and render the v1beta1
// GitLab resource and are frozen; the two paths share no code.
//
// The reconciler is alpha and rides with Bridge, so it is gated twice: the
// `bridge` build tag compiles the wiring in (gitlabcore.go / gitlabcore_stub.go
// in the root package), and ENABLE_BRIDGE registers it at runtime. Its
// definition reaches no installation (see ADR 26), so registering the watch
// unconditionally would fail the manager on every cluster that has no
// gitlabcores definition.
package gitlabcore

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

const (
	// defaultRequeueDelay paces the loop. Every successful reconcile asks for
	// the next one, so this is both how fast an instance is followed while it
	// starts and how long drift survives once it runs. A reconcile renders the
	// whole chart and applies every object, so it is deliberately slower than
	// the delay of the v1beta1 controller.
	defaultRequeueDelay = 30 * time.Second

	// defaultHookTimeout bounds one chart hook. The Helm CLI default of five
	// minutes is too short for the shared secrets Job, which pulls the toolbox
	// image on a cold node.
	defaultHookTimeout = 10 * time.Minute

	// finalizerName marks a resource whose cluster-scoped objects still have to
	// be swept. See finalize.
	finalizerName = "gitlabcore.apps.gitlab.com/finalizer"
)

// Reconciler reconciles a GitLabCore object.
type Reconciler struct {
	client.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	// Discovery reads the cluster facts the chart renders against. Left unset,
	// SetupWithManager builds one from the configuration of the manager.
	Discovery discovery.DiscoveryInterface

	// RESTClientGetter builds the Helm kube client the chart hooks run through.
	// Left unset, SetupWithManager falls back to the default configuration
	// loading rules, like the legacy renderer does.
	RESTClientGetter genericclioptions.RESTClientGetter

	// HookTimeout bounds one chart hook. It defaults to defaultHookTimeout.
	HookTimeout time.Duration
}

// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabcores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabcores/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabcores/status,verbs=get;update;patch

// The reconciler applies everything the chart renders, which the v1beta1
// controller does not, so it needs permissions that controller never asked for.
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

// Reconcile brings the cluster in line with one GitLabCore resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gitlabcore", req.NamespacedName)

	core := &apiv2alpha1.GitLabCore{}
	if err := r.Get(ctx, req.NamespacedName, core); err != nil {
		if errors.IsNotFound(err) {
			log.V(1).Info("GitLabCore not found, nothing to reconcile")

			return doNotRequeue()
		}

		return ctrl.Result{}, err
	}

	if !core.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, core, log)
	}

	// The finalizer goes on before anything is created, and this pass continues
	// with it in place. Returning here instead would stall the resource: adding a
	// finalizer changes metadata, not the spec, so the generation stays the same
	// and the event filter of SetupWithManager drops the update.
	if !controllerutil.ContainsFinalizer(core, finalizerName) {
		controllerutil.AddFinalizer(core, finalizerName)

		if err := r.Update(ctx, core); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("reconciling GitLabCore", "chart version", core.Spec.Chart.Version)

	result, err := r.reconcile(ctx, core, log)

	// The status is written once per loop, with whatever the loop observed
	// before it returned.
	if statusErr := r.Status().Update(ctx, core); statusErr != nil {
		log.Error(statusErr, "unable to update the GitLabCore status")

		if err == nil {
			return ctrl.Result{}, statusErr
		}
	}

	return result, err
}

// reconcile renders the release and applies it, recording what it observes on
// the status of the resource in memory.
func (r *Reconciler) reconcile(ctx context.Context, core *apiv2alpha1.GitLabCore, log logr.Logger) (ctrl.Result, error) {
	discovered, err := r.capabilities()
	if err != nil {
		return ctrl.Result{}, err
	}

	// An upgrade renders and converges toward the next minor version rather than
	// straight to the target, which is what keeps a multi-minor upgrade a
	// sequence of zero-downtime single-minor ones. A required intermediate the
	// Operator does not carry blocks the upgrade: nothing self-heals until the
	// chart is added or the spec changes, so it does not requeue.
	renderVersion := core.Spec.Chart.Version
	upgrading := isUpgrade(core)

	if upgrading {
		next, err := nextChartVersion(charts.GlobalCatalog(), core.Status.Version, core.Spec.Chart.Version)
		if err != nil {
			r.Recorder.Eventf(core, nil, corev1.EventTypeWarning, "UpgradeBlocked", "Upgrade",
				"Upgrade cannot proceed: %v", err)

			log.Error(err, "the upgrade cannot proceed, check the GitLabCore events")

			core.Status.Phase = PhaseFailed
			setCondition(core, ConditionUpgradeable, metav1.ConditionFalse, reasonMissingIntermediateChart, err.Error())

			return doNotRequeue()
		}

		renderVersion = next

		setCondition(core, ConditionUpgradeable, metav1.ConditionTrue, reasonUpgradePathValid,
			fmt.Sprintf("upgrading toward %s, next %s", core.Spec.Chart.Version, renderVersion))
	}

	release, err := renderReleaseAt(core, settings.HelmChartsDirectory, renderVersion, discovered)
	if err != nil {
		// The specification cannot produce a release. Nothing changes until the
		// resource does, and a resource change triggers a new reconcile, so
		// retrying on a timer would only repeat the same error.
		r.Recorder.Eventf(core, nil, corev1.EventTypeWarning, "ConfigError", "Render",
			"Configuration error detected: %v", err)

		log.Error(err, "unable to render the GitLab chart, check the GitLabCore events")

		core.Status.Phase = PhaseFailed
		setCondition(core, ConditionInitialized, metav1.ConditionFalse, reasonRenderFailed, err.Error())

		return doNotRequeue()
	}

	for _, warning := range release.Warnings {
		log.Info("dropped a rendered document", "warning", warning.String())
	}

	core.Status.Phase = PhasePreparing

	// The hooks of a release run once, not once per pass.
	if hooksAreCurrent(core) {
		log.V(1).Info("the hooks of this release already ran, skipping them",
			"generation", core.Generation)
	} else if err := r.runHooks(ctx, core, release, hookEventPreInstall, log); err != nil {
		setCondition(core, ConditionInitialized, metav1.ConditionFalse, reasonHooksFailed, err.Error())

		return ctrl.Result{}, err
	}

	// The condition doubles as the marker hooksAreCurrent reads, so it is
	// recorded before the apply: an apply that fails is retried without the
	// hooks, which have nothing to do with it.
	setCondition(core, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered,
		fmt.Sprintf("the GitLab chart %s is rendered and its hooks ran", renderVersion))

	// An upgrade applies the release in the zero-downtime order rather than all
	// at once: pre-migrations, roll out, post-migrations, drop the schema bypass.
	// It records the version and drives the next minor step itself.
	if upgrading {
		return r.reconcileUpgrade(ctx, core, release, renderVersion, log)
	}

	if err := r.applyObjects(ctx, core, release, log); err != nil {
		setCondition(core, ConditionAvailable, metav1.ConditionFalse, reasonApplyFailed, err.Error())

		return ctrl.Result{}, err
	}

	// The version is recorded once the objects are applied, whether or not they
	// are ready, because it describes what was deployed rather than its health.
	core.Status.Version = renderVersion

	ready, pending, err := r.workloadsReady(ctx, core, release)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !ready {
		log.Info("waiting for a workload to become ready", "workload", pending)

		setCondition(core, ConditionAvailable, metav1.ConditionFalse, reasonWorkloadsNotReady,
			fmt.Sprintf("waiting for %s to become ready", pending))

		return requeueWithDefaultDelay()
	}

	log.Info("GitLab is running")

	core.Status.Phase = PhaseRunning
	setCondition(core, ConditionAvailable, metav1.ConditionTrue, reasonWorkloadsReady,
		"every rendered workload has its desired replicas ready")

	// A running instance is reconciled again anyway. Nothing watches the objects
	// of the release, so the requeue is what repairs drift.
	return requeueWithDefaultDelay()
}

// SetupWithManager registers the reconciler and the resources it watches.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.HookTimeout == 0 {
		r.HookTimeout = defaultHookTimeout
	}

	if r.Discovery == nil {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
		if err != nil {
			return fmt.Errorf("building the discovery client: %w", err)
		}

		r.Discovery = discoveryClient
	}

	if r.RESTClientGetter == nil {
		r.RESTClientGetter = genericclioptions.NewConfigFlags(true)
	}

	// Nothing but the resource is watched. A release is hundreds of objects of
	// kinds the Operator does not know ahead of time, so watching them would
	// mean an informer per kind and a reconcile, which renders the whole chart,
	// on every status update they make. The requeue covers them instead: a
	// deleted or edited object is restored on the next pass.
	//
	// The predicate keeps the status write of each loop from triggering the
	// next one. The deletion of the resource still passes it, because marking a
	// resource that carries a finalizer for deletion increments its generation.
	return ctrl.NewControllerManagedBy(mgr).
		Named("gitlabcore").
		For(&apiv2alpha1.GitLabCore{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func doNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func requeueWithDefaultDelay() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: defaultRequeueDelay}, nil
}
