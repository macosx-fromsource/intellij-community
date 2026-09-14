package siphon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// reconcileOnce runs one pass over a resource and returns it as the reconciler
// left it.
func reconcileOnce(t *testing.T, reconciler *Reconciler, siphon *apiv2alpha1.Siphon) (*apiv2alpha1.Siphon, ctrl.Result, error) {
	t.Helper()

	request := ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: siphon.Namespace, Name: siphon.Name,
	}}

	result, err := reconciler.Reconcile(context.Background(), request)

	observed := &apiv2alpha1.Siphon{}
	require.NoError(t, reconciler.Get(context.Background(), request.NamespacedName, observed))

	return observed, result, err
}

// conditionOf returns a condition of a resource, or nil.
func conditionOf(siphon *apiv2alpha1.Siphon, conditionType string) *metav1.Condition {
	return apimeta.FindStatusCondition(siphon.Status.Conditions, conditionType)
}

func TestReconcileWaitsForItsInstance(t *testing.T) {
	t.Run("waits for an instance that does not exist yet", func(t *testing.T) {
		// Bridge writes the two resources separately, so a Siphon can
		// legitimately exist before its instance does.
		siphon := newSiphon()
		reconciler := testReconciler(t, siphon)

		observed, result, err := reconcileOnce(t, reconciler, siphon)

		require.NoError(t, err)
		assert.Equal(t, defaultRequeueDelay, result.RequeueAfter)
		assert.Equal(t, PhasePreparing, observed.Status.Phase)

		condition := conditionOf(observed, ConditionInitialized)
		require.NotNil(t, condition)
		assert.Equal(t, reasonGitLabNotFound, condition.Reason)
	})

	t.Run("waits for an instance that is not available yet", func(t *testing.T) {
		core := availableGitLab(testGitLabVersion)
		apimeta.SetStatusCondition(&core.Status.Conditions, metav1.Condition{
			Type:   gitLabConditionAvailable,
			Status: metav1.ConditionFalse,
			Reason: "WorkloadsNotReady",
		})

		siphon := newSiphon()
		reconciler := testReconciler(t, siphon, core)

		observed, result, err := reconcileOnce(t, reconciler, siphon)

		require.NoError(t, err)
		assert.Equal(t, defaultRequeueDelay, result.RequeueAfter)

		condition := conditionOf(observed, ConditionInitialized)
		require.NotNil(t, condition)
		assert.Equal(t, reasonGitLabNotReady, condition.Reason)
	})

	t.Run("waits for an instance that has not published its version yet", func(t *testing.T) {
		// The instance records the version of the application it deploys on a
		// pass of its own, which this reconcile cannot order itself against, so
		// it comes back rather than failing.
		siphon := newSiphon()
		reconciler := testReconciler(t, siphon, availableGitLab(""))

		observed, result, err := reconcileOnce(t, reconciler, siphon)

		require.NoError(t, err)
		assert.Equal(t, defaultRequeueDelay, result.RequeueAfter)
		assert.Equal(t, PhasePreparing, observed.Status.Phase)

		condition := conditionOf(observed, ConditionInitialized)
		require.NotNil(t, condition)
		assert.Equal(t, reasonVersionPending, condition.Reason)
	})
}

func TestReconcileAddsTheFinalizerAndContinues(t *testing.T) {
	// Adding a finalizer changes metadata, not the spec, so the generation stays
	// the same and the event filter would drop the update. Returning after adding
	// it would stall the resource, so the same pass has to continue.
	siphon := newSiphon()
	reconciler := testReconciler(t, siphon)

	observed, _, err := reconcileOnce(t, reconciler, siphon)

	require.NoError(t, err)
	assert.True(t, controllerutil.ContainsFinalizer(observed, finalizerName))

	// The pass went on to record what it observed rather than returning early.
	assert.NotEmpty(t, observed.Status.Conditions)
	assert.Equal(t, producerID, observed.Status.Publication)
}

func TestReconcileRecordsTheRetainedState(t *testing.T) {
	// These are recorded on the first pass, before anything is deployed, because
	// they are what an administrator needs after the resource is gone.
	siphon := newSiphon()
	reconciler := testReconciler(t, siphon)

	observed, _, err := reconcileOnce(t, reconciler, siphon)

	require.NoError(t, err)
	assert.Equal(t, producerID, observed.Status.Publication)
	assert.Equal(t, replicationSlot, observed.Status.ReplicationSlot)
	assert.Equal(t, streamName, observed.Status.StreamName)
}

func TestReconcileIsANoOpForAResourceThatIsGone(t *testing.T) {
	reconciler := testReconciler(t)

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName},
	})

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
}

func TestFinalizeWarnsAboutTheRetainedSlot(t *testing.T) {
	// A retained replication slot pins write-ahead log indefinitely and will
	// eventually fill the volume of the source server. The Operator cannot remove
	// it under the reference-only scope of this resource, so it says so.
	siphon := newSiphon()
	siphon.Finalizers = []string{finalizerName}
	siphon.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}

	reconciler := testReconciler(t, siphon)
	recorder := &recordingRecorder{}
	reconciler.Recorder = recorder

	_, err := reconciler.finalize(context.Background(), siphon, testLogger(t))
	require.NoError(t, err)

	require.Len(t, recorder.events, 1)
	assert.Equal(t, slotRetainedEvent, recorder.events[0].reason)
	assert.Contains(t, recorder.events[0].message, replicationSlot)
	assert.Contains(t, recorder.events[0].message, "pg_drop_replication_slot")
	assert.Contains(t, recorder.events[0].message, streamName)

	assert.False(t, controllerutil.ContainsFinalizer(siphon, finalizerName))
}

func TestFinalizeIsANoOpWithoutTheFinalizer(t *testing.T) {
	siphon := newSiphon()
	reconciler := testReconciler(t, siphon)
	reconciler.Recorder = &recordingRecorder{}

	result, err := reconciler.finalize(context.Background(), siphon, testLogger(t))

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
}

// compile-time check that the reconciler satisfies the interface the manager
// registers.
var _ interface {
	Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
} = &Reconciler{}

var _ client.Object = &apiv2alpha1.Siphon{}
