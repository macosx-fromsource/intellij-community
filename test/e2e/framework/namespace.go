package framework

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceLabel marks every namespace the harness creates, so that a run that
// was killed can be swept with one command.
const NamespaceLabel = "e2e.apps.gitlab.com/run"

// CreateNamespace creates a namespace for the suite and deletes it afterwards.
//
// Deletion is registered before anything else runs in the namespace, and the
// diagnostics dump is registered after, so that cleanup functions unwind in the
// order that matters: dump first, then delete.
func CreateNamespace(ctx context.Context, t *testing.T, kubeClient client.Client, name, runID string) {
	t.Helper()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{NamespaceLabel: runID},
		},
	}

	if err := kubeClient.Create(ctx, namespace); err != nil {
		t.Fatalf("creating the namespace %s: %v", name, err)
	}
}

// DeleteNamespace deletes the namespace and waits for it to go away, so that a
// rerun against the same cluster does not collide with a namespace still
// terminating.
func DeleteNamespace(t *testing.T, kubeClient client.Client, name string) {
	t.Helper()

	// A context of its own: the one of the suite may already be cancelled.
	ctx := context.Background()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := kubeClient.Delete(ctx, namespace); err != nil && !apierrors.IsNotFound(err) {
		t.Errorf("deleting the namespace %s: %v", name, err)

		return
	}

	Eventually(ctx, t, MediumTimeout, fmt.Sprintf("the namespace %s is gone", name),
		func(ctx context.Context) (bool, string, error) {
			live := &corev1.Namespace{}

			err := kubeClient.Get(ctx, client.ObjectKey{Name: name}, live)
			if apierrors.IsNotFound(err) {
				return true, "", nil
			}

			if err != nil {
				return false, "", err
			}

			return false, reasonf("its phase is %s", live.Status.Phase), nil
		})
}

// RunID is a short random token that distinguishes one run from another.
func RunID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reading random bytes for the run identifier: %w", err)
	}

	return hex.EncodeToString(buf), nil
}
