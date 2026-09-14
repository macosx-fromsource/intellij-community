package release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestWorkloadsReady(t *testing.T) {
	ctx := context.Background()

	rendered := []*unstructured.Unstructured{
		renderedObject("apps/v1", DeploymentKind, "gitlab-webservice"),
	}

	t.Run("reports a release whose workloads are up", func(t *testing.T) {
		reader := testClient(t, renderedWorkload(t, "gitlab-webservice", 2, 2))

		ready, pending, err := WorkloadsReady(ctx, reader, nil, testNamespace, rendered)

		require.NoError(t, err)
		assert.True(t, ready)
		assert.Empty(t, pending)
	})

	t.Run("names the workload that is not up", func(t *testing.T) {
		reader := testClient(t, renderedWorkload(t, "gitlab-webservice", 2, 1))

		ready, pending, err := WorkloadsReady(ctx, reader, nil, testNamespace, rendered)

		require.NoError(t, err)
		assert.False(t, ready)
		assert.Equal(t, "Deployment gitlab-webservice", pending)
	})

	t.Run("treats a workload that does not exist yet as not up", func(t *testing.T) {
		ready, pending, err := WorkloadsReady(ctx, testClient(t), nil, testNamespace, rendered)

		require.NoError(t, err)
		assert.False(t, ready)
		assert.Equal(t, "Deployment gitlab-webservice", pending)
	})

	t.Run("reads a workload in the namespace it names", func(t *testing.T) {
		workload := renderedWorkload(t, "gitlab-webservice", 1, 1)
		workload.SetNamespace("other-namespace")

		reader := testClient(t, workload)

		wanted := renderedObject("apps/v1", DeploymentKind, "gitlab-webservice")
		wanted.SetNamespace("other-namespace")

		ready, _, err := WorkloadsReady(ctx, reader, nil, testNamespace, []*unstructured.Unstructured{wanted})

		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("is ready with nothing to wait for", func(t *testing.T) {
		ready, pending, err := WorkloadsReady(ctx, testClient(t), nil, testNamespace, nil)

		require.NoError(t, err)
		assert.True(t, ready)
		assert.Empty(t, pending)
	})

	t.Run("gates on the subset it is given", func(t *testing.T) {
		// A zero-downtime upgrade waits on the workloads it rolled out, and a
		// reconciler leaves out a component with no real readiness probe.
		reader := testClient(t,
			renderedWorkload(t, "gitlab-webservice", 1, 1),
			renderedWorkload(t, "gitlab-sidekiq", 1, 0),
		)

		ready, _, err := WorkloadsReady(ctx, reader, nil, testNamespace,
			[]*unstructured.Unstructured{renderedObject("apps/v1", DeploymentKind, "gitlab-webservice")})

		require.NoError(t, err)
		assert.True(t, ready, "the excluded workload must not decide the answer")
	})
}

func TestWorkloadReady(t *testing.T) {
	// The desired count comes from the live object rather than the rendered one,
	// so a workload an autoscaler scaled is measured against what the cluster
	// wants.
	t.Run("treats an absent replica count as one", func(t *testing.T) {
		workload := renderedWorkload(t, "gitlab-webservice", 1, 1)
		unstructured.RemoveNestedField(workload.Object, "spec", "replicas")

		ready, err := WorkloadReady(workload)

		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("treats a workload scaled to zero as up", func(t *testing.T) {
		ready, err := WorkloadReady(renderedWorkload(t, "gitlab-webservice", 0, 0))

		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("is not up until the controller has observed the current spec", func(t *testing.T) {
		// Before that the status counts describe the previous template.
		workload := renderedWorkload(t, "gitlab-webservice", 1, 1)
		workload.SetGeneration(observedGenerationSentinel + 1)

		ready, err := WorkloadReady(workload)

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("is not up while an old replica lingers", func(t *testing.T) {
		// This is the case readiness alone misses: right after a paused
		// Deployment is unpaused the old pods are still ready, so a ready-only
		// check would pass before the new pods roll out.
		workload := renderedWorkload(t, "gitlab-webservice", 2, 2)
		require.NoError(t, unstructured.SetNestedField(workload.Object, int64(3), "status", "replicas"))

		ready, err := WorkloadReady(workload)

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("is not up while a replica is not updated to the new template", func(t *testing.T) {
		workload := renderedWorkload(t, "gitlab-webservice", 2, 2)
		require.NoError(t, unstructured.SetNestedField(workload.Object, int64(1), "status", "updatedReplicas"))

		ready, err := WorkloadReady(workload)

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("is not up while a replica is unavailable", func(t *testing.T) {
		workload := renderedWorkload(t, "gitlab-webservice", 2, 2)
		require.NoError(t, unstructured.SetNestedField(workload.Object, int64(1), "status", "availableReplicas"))

		ready, err := WorkloadReady(workload)

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("treats an absent status count as zero, the way the controllers do", func(t *testing.T) {
		workload := renderedObject("apps/v1", DeploymentKind, "gitlab-webservice")
		require.NoError(t, unstructured.SetNestedField(workload.Object, int64(1), "spec", "replicas"))

		ready, err := WorkloadReady(workload)

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("reports a replica count of the wrong type", func(t *testing.T) {
		workload := renderedObject("apps/v1", DeploymentKind, "gitlab-webservice")
		require.NoError(t, unstructured.SetNestedField(workload.Object, "two", "spec", "replicas"))

		_, err := WorkloadReady(workload)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading the replicas of Deployment")
	})

	t.Run("reports a status count of the wrong type", func(t *testing.T) {
		workload := renderedObject("apps/v1", DeploymentKind, "gitlab-webservice")
		require.NoError(t, unstructured.SetNestedField(workload.Object, int64(1), "spec", "replicas"))
		require.NoError(t, unstructured.SetNestedField(workload.Object, "many", "status", "updatedReplicas"))

		_, err := WorkloadReady(workload)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading status.updatedReplicas of Deployment")
	})
}

func TestWorkloadKinds(t *testing.T) {
	t.Run("covers the kinds that carry replicas", func(t *testing.T) {
		assert.Equal(t, []string{DeploymentKind, "StatefulSet"}, WorkloadKinds)
	})
}

// The Available condition of a release says "waiting for Deployment X to become
// ready" whether X is rolling out or will never start, so a pod that names a
// cause is what tells the two apart.
func TestPendingReason(t *testing.T) {
	ctx := context.Background()

	selector := map[string]string{"app": "webservice"}

	workload := renderedWorkload(t, "gitlab-webservice", 1, 0)
	require.NoError(t, unstructured.SetNestedStringMap(
		workload.Object, selector, "spec", "selector", "matchLabels"))

	t.Run("names the workload alone without a pod reader", func(t *testing.T) {
		assert.Equal(t, "Deployment gitlab-webservice",
			PendingReason(ctx, nil, workload, testNamespace))
	})

	t.Run("names the workload alone when no pod reports a cause", func(t *testing.T) {
		reader := testClient(t, waitingPod("gitlab-webservice-ready", selector, "", ""))

		assert.Equal(t, "Deployment gitlab-webservice",
			PendingReason(ctx, reader, workload, testNamespace))
	})

	t.Run("names the reason a container is stuck", func(t *testing.T) {
		reader := testClient(t,
			waitingPod("gitlab-webservice-abc", selector, "", "ImagePullBackOff"))

		assert.Equal(t,
			"Deployment gitlab-webservice (pod gitlab-webservice-abc: container webservice is waiting: ImagePullBackOff)",
			PendingReason(ctx, reader, workload, testNamespace))
	})

	t.Run("prefers an init container, which a pod never gets past", func(t *testing.T) {
		reader := testClient(t,
			waitingPod("gitlab-webservice-abc", selector, "InvalidImageName", "PodInitializing"))

		assert.Equal(t,
			"Deployment gitlab-webservice (pod gitlab-webservice-abc: init container certificates is waiting: InvalidImageName)",
			PendingReason(ctx, reader, workload, testNamespace))
	})

	t.Run("ignores the reasons a pod passes through on its way up", func(t *testing.T) {
		// Reporting these would name a normal rollout as a problem.
		for _, reason := range []string{"ContainerCreating", "PodInitializing"} {
			reader := testClient(t, waitingPod("gitlab-webservice-abc", selector, "", reason))

			assert.Equal(t, "Deployment gitlab-webservice",
				PendingReason(ctx, reader, workload, testNamespace), reason)
		}
	})

	t.Run("names why a pod is not scheduled, which has no container status", func(t *testing.T) {
		reader := testClient(t, unschedulablePod("gitlab-webservice-abc", selector))

		assert.Equal(t,
			"Deployment gitlab-webservice (pod gitlab-webservice-abc is not scheduled: Unschedulable: 0/1 nodes are available: insufficient memory)",
			PendingReason(ctx, reader, workload, testNamespace))
	})

	t.Run("names the workload alone when it selects nothing", func(t *testing.T) {
		bare := renderedWorkload(t, "gitlab-webservice", 1, 0)
		reader := testClient(t,
			waitingPod("gitlab-webservice-abc", selector, "", "ImagePullBackOff"))

		assert.Equal(t, "Deployment gitlab-webservice",
			PendingReason(ctx, reader, bare, testNamespace))
	})

	t.Run("reaches the condition through WorkloadsReady", func(t *testing.T) {
		reader := testClient(t, workload,
			waitingPod("gitlab-webservice-abc", selector, "", "CrashLoopBackOff"))

		ready, pending, err := WorkloadsReady(ctx, reader, reader, testNamespace,
			[]*unstructured.Unstructured{renderedObject("apps/v1", DeploymentKind, "gitlab-webservice")})

		require.NoError(t, err)
		assert.False(t, ready)
		assert.Equal(t,
			"Deployment gitlab-webservice (pod gitlab-webservice-abc: container webservice is waiting: CrashLoopBackOff)",
			pending)
	})

	t.Run("says nothing about pods for a workload that does not exist yet", func(t *testing.T) {
		// There is nothing to ask, and a stale pod of a previous release must not
		// be reported as the cause.
		reader := testClient(t, waitingPod("gitlab-webservice-abc", selector, "", "ImagePullBackOff"))

		ready, pending, err := WorkloadsReady(ctx, reader, reader, testNamespace,
			[]*unstructured.Unstructured{renderedObject("apps/v1", DeploymentKind, "gitlab-webservice")})

		require.NoError(t, err)
		assert.False(t, ready)
		assert.Equal(t, "Deployment gitlab-webservice", pending)
	})
}

// waitingPod builds a scheduled pod whose init container and container wait for
// the given reasons. An empty reason leaves that container running.
func waitingPod(name string, labels map[string]string, initReason, reason string) client.Object {
	status := func(container, waiting string) corev1.ContainerStatus {
		state := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
		if waiting != "" {
			state = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: waiting}}
		}

		return corev1.ContainerStatus{Name: container, State: state}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace, Labels: labels},
		Spec:       corev1.PodSpec{NodeName: "node-1"},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{status("certificates", initReason)},
			ContainerStatuses:     []corev1.ContainerStatus{status("webservice", reason)},
		},
	}
}

// unschedulablePod builds a pod that never landed on a node, so it carries no
// container status at all.
func unschedulablePod(name string, labels map[string]string) client.Object {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace, Labels: labels},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Type:    corev1.PodScheduled,
				Status:  corev1.ConditionFalse,
				Reason:  corev1.PodReasonUnschedulable,
				Message: "0/1 nodes are available: insufficient memory",
			}},
		},
	}
}
