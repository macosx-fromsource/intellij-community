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

// Package siphon reconciles the apps.gitlab.com/v2alpha1 Siphon resource, the
// change data capture pipeline from the PostgreSQL database of a GitLab instance
// into ClickHouse.
//
// It renders the Siphon chart with internal/render and applies the result. The
// resource references its PostgreSQL source, its NATS server and its ClickHouse
// sink; it provisions none of them, and the source database objects the producer
// needs are administrator prerequisites. See doc/developer/siphon.md.
//
// It shares the release mechanics with the GitLabCore reconciler through
// internal/controller/release, and keeps its own vocabulary: its conditions, its
// phases, and the fixed deployment topology in topology.go.
//
// Siphon waits for the instance it references to become available. The reverse
// must never hold: nothing in the readiness of a GitLabCore may depend on a
// Siphon, because the two would deadlock, each waiting for the other. As peers
// under ADR 24 that holds by construction, and this is the reason to keep it that
// way.
//
// The reconciler is alpha and rides with Bridge, so it is gated twice: the
// `bridge` build tag compiles the wiring in (siphon.go / siphon_stub.go in
// cmd/manager), and ENABLE_BRIDGE registers it at runtime. Its definition reaches
// no installation (see ADR 26 and ADR 27), so registering the watch
// unconditionally would fail the manager on every cluster that has no siphons
// definition.
package siphon

import (
	"context"
	goerrors "errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
)

const (
	// defaultRequeueDelay paces the loop. Every successful reconcile asks for
	// the next one, so this is both how fast a pipeline is followed while it
	// starts and how long drift survives once it runs.
	defaultRequeueDelay = 30 * time.Second

	// finalizerName marks a resource whose retained state still has to be
	// reported and whose unowned objects still have to be swept. See finalize.
	finalizerName = "siphon.apps.gitlab.com/finalizer"
)

// Reconciler reconciles a Siphon object.
type Reconciler struct {
	client.Client

	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	// Discovery reads the cluster facts the chart renders against. Left unset,
	// SetupWithManager builds one from the configuration of the manager.
	Discovery discovery.DiscoveryInterface

	// PodReader is asked why a workload of the release is not ready, and only
	// then. It is the uncached reader of the manager, because nothing here
	// watches pods and a cached read would start an informer for every pod in
	// scope. Left unset, SetupWithManager takes it from the manager, and a
	// not-ready workload is reported by name alone until it is set.
	PodReader client.Reader
}

// +kubebuilder:rbac:groups=apps.gitlab.com,resources=siphons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=siphons/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=siphons/status,verbs=get;update;patch

// The reference is resolved by reading the instance it names.
// +kubebuilder:rbac:groups=apps.gitlab.com,resources=gitlabcores,verbs=get;list;watch

// Read to report why a workload of the release is not ready.
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list

// What the chart renders, plus the ConfigMap the extracted table definitions are
// published to and the Secret the registry credential is read from.
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Rendered only on a cluster that serves the Prometheus operator API.
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile brings the cluster in line with one Siphon resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("siphon", req.NamespacedName)

	siphon := &apiv2alpha1.Siphon{}
	if err := r.Get(ctx, req.NamespacedName, siphon); err != nil {
		if errors.IsNotFound(err) {
			log.V(1).Info("Siphon not found, nothing to reconcile")

			return doNotRequeue()
		}

		return ctrl.Result{}, err
	}

	if !siphon.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, siphon, log)
	}

	// The finalizer goes on before anything is created, and this pass continues
	// with it in place. Returning here instead would stall the resource: adding a
	// finalizer changes metadata, not the spec, so the generation stays the same
	// and the event filter of SetupWithManager drops the update.
	if !controllerutil.ContainsFinalizer(siphon, finalizerName) {
		controllerutil.AddFinalizer(siphon, finalizerName)

		if err := r.Update(ctx, siphon); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("reconciling Siphon", "chart version", siphon.Spec.Chart.Version)

	result, err := r.reconcile(ctx, siphon, log)

	// The status is written once per loop, with whatever the loop observed
	// before it returned.
	if statusErr := r.Status().Update(ctx, siphon); statusErr != nil {
		log.Error(statusErr, "unable to update the Siphon status")

		if err == nil {
			return ctrl.Result{}, statusErr
		}
	}

	return result, err
}

