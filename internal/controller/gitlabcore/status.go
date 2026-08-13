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
)

// The phases a GitLabCore reports through status.phase, which is the column
// kubectl prints.
const (
	PhasePreparing = "Preparing"
	PhaseRunning   = "Running"
	PhaseFailed    = "Failed"
)

// The reasons the conditions carry.
const (
	reasonChartRendered     = "ChartRendered"
	reasonRenderFailed      = "RenderFailed"
	reasonHooksFailed       = "HooksFailed"
	reasonApplyFailed       = "ApplyFailed"
	reasonWorkloadsReady    = "WorkloadsReady"
	reasonWorkloadsNotReady = "WorkloadsNotReady"
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

	for _, workload := range objects.Filter(result.Objects, objects.Or(predicates...)) {
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

// workloadReady reports whether a live workload has its desired replicas ready.
// A replica count the object does not carry is one, the Kubernetes default, and
// a workload scaled to zero is ready by definition.
func workloadReady(live *unstructured.Unstructured) (bool, error) {
	desired, found, err := unstructured.NestedInt64(live.Object, "spec", "replicas")
	if err != nil {
		return false, fmt.Errorf("reading the replicas of %s %q: %w", live.GetKind(), live.GetName(), err)
	}

	if !found {
		desired = 1
	}

	ready, _, err := unstructured.NestedInt64(live.Object, "status", "readyReplicas")
	if err != nil {
		return false, fmt.Errorf("reading the ready replicas of %s %q: %w", live.GetKind(), live.GetName(), err)
	}

	return ready >= desired, nil
}
