package release

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

func TestUnownedTargets(t *testing.T) {
	sweeper := &Sweeper{Client: testClient(t)}

	t.Run("targets a cluster-scoped object, which cannot be owned", func(t *testing.T) {
		result := &render.Result{Objects: []*unstructured.Unstructured{
			renderedObject("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "gitlab-webhook"),
		}}

		targets := sweeper.UnownedTargets(testOwner(testNamespace), result, testLogger(t))

		assert.Equal(t, []SweepTarget{{
			GVK: admissionv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration"),
		}}, targets)
	})

	t.Run("targets an object of another namespace", func(t *testing.T) {
		configMap := renderedObject("v1", "ConfigMap", "gitlab-configmap")
		configMap.SetNamespace(testNamespace)

		result := &render.Result{Objects: []*unstructured.Unstructured{configMap}}

		targets := sweeper.UnownedTargets(testOwner("other-namespace"), result, testLogger(t))

		assert.Equal(t, []SweepTarget{{
			GVK:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
			Namespace: testNamespace,
		}}, targets)
	})

	t.Run("leaves an owned object to garbage collection", func(t *testing.T) {
		result := &render.Result{Objects: []*unstructured.Unstructured{
			renderedObject("v1", "ConfigMap", "gitlab-configmap"),
		}}

		assert.Empty(t, sweeper.UnownedTargets(testOwner(testNamespace), result, testLogger(t)))
	})

	t.Run("does not sweep what was never applied", func(t *testing.T) {
		result := &render.Result{Objects: []*unstructured.Unstructured{
			renderedObject("rbac.authorization.k8s.io/v1", "ClusterRole", "gitlab-clusterrole"),
			renderedObject("apiextensions.k8s.io/v1", CRDKind, "gateways.gateway.networking.k8s.io"),
		}}

		assert.Empty(t, sweeper.UnownedTargets(testOwner(testNamespace), result, testLogger(t)))
	})

	t.Run("includes a hook, which a delete policy may have left in place", func(t *testing.T) {
		hook := renderedObject("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "gitlab-hook")

		result := &render.Result{Hooks: []render.Hook{{Object: hook}}}

		targets := sweeper.UnownedTargets(testOwner(testNamespace), result, testLogger(t))

		assert.Equal(t, []SweepTarget{{
			GVK: admissionv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration"),
		}}, targets)
	})

	t.Run("reports each kind once", func(t *testing.T) {
		result := &render.Result{Objects: []*unstructured.Unstructured{
			renderedObject("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "one"),
			renderedObject("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "two"),
		}}

		assert.Len(t, sweeper.UnownedTargets(testOwner(testNamespace), result, testLogger(t)), 1)
	})

	t.Run("skips a kind the cluster does not serve", func(t *testing.T) {
		// Nothing of that kind exists, so there is nothing to sweep, and a
		// deletion must not be blocked by it.
		result := &render.Result{Objects: []*unstructured.Unstructured{
			renderedObject("example.com/v1", "Widget", "gitlab-widget"),
		}}

		assert.Empty(t, sweeper.UnownedTargets(testOwner(testNamespace), result, testLogger(t)))
	})
}

func TestSweep(t *testing.T) {
	ctx := context.Background()

	// The label selector the sweep matches on, as the renderer stamps it.
	labels := map[string]string{
		render.ReleaseNameLabel:      releaseName,
		render.ReleaseNamespaceLabel: testNamespace,
	}

	t.Run("deletes an unowned object of the release", func(t *testing.T) {
		live := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "gitlab-webhook", Labels: labels},
		}

		sweeper := &Sweeper{Client: testClient(t, live)}

		result := &render.Result{Objects: []*unstructured.Unstructured{
			renderedObject("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "gitlab-webhook"),
		}}

		sweeper.Sweep(ctx, testOwner(testNamespace), func() (*render.Result, error) {
			return result, nil
		}, testLogger(t))

		err := sweeper.Client.Get(ctx,
			types.NamespacedName{Name: "gitlab-webhook"}, &admissionv1.ValidatingWebhookConfiguration{})
		assert.True(t, apierrors.IsNotFound(err), "got %v", err)
	})

	t.Run("leaves an object of another release alone", func(t *testing.T) {
		other := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-webhook",
				Labels: map[string]string{
					render.ReleaseNameLabel:      "other",
					render.ReleaseNamespaceLabel: testNamespace,
				},
			},
		}

		sweeper := &Sweeper{Client: testClient(t, other)}

		result := &render.Result{Objects: []*unstructured.Unstructured{
			renderedObject("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "gitlab-webhook"),
		}}

		sweeper.Sweep(ctx, testOwner(testNamespace), func() (*render.Result, error) {
			return result, nil
		}, testLogger(t))

		assert.NoError(t, sweeper.Client.Get(ctx,
			types.NamespacedName{Name: "other-webhook"}, &admissionv1.ValidatingWebhookConfiguration{}))
	})

	t.Run("a render it cannot repeat does not block the deletion", func(t *testing.T) {
		// A chart archive the Operator no longer carries, or values that no
		// longer render, must not leave behind a resource that cannot be
		// deleted. The objects are left behind and logged instead.
		sweeper := &Sweeper{Client: testClient(t)}

		sweeper.Sweep(ctx, testOwner(testNamespace), func() (*render.Result, error) {
			return nil, errors.New("the chart is gone")
		}, testLogger(t))
	})
}
