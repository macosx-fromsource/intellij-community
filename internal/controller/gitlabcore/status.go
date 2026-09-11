package gitlabcore

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// The condition types of a GitLabCore. The set is deliberately small; it grows
// with what Bridge renders, which ADR 26 leaves open.
const (
	// ConditionInitialized reports whether the chart resolved and rendered and
	// its hooks ran. It is false while the specification cannot produce a
	// release, which is the condition an administrator has to act on.
	ConditionInitialized = "Initialized"

	// ConditionAvailable reports whether every rendered workload has its
	// desired replicas ready.
	ConditionAvailable = "Available"

	// ConditionProgressing reports whether a zero-downtime upgrade is in
	// flight. It is true while a migration Job runs or the new workloads roll
	// out, and false once the deployed version reaches the spec.
	ConditionProgressing = "Progressing"

	// ConditionUpgradeable reports whether the upgrade the spec asks for can be
	// carried out. It is false when the path is not a zero-downtime one, for
	// example when an intermediate chart a multi-minor upgrade steps through is
	// not among the charts the Operator carries.
	ConditionUpgradeable = "Upgradeable"
)

// The phases a GitLabCore reports through status.phase, which is the column
// kubectl prints.
const (
	PhasePreparing = "Preparing"
	PhaseUpgrading = "Upgrading"
	PhaseRunning   = "Running"
	PhaseFailed    = "Failed"
)

// The reasons the conditions carry.
const (
	reasonChartRendered     = "ChartRendered"
	reasonRenderFailed      = "RenderFailed"
	reasonChartPullFailed   = "ChartPullFailed"
	reasonHooksFailed       = "HooksFailed"
	reasonApplyFailed       = "ApplyFailed"
	reasonWorkloadsReady    = "WorkloadsReady"
	reasonWorkloadsNotReady = "WorkloadsNotReady"

	// The reasons of the upgrade conditions.
	reasonRunningPreMigrations  = "RunningPreMigrations"
	reasonUpgradingRails        = "UpgradingRails"
	reasonRollingOutWorkloads   = "RollingOutWorkloads"
	reasonRunningPostMigrations = "RunningPostMigrations"
	reasonAdvancingVersion      = "AdvancingVersion"
	reasonUpgradeComplete       = "UpgradeComplete"

	reasonMissingIntermediateChart = "MissingIntermediateChart"
	reasonUpgradePathValid         = "UpgradePathValid"
	reasonMigrationsJobFailed      = "MigrationsJobFailed"
)

// The workload kinds whose readiness decides ConditionAvailable.
var readinessKinds = []string{"Deployment", "StatefulSet"}

// setCondition records a condition on the resource in memory. Reconcile writes
// the status once, at the end of the loop.
func setCondition(core *apiv2alpha1.GitLabCore, conditionType string, status metav1.ConditionStatus, reason, message string) {
	apimeta.SetStatusCondition(&core.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: core.Generation,
	})
}

// workloadsReady reports whether every rendered workload has its desired
// replicas ready, naming the first one that does not.
//
// The desired count comes from the live object rather than the rendered one, so
// that a Deployment an autoscaler or an administrator scaled is measured
// against what the cluster wants. A workload that is not created yet counts as
// not ready.
func (r *Reconciler) workloadsReady(ctx context.Context, core *apiv2alpha1.GitLabCore, result *render.Result) (bool, string, error) {
	predicates := make([]objects.Predicate, 0, len(readinessKinds))

	for _, kind := range readinessKinds {
		predicates = append(predicates, objects.ByKind(kind))
	}

	return r.workloadsReadyIn(ctx, core, objects.Filter(result.Objects, objects.Or(predicates...)))
}

// workloadsReadyIn reports whether every workload in the given set has its
// desired replicas ready, naming the first one that does not. It is the body of
// workloadsReady, split out so a zero-downtime upgrade can gate on a subset of
// the release, for example only the webservice and sidekiq Deployments.
func (r *Reconciler) workloadsReadyIn(ctx context.Context, core *apiv2alpha1.GitLabCore, workloads []*unstructured.Unstructured) (bool, string, error) {
	for _, workload := range workloads {
		namespace := workload.GetNamespace()
		if namespace == "" {
			namespace = core.Namespace
		}

		name := fmt.Sprintf("%s %s", workload.GetKind(), workload.GetName())

		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(workload.GroupVersionKind())

		err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: workload.GetName()}, live)

		switch {
		case apierrors.IsNotFound(err):
			return false, name, nil
		case err != nil:
			return false, "", err
		}

		ready, err := workloadReady(live)
		if err != nil {
			return false, "", err
		}

		if !ready {
			return false, name, nil
		}
	}

	return true, "", nil
}

// workloadReady reports whether a live workload has finished rolling out its
// desired replicas. A replica count the object does not carry is one, the
// Kubernetes default, and a workload scaled to zero is ready by definition.
//
// Readiness alone (status.readyReplicas) is not enough for a zero-downtime
// upgrade: right after a paused Deployment is unpaused, the old pods are still
// ready, so a ready-only check would pass before the new pods roll out and let
// the post-deployment migrations run against the old code (AC #2). So this
// applies the completion check `kubectl rollout status` uses: the controller has
// observed the current spec (observedGeneration), every desired replica is
// updated to the new template (updatedReplicas), no old replicas linger
// (status.replicas), and the updated replicas are available (availableReplicas).
func workloadReady(live *unstructured.Unstructured) (bool, error) {
	desired, found, err := unstructured.NestedInt64(live.Object, "spec", "replicas")
	if err != nil {
		return false, fmt.Errorf("reading the replicas of %s %q: %w", live.GetKind(), live.GetName(), err)
	}

	if !found {
		desired = 1
	}

	if desired == 0 {
		return true, nil
	}

	// Until the controller has observed the current spec, the status counts
	// describe the previous template and cannot report on this rollout.
	observed, err := statusInt64(live, "observedGeneration")
	if err != nil {
		return false, err
	}

	if observed < live.GetGeneration() {
		return false, nil
	}

	updated, err := statusInt64(live, "updatedReplicas")
	if err != nil {
		return false, err
	}

	total, err := statusInt64(live, "replicas")
	if err != nil {
		return false, err
	}

	available, err := statusInt64(live, "availableReplicas")
	if err != nil {
		return false, err
	}

	// Every desired replica runs the new template, no old replica lingers, and
	// every replica present is available.
	return updated >= desired && total <= updated && available >= desired, nil
}

// statusInt64 reads a status count off a live workload, treating an absent field
// as zero the way the Kubernetes controllers do before they first write it.
func statusInt64(live *unstructured.Unstructured, field string) (int64, error) {
	value, _, err := unstructured.NestedInt64(live.Object, "status", field)
	if err != nil {
		return 0, fmt.Errorf("reading status.%s of %s %q: %w", field, live.GetKind(), live.GetName(), err)
	}

	return value, nil
}
