package siphon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// liveWorkload builds a single-replica Deployment, rolled out when ready is one
// and still rolling out when it is zero.
func liveWorkload(t *testing.T, name string, ready int64) *unstructured.Unstructured {
	t.Helper()

	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion("apps/v1")
	workload.SetKind("Deployment")
	workload.SetName(name)
	workload.SetNamespace(testNamespace)

	require.NoError(t, unstructured.SetNestedField(workload.Object, int64(1), "spec", "replicas"))
	require.NoError(t, unstructured.SetNestedField(workload.Object, int64(1), "status", "observedGeneration"))

	for _, field := range []string{"readyReplicas", "updatedReplicas", "availableReplicas", "replicas"} {
		require.NoError(t, unstructured.SetNestedField(workload.Object, ready, "status", field))
	}

	return workload
}

// renderedDeployment builds the rendered counterpart of a workload.
func renderedDeployment(name string) *unstructured.Unstructured {
	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion("apps/v1")
	workload.SetKind("Deployment")
	workload.SetName(name)
	workload.SetNamespace(testNamespace)

	return workload
}

func TestSetCondition(t *testing.T) {
	t.Run("carries the generation it observed", func(t *testing.T) {
		siphon := newSiphon()
		siphon.Generation = 7

		setCondition(siphon, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered, "rendered")

		condition := apimeta.FindStatusCondition(siphon.Status.Conditions, ConditionInitialized)

		require.NotNil(t, condition)
		assert.Equal(t, metav1.ConditionTrue, condition.Status)
		assert.Equal(t, reasonChartRendered, condition.Reason)
		assert.Equal(t, int64(7), condition.ObservedGeneration)
	})

	t.Run("replaces the condition of the same type instead of appending one", func(t *testing.T) {
		siphon := newSiphon()

		setCondition(siphon, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered, "rendered")
		setCondition(siphon, ConditionInitialized, metav1.ConditionFalse, reasonApplyFailed, "failed")

		assert.Len(t, siphon.Status.Conditions, 1)
		assert.False(t, apimeta.IsStatusConditionTrue(siphon.Status.Conditions, ConditionInitialized))
	})
}

func TestSetTopologyStatus(t *testing.T) {
	t.Run("records what outlives the resource", func(t *testing.T) {
		// Deleting the resource leaves all three behind, and a retained
		// replication slot pins write-ahead log, so an administrator needs the
		// names after the object is gone.
		siphon := newSiphon()

		setTopologyStatus(siphon)

		assert.Equal(t, producerID, siphon.Status.Publication)
		assert.Equal(t, replicationSlot, siphon.Status.ReplicationSlot)
		assert.Equal(t, streamName, siphon.Status.StreamName)
	})
}

func TestWorkloadsReady(t *testing.T) {
	ctx := context.Background()

	siphon := newSiphon()

	rendered := &render.Result{Objects: []*unstructured.Unstructured{
		renderedDeployment(producerDeployment(siphon.Name)),
		renderedDeployment(consumerDeployment(siphon.Name)),
		renderedDeployment(reconcilerDeployment(siphon.Name)),
	}}

	t.Run("reports the pipeline as up once the producer and the consumer rolled out", func(t *testing.T) {
		reconciler := testReconciler(t,
			liveWorkload(t, producerDeployment(siphon.Name), 1),
			liveWorkload(t, consumerDeployment(siphon.Name), 1),
			liveWorkload(t, reconcilerDeployment(siphon.Name), 1),
		)

		ready, pending, err := reconciler.workloadsReady(ctx, siphon, rendered)

		require.NoError(t, err)
		assert.True(t, ready)
		assert.Empty(t, pending)
	})

	t.Run("ignores the reconciler, whose readiness proves nothing", func(t *testing.T) {
		// Siphon serves placeholder health endpoints for that role which answer
		// 200 unconditionally, so including it would report health the reconciler
		// cannot observe. Here it has not rolled out at all and the pipeline is
		// still up.
		reconciler := testReconciler(t,
			liveWorkload(t, producerDeployment(siphon.Name), 1),
			liveWorkload(t, consumerDeployment(siphon.Name), 1),
			liveWorkload(t, reconcilerDeployment(siphon.Name), 0),
		)

		ready, _, err := reconciler.workloadsReady(ctx, siphon, rendered)

		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("names the producer when it has not rolled out", func(t *testing.T) {
		reconciler := testReconciler(t,
			liveWorkload(t, producerDeployment(siphon.Name), 0),
			liveWorkload(t, consumerDeployment(siphon.Name), 1),
		)

		ready, pending, err := reconciler.workloadsReady(ctx, siphon, rendered)

		require.NoError(t, err)
		assert.False(t, ready)
		assert.Contains(t, pending, producerDeployment(siphon.Name))
	})

	t.Run("names the consumer when it has not rolled out", func(t *testing.T) {
		reconciler := testReconciler(t,
			liveWorkload(t, producerDeployment(siphon.Name), 1),
			liveWorkload(t, consumerDeployment(siphon.Name), 0),
		)

		ready, pending, err := reconciler.workloadsReady(ctx, siphon, rendered)

		require.NoError(t, err)
		assert.False(t, ready)
		assert.Contains(t, pending, consumerDeployment(siphon.Name))
	})

	t.Run("treats a workload that does not exist yet as not up", func(t *testing.T) {
		reconciler := testReconciler(t)

		ready, pending, err := reconciler.workloadsReady(ctx, siphon, rendered)

		require.NoError(t, err)
		assert.False(t, ready)
		assert.NotEmpty(t, pending)
	})
}
