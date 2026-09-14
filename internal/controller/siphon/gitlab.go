package siphon

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// gitLabConditionAvailable is the condition of a GitLabCore that reports its
// workloads are up. Siphon waits for it: the ClickHouse target tables are created
// by the migrations of the instance, and the producer snapshots schemas those
// migrations create.
const gitLabConditionAvailable = "Available"

// resolveGitLab reads the referenced instance and returns the application version
// the table definitions are pinned to.
//
// It is a wait, not a hard requirement: an instance that is not available yet is
// reported as such and the reconcile comes back. What must never happen is the
// reverse dependency. Nothing in the readiness of a GitLabCore may depend on a
// Siphon, or the two deadlock, each waiting for the other. As peers under ADR 24
// that holds by construction, and it is why Siphon waits in its own reconcile
// rather than the instance gating on it.
//
// The version comes from the status of the instance and nowhere else. Deriving it
// from the chart version instead would be wrong twice over: the two are unrelated
// numbers, and the chart a GitLabCore renders is not necessarily one this Operator
// carries, because it may have been pulled at reconcile time. The instance reads
// the version off its own render and publishes it, so this only has to read it.
func (r *Reconciler) resolveGitLab(ctx context.Context, siphon *apiv2alpha1.Siphon) (string, error) {
	core := &apiv2alpha1.GitLabCore{}

	key := types.NamespacedName{Namespace: siphon.Namespace, Name: siphon.Spec.GitLabRef.Name}
	if err := r.Get(ctx, key, core); err != nil {
		return "", err
	}

	if !apimeta.IsStatusConditionTrue(core.Status.Conditions, gitLabConditionAvailable) {
		return "", &notReadyError{name: core.Name, phase: core.Status.Phase}
	}

	if core.Status.GitLabVersion == "" {
		return "", &versionPendingError{name: core.Name}
	}

	return core.Status.GitLabVersion, nil
}

// notReadyError reports a referenced instance that is not available yet. It is a
// type of its own so the reconciler can wait rather than fail.
type notReadyError struct {
	name  string
	phase string
}

func (e *notReadyError) Error() string {
	phase := e.phase
	if phase == "" {
		phase = "unknown"
	}

	return fmt.Sprintf("waiting for the GitLabCore %q to become available, its phase is %s", e.name, phase)
}

// versionPendingError reports a referenced instance that is available but has not
// published the version of the application it deploys. It is a wait too, and for
// the same reason: the instance records it on a pass of its own, which this
// reconcile cannot order itself against.
//
// It is reachable in practice on an Operator that has just been upgraded to a
// version that publishes the field, against an instance that has not reconciled
// since, and would be permanent if it were treated as a failure.
type versionPendingError struct {
	name string
}

func (e *versionPendingError) Error() string {
	return fmt.Sprintf(
		"waiting for the GitLabCore %q to report the version of GitLab it deploys, so the table definitions can be pinned",
		e.name)
}
