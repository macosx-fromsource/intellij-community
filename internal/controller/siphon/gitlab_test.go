package siphon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveGitLab(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the application version the instance publishes", func(t *testing.T) {
		// The instance reads it off its own render and publishes it. Deriving it
		// from the chart version here would be wrong: the two are unrelated
		// numbers, and the chart behind an instance is not necessarily one this
		// Operator carries.
		reconciler := testReconciler(t, availableGitLab(testGitLabVersion))

		version, err := reconciler.resolveGitLab(ctx, newSiphon())

		require.NoError(t, err)
		assert.Equal(t, testGitLabVersion, version)
		assert.NotEqual(t, testGitLabChartVersion, version, "the application version is not the chart version")
	})

	t.Run("waits for an instance that is not available yet", func(t *testing.T) {
		core := availableGitLab(testGitLabVersion)
		apimeta.SetStatusCondition(&core.Status.Conditions, metav1.Condition{
			Type:   gitLabConditionAvailable,
			Status: metav1.ConditionFalse,
			Reason: "WorkloadsNotReady",
		})
		core.Status.Phase = "Preparing"

		reconciler := testReconciler(t, core)

		_, err := reconciler.resolveGitLab(ctx, newSiphon())

		require.Error(t, err)

		// A wait, not a failure, so the reconciler can requeue rather than give
		// up.
		var notReady *notReadyError
		require.ErrorAs(t, err, &notReady)
		assert.Contains(t, err.Error(), "Preparing")
	})

	t.Run("reports an instance that does not exist", func(t *testing.T) {
		reconciler := testReconciler(t)

		_, err := reconciler.resolveGitLab(ctx, newSiphon())

		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err), "got %v", err)
	})

	t.Run("waits for an instance that has not published a version yet", func(t *testing.T) {
		// An Operator just upgraded to a version that publishes the field sees
		// this against an instance that has not reconciled since. The instance
		// fills it in on a pass of its own, so this waits rather than fails.
		reconciler := testReconciler(t, availableGitLab(""))

		_, err := reconciler.resolveGitLab(ctx, newSiphon())

		require.Error(t, err)

		var pending *versionPendingError
		require.ErrorAs(t, err, &pending)
		assert.Contains(t, err.Error(), "the version of GitLab it deploys")
	})

	t.Run("resolves the instance in the namespace of the resource", func(t *testing.T) {
		// The reference is namespace-local, so a namespace can hold more than one
		// instance and each Siphon names its own.
		core := availableGitLab(testGitLabVersion)
		core.Namespace = "other-namespace"

		reconciler := testReconciler(t, core)

		_, err := reconciler.resolveGitLab(ctx, newSiphon())

		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err), "got %v", err)
	})
}

func TestNotReadyError(t *testing.T) {
	t.Run("names the instance and its phase", func(t *testing.T) {
		err := &notReadyError{name: "gitlab", phase: "Preparing"}

		assert.Contains(t, err.Error(), `"gitlab"`)
		assert.Contains(t, err.Error(), "Preparing")
	})

	t.Run("says unknown for an instance with no phase yet", func(t *testing.T) {
		err := &notReadyError{name: "gitlab"}

		assert.Contains(t, err.Error(), "unknown")
	})
}

func TestVersionPendingError(t *testing.T) {
	t.Run("names the instance", func(t *testing.T) {
		err := &versionPendingError{name: "gitlab"}

		assert.Contains(t, err.Error(), `"gitlab"`)
	})
}
