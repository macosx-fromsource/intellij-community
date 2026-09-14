package gitlabcore

import (
	"context"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
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
	reasonRunningPreMigrations        = "RunningPreMigrations"
	reasonUpgradingRails              = "UpgradingRails"
	reasonRollingOutWorkloads         = "RollingOutWorkloads"
	reasonRunningPostMigrations       = "RunningPostMigrations"
	reasonWaitingForBatchedMigrations = "WaitingForBatchedMigrations"
	reasonAdvancingVersion            = "AdvancingVersion"
	reasonUpgradeComplete             = "UpgradeComplete"

	reasonMissingIntermediateChart     = "MissingIntermediateChart"
	reasonUpgradePathValid             = "UpgradePathValid"
	reasonMigrationsJobFailed          = "MigrationsJobFailed"
	reasonBatchedMigrationsCheckFailed = "BatchedMigrationsCheckFailed"
)

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
func (r *Reconciler) workloadsReady(ctx context.Context, core *apiv2alpha1.GitLabCore, result *render.Result) (bool, string, error) {
	predicates := make([]objects.Predicate, 0, len(release.WorkloadKinds))

	for _, kind := range release.WorkloadKinds {
		predicates = append(predicates, objects.ByKind(kind))
	}

	return r.workloadsReadyIn(ctx, core, objects.Filter(result.Objects, objects.Or(predicates...)))
}

// workloadsReadyIn reports whether every workload in the given set has its
// desired replicas ready, naming the first one that does not. A zero-downtime
// upgrade gates on a subset of the release this way, for example only the
// webservice and sidekiq Deployments.
func (r *Reconciler) workloadsReadyIn(ctx context.Context, core *apiv2alpha1.GitLabCore, workloads []*unstructured.Unstructured) (bool, string, error) {
	return release.WorkloadsReady(ctx, r.Client, r.PodReader, core.Namespace, workloads)
}
