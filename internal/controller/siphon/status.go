package siphon

import (
	"context"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// The condition types of a Siphon.
const (
	// ConditionInitialized reports whether the reference resolved, the chart
	// rendered, and the release was applied. It is false while the
	// specification cannot produce a release, which is the condition an
	// administrator has to act on.
	ConditionInitialized = "Initialized"

	// ConditionTablesResolved reports whether the table definitions of the
	// deployed GitLab version are in place: mounted from the image, or published
	// to the ConfigMap the Operator owns.
	ConditionTablesResolved = "TablesResolved"

	// ConditionAvailable reports whether the producer and the consumer have
	// finished rolling out.
	//
	// It excludes the reconciler on purpose. Siphon serves placeholder health
	// endpoints for that role which answer 200 unconditionally, so its readiness
	// proves only that a process is listening. Including it would make this
	// condition report health it cannot observe.
	//
	// It is not proof that change data is flowing. The producer answers ready as
	// soon as it holds the advisory lock, which it does against a publication
	// with no tables in it, so a pipeline that replicates nothing reports
	// available. See doc/developer/siphon.md for the checks that do answer that
	// question.
	ConditionAvailable = "Available"
)

// The phases a Siphon reports through status.phase, which is the column kubectl
// prints.
const (
	PhasePreparing = "Preparing"
	PhaseRunning   = "Running"
	PhaseFailed    = "Failed"
)

// The reasons the conditions carry.
const (
	reasonChartRendered   = "ChartRendered"
	reasonRenderFailed    = "RenderFailed"
	reasonApplyFailed     = "ApplyFailed"
	reasonGitLabNotFound  = "GitLabNotFound"
	reasonGitLabNotReady  = "GitLabNotReady"
	reasonVersionPending  = "GitLabVersionPending"
	reasonTablesMounted   = "TablesMounted"
	reasonTablesPublished = "TablesPublished"
	reasonTablesUnchanged = "TablesUnchanged"
	reasonTablesFailed    = "TablesFetchFailed"
	reasonTablesTooLarge  = "TablesTooLarge"

	reasonWorkloadsReady    = "WorkloadsReady"
	reasonWorkloadsNotReady = "WorkloadsNotReady"
)

// setCondition records a condition on the resource in memory. Reconcile writes
// the status once, at the end of the loop.
func setCondition(siphon *apiv2alpha1.Siphon, conditionType string, status metav1.ConditionStatus, reason, message string) {
	apimeta.SetStatusCondition(&siphon.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: siphon.Generation,
	})
}

// setTopologyStatus records the PostgreSQL and NATS objects the release depends
// on.
//
// They are derived from the fixed topology rather than observed, and they are
// recorded because deleting the resource leaves all three behind. A retained
// replication slot pins write-ahead log and will eventually fill the volume of
// the source server, so an administrator needs the names after the object is
// gone.
func setTopologyStatus(siphon *apiv2alpha1.Siphon) {
	siphon.Status.Publication = producerID
	siphon.Status.ReplicationSlot = replicationSlot
	siphon.Status.StreamName = streamName
}

// workloadsReady reports whether the producer and the consumer of the release
// have finished rolling out, naming the first one that has not.
func (r *Reconciler) workloadsReady(ctx context.Context, siphon *apiv2alpha1.Siphon, result *render.Result) (bool, string, error) {
	gated := objects.Filter(result.Objects, objects.And(
		objects.ByKind("Deployment"),
		anyName(gatedDeployments(siphon.Name)...),
	))

	return release.WorkloadsReady(ctx, r.Client, r.PodReader, siphon.Namespace, gated)
}

// anyName matches an object whose name is one of the given ones.
func anyName(names ...string) objects.Predicate {
	predicates := make([]objects.Predicate, 0, len(names))

	for _, name := range names {
		predicates = append(predicates, objects.ByName(name))
	}

	return objects.Or(predicates...)
}