// reconcile resolves the reference, puts the table definitions in place, renders
// the release and applies it, recording what it observes on the status of the
// resource in memory.
func (r *Reconciler) reconcile(ctx context.Context, siphon *apiv2alpha1.Siphon, log logr.Logger) (ctrl.Result, error) {
	setTopologyStatus(siphon)

	discovered, err := r.capabilities()
	if err != nil {
		return ctrl.Result{}, err
	}

	resolved, result, err := r.resolveRelease(ctx, siphon, log)
	if resolved == nil {
		return result, err
	}

	if err := r.reconcileTables(ctx, siphon, resolved, log); err != nil {
		log.Error(err, "unable to put the table definitions in place, check the Siphon status")

		siphon.Status.Phase = PhasePreparing

		// A registry that cannot be reached, or an image that is not published
		// yet, is worth retrying: neither is repaired by a change to the
		// resource.
		return requeueWithDefaultDelay()
	}

	rendered, err := renderRelease(siphon, settings.HelmChartsDirectory, *resolved, discovered)
	if err != nil {
		// The specification cannot produce a release. Nothing changes until the
		// resource does, and a resource change triggers a new reconcile, so
		// retrying on a timer would only repeat the same error.
		r.Recorder.Eventf(siphon, nil, corev1.EventTypeWarning, "ConfigError", "Render",
			"Configuration error detected: %v", err)

		log.Error(err, "unable to render the Siphon chart, check the Siphon events")

		siphon.Status.Phase = PhaseFailed
		setCondition(siphon, ConditionInitialized, metav1.ConditionFalse, reasonRenderFailed, err.Error())

		return doNotRequeue()
	}

	for _, warning := range rendered.Warnings {
		log.Info("dropped a rendered document", "warning", warning.String())
	}

	// Where the cluster does not mount an OCI image into a pod, the release is
	// rewritten to read the definitions from the ConfigMap instead. No chart
	// value can express this: the chart emits the image volume unconditionally
	// in split mode and appends the volumes a value supplies.
	if resolved.TablesSource == apiv2alpha1.TablesSourceConfigMap {
		if err := mountTables(rendered.Objects, resolved.TablesConfigMap); err != nil {
			siphon.Status.Phase = PhaseFailed
			setCondition(siphon, ConditionInitialized, metav1.ConditionFalse, reasonRenderFailed, err.Error())

			return doNotRequeue()
		}
	}

	siphon.Status.Phase = PhasePreparing

	if err := r.applyObjects(ctx, siphon, rendered, log); err != nil {
		setCondition(siphon, ConditionInitialized, metav1.ConditionFalse, reasonApplyFailed, err.Error())

		return ctrl.Result{}, err
	}

	setCondition(siphon, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered,
		fmt.Sprintf("the Siphon chart %s is rendered and applied", siphon.Spec.Chart.Version))

	// The version is recorded once the objects are applied, whether or not they
	// are ready, because it describes what was deployed rather than its health.
	siphon.Status.Version = siphon.Spec.Chart.Version

	ready, pending, err := r.workloadsReady(ctx, siphon, rendered)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !ready {
		log.Info("waiting for a workload to become ready", "workload", pending)

		setCondition(siphon, ConditionAvailable, metav1.ConditionFalse, reasonWorkloadsNotReady,
			fmt.Sprintf("waiting for %s to become ready", pending))

		return requeueWithDefaultDelay()
	}

	log.Info("Siphon is running")

	siphon.Status.Phase = PhaseRunning
	setCondition(siphon, ConditionAvailable, metav1.ConditionTrue, reasonWorkloadsReady,
		"the producer and the consumer have rolled out")

	// A running pipeline is reconciled again anyway. Nothing watches the objects
	// of the release, so the requeue is what repairs drift, and it is also what
	// re-resolves a table definitions tag that moves.
	return requeueWithDefaultDelay()
}

// resolveRelease reads the referenced instance and settles where the table
// definitions come from.
//
// A reference that does not resolve yet is a wait rather than a failure, so it
// returns a result to requeue on instead of an error. An unresolvable one, on the
// other hand, does not repair itself on a timer.
func (r *Reconciler) resolveRelease(ctx context.Context, siphon *apiv2alpha1.Siphon, log logr.Logger) (*Release, ctrl.Result, error) {
	version, err := r.resolveGitLab(ctx, siphon)
	if err != nil {
		siphon.Status.Phase = PhasePreparing

		var (
			notReady       *notReadyError
			versionPending *versionPendingError
		)

		switch {
		case goerrors.As(err, &notReady):
			log.Info("waiting for the referenced GitLab instance", "reason", err.Error())

			setCondition(siphon, ConditionInitialized, metav1.ConditionFalse, reasonGitLabNotReady, err.Error())

		case errors.IsNotFound(err):
			// Bridge writes the two resources separately, so a Siphon can
			// legitimately exist before its instance does. It resolves once the
			// instance is created, which is why this waits rather than fails.
			message := fmt.Sprintf("the GitLabCore %q does not exist in this namespace",
				siphon.Spec.GitLabRef.Name)

			log.Info("waiting for the referenced GitLab instance", "reason", message)

			setCondition(siphon, ConditionInitialized, metav1.ConditionFalse, reasonGitLabNotFound, message)

		case goerrors.As(err, &versionPending):
			log.Info("waiting for the referenced GitLab instance", "reason", err.Error())

			setCondition(siphon, ConditionInitialized, metav1.ConditionFalse, reasonVersionPending, err.Error())

		default:
			// Reading the instance failed for some other reason, such as an
			// unavailable API server. It is transient and not a statement about
			// the resource, so it goes back to the controller, which retries it
			// with a backoff.
			return nil, ctrl.Result{}, err
		}

		result, err := requeueWithDefaultDelay()

		return nil, result, err
	}

	siphon.Status.GitLabVersion = version

	return &Release{
		GitLabVersion: version,
		TablesImage:   resolveTablesImage(siphon, version),
		TablesSource:  resolveTablesSource(siphon.Spec.Tables.Source),
	}, ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler and the resources it watches.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Discovery == nil {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
		if err != nil {
			return fmt.Errorf("building the discovery client: %w", err)
		}

		r.Discovery = discoveryClient
	}

	if r.PodReader == nil {
		r.PodReader = mgr.GetAPIReader()
	}

	// Nothing but the resource is watched, not even the ConfigMap it owns: the
	// requeue restores a deleted or edited object on the next pass, and an
	// informer per kind would mean a reconcile, which renders the whole chart,
	// on every status update they make.
	//
	// The referenced instance is deliberately not watched either. Its status
	// changing does not bump its generation, so the event filter below would
	// drop the event, and the filter applies to every source. The requeue covers
	// it instead, at a worst case of one delay before a pipeline notices its
	// instance became available.
	//
	// The predicate keeps the status write of each loop from triggering the next
	// one. The deletion of the resource still passes it, because marking a
	// resource that carries a finalizer for deletion increments its generation.
	return ctrl.NewControllerManagedBy(mgr).
		Named("siphon").
		For(&apiv2alpha1.Siphon{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func doNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func requeueWithDefaultDelay() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: defaultRequeueDelay}, nil
}
