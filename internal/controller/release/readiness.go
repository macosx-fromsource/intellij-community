package release

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentKind is the workload kind a caller most often filters a release down
// to.
const DeploymentKind = "Deployment"

// WorkloadKinds are the workload kinds whose readiness decides whether a release
// is up.
var WorkloadKinds = []string{DeploymentKind, "StatefulSet"}

// WorkloadsReady reports whether every workload in the given set has its desired
// replicas ready, describing the first one that does not.
//
// The desired count comes from the live object rather than the rendered one, so
// that a Deployment an autoscaler or an administrator scaled is measured against
// what the cluster wants. A workload that is not created yet counts as not ready.
//
// The set is a parameter rather than the whole release, so a caller can gate on a
// subset: a zero-downtime upgrade waits on the workloads it rolled out, and a
// reconciler whose release includes a component with no real readiness probe
// leaves that component out.
//
// pods is asked why a workload is not ready, and only then. Pass an uncached
// reader: nothing in the Operator watches pods, so a cached read would start an
// informer for every pod in scope. Pass nil to describe a workload by name alone,
// which is what a caller without the permission to list pods does.
func WorkloadsReady(ctx context.Context, reader client.Reader, pods client.Reader, defaultNamespace string, workloads []*unstructured.Unstructured) (bool, string, error) {
	for _, workload := range workloads {
		namespace := workload.GetNamespace()
		if namespace == "" {
			namespace = defaultNamespace
		}

		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(workload.GroupVersionKind())

		err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: workload.GetName()}, live)

		switch {
		case apierrors.IsNotFound(err):
			// Nothing was created, so there are no pods to ask.
			return false, fmt.Sprintf("%s %s", workload.GetKind(), workload.GetName()), nil
		case err != nil:
			return false, "", err
		}

		ready, err := WorkloadReady(live)
		if err != nil {
			return false, "", err
		}

		if !ready {
			return false, PendingReason(ctx, pods, live, namespace), nil
		}
	}

	return true, "", nil
}

// WorkloadReady reports whether a live workload has finished rolling out its
// desired replicas. A replica count the object does not carry is one, the
// Kubernetes default, and a workload scaled to zero is ready by definition.
//
// Readiness alone (status.readyReplicas) is not enough for a zero-downtime
// upgrade: right after a paused Deployment is unpaused, the old pods are still
// ready, so a ready-only check would pass before the new pods roll out and let
// the post-deployment migrations run against the old code. So this applies the
// completion check `kubectl rollout status` uses: the controller has observed the
// current spec (observedGeneration), every desired replica is updated to the new
// template (updatedReplicas), no old replicas linger (status.replicas), and the
// updated replicas are available (availableReplicas).
func WorkloadReady(live *unstructured.Unstructured) (bool, error) {
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

// PendingReason describes why one workload is not ready, in the form
// "Deployment gitlab-webservice" or, when a pod of it names a cause,
// "Deployment gitlab-webservice (pod gitlab-webservice-abc: container webservice
// is waiting: ImagePullBackOff)".
//
// The cause is what a status line without it cannot say. A workload that is
// rolling out, one whose image reference does not parse, and one whose pods
// cannot be scheduled all leave the same "not ready" behind, so an installation
// that will never come up looks like one that is still starting.
func PendingReason(ctx context.Context, pods client.Reader, workload *unstructured.Unstructured, namespace string) string {
	name := fmt.Sprintf("%s %s", workload.GetKind(), workload.GetName())

	if pods == nil {
		return name
	}

	cause := podCause(ctx, pods, workload, namespace)
	if cause == "" {
		return name
	}

	return fmt.Sprintf("%s (%s)", name, cause)
}

// podCause returns the cause the first pod of a workload reports, or an empty
// string while every pod is either fine or only transiently unsettled.
//
// It is best effort throughout. The reader is asked for pods only when a workload
// is already known not to be ready, and every failure to read them is swallowed:
// this decorates a status message, so it must never turn a wait into an error.
func podCause(ctx context.Context, pods client.Reader, workload *unstructured.Unstructured, namespace string) string {
	selector, found, err := unstructured.NestedStringMap(workload.Object, "spec", "selector", "matchLabels")
	if err != nil || !found || len(selector) == 0 {
		return ""
	}

	list := &corev1.PodList{}
	if err := pods.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return ""
	}

	for index := range list.Items {
		if cause := containerCause(&list.Items[index]); cause != "" {
			return cause
		}

		if cause := unschedulableCause(&list.Items[index]); cause != "" {
			return cause
		}
	}

	return ""
}

// transientWaitingReasons are the waiting reasons a pod passes through on its way
// up. Reporting one would name a normal rollout as a problem, which is the noise
// this whole path exists to avoid.
var transientWaitingReasons = map[string]bool{
	"":                  true,
	"ContainerCreating": true,
	"PodInitializing":   true,
}

// containerCause returns the first container of a pod that is stuck, init
// containers first because a pod never reaches its main containers while one of
// them is.
func containerCause(pod *corev1.Pod) string {
	groups := []struct {
		kind     string
		statuses []corev1.ContainerStatus
	}{
		{"init container", pod.Status.InitContainerStatuses},
		{"container", pod.Status.ContainerStatuses},
	}

	for _, group := range groups {
		for _, status := range group.statuses {
			waiting := status.State.Waiting
			if waiting == nil || transientWaitingReasons[waiting.Reason] {
				continue
			}

			return fmt.Sprintf("pod %s: %s %s is waiting: %s",
				pod.Name, group.kind, status.Name, waiting.Reason)
		}
	}

	return ""
}

// unschedulableCause returns why a pod has not been scheduled, which no container
// status covers because a pod that never lands on a node has none.
func unschedulableCause(pod *corev1.Pod) string {
	if pod.Spec.NodeName != "" {
		return ""
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type != corev1.PodScheduled || condition.Status != corev1.ConditionFalse {
			continue
		}

		if condition.Reason == "" {
			continue
		}

		if condition.Message == "" {
			return fmt.Sprintf("pod %s is not scheduled: %s", pod.Name, condition.Reason)
		}

		return fmt.Sprintf("pod %s is not scheduled: %s: %s", pod.Name, condition.Reason, condition.Message)
	}

	return ""
}
