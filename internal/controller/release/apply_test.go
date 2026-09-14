package release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestApply(t *testing.T) {
	ctx := context.Background()

	t.Run("owns an object in the namespace of the owner", func(t *testing.T) {
		applier := testApplier(t)
		owner := testOwner(testNamespace)

		configMap := renderedObject("v1", "ConfigMap", "gitlab-configmap")

		require.NoError(t, applier.Apply(ctx, owner,
			[]*unstructured.Unstructured{configMap}, testLogger(t)))

		created := &corev1.ConfigMap{}
		require.NoError(t, applier.Client.Get(ctx,
			types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created))

		// Garbage collection is what removes it, so the reference has to be there.
		require.Len(t, created.OwnerReferences, 1)
		assert.Equal(t, owner.GetName(), created.OwnerReferences[0].Name)
		assert.True(t, *created.OwnerReferences[0].Controller)
	})

	t.Run("falls back to the namespace of the owner", func(t *testing.T) {
		// The renderer stamps no namespace, and a dependency chart leaves it
		// unset, so such an object would otherwise be created beside the
		// Operator rather than beside the release.
		applier := testApplier(t)

		configMap := renderedObject("v1", "ConfigMap", "gitlab-configmap")
		require.Empty(t, configMap.GetNamespace())

		require.NoError(t, applier.Apply(ctx, testOwner(testNamespace),
			[]*unstructured.Unstructured{configMap}, testLogger(t)))

		created := &corev1.ConfigMap{}
		assert.NoError(t, applier.Client.Get(ctx,
			types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created))
	})

	t.Run("records the field owner it was given", func(t *testing.T) {
		applier := testApplier(t)

		require.NoError(t, applier.Apply(ctx, testOwner(testNamespace),
			[]*unstructured.Unstructured{renderedObject("v1", "ConfigMap", "gitlab-configmap")},
			testLogger(t)))

		created := &corev1.ConfigMap{}
		require.NoError(t, applier.Client.Get(ctx,
			types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created))

		require.NotEmpty(t, created.ManagedFields)
		assert.Equal(t, string(applier.FieldOwner), created.ManagedFields[0].Manager)
	})

	t.Run("applies a cluster-scoped object without an owner reference", func(t *testing.T) {
		// The API server rejects an owner it cannot resolve: a namespaced
		// resource may not own a cluster-scoped object.
		applier := testApplier(t)

		webhook := renderedObject(
			"admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "gitlab-webhook")

		require.NoError(t, applier.Apply(ctx, testOwner(testNamespace),
			[]*unstructured.Unstructured{webhook}, testLogger(t)))

		created := &admissionv1.ValidatingWebhookConfiguration{}
		require.NoError(t, applier.Client.Get(ctx,
			types.NamespacedName{Name: "gitlab-webhook"}, created))

		assert.Empty(t, created.Namespace)
		assert.Empty(t, created.OwnerReferences)
	})

	t.Run("applies an object of another namespace without an owner reference", func(t *testing.T) {
		// The chart does this: cert-manager renders its leader election Role
		// into kube-system.
		applier := testApplier(t)

		configMap := renderedObject("v1", "ConfigMap", "gitlab-configmap")
		configMap.SetNamespace(testNamespace)

		require.NoError(t, applier.Apply(ctx, testOwner("other-namespace"),
			[]*unstructured.Unstructured{configMap}, testLogger(t)))

		created := &corev1.ConfigMap{}
		require.NoError(t, applier.Client.Get(ctx,
			types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created))

		assert.Empty(t, created.OwnerReferences)
	})

	t.Run("skips the RBAC of a release", func(t *testing.T) {
		applier := testApplier(t)

		clusterRole := renderedObject("rbac.authorization.k8s.io/v1", "ClusterRole", "gitlab-clusterrole")
		configMap := renderedObject("v1", "ConfigMap", "gitlab-configmap")

		require.NoError(t, applier.Apply(ctx, testOwner(testNamespace),
			[]*unstructured.Unstructured{clusterRole, configMap}, testLogger(t)))

		assert.NoError(t, applier.Client.Get(ctx,
			types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, &corev1.ConfigMap{}))

		err := applier.Client.Get(ctx,
			types.NamespacedName{Name: "gitlab-clusterrole"}, &rbacv1.ClusterRole{})
		assert.True(t, apierrors.IsNotFound(err), "got %v", err)
	})

	t.Run("skips the definitions of a release", func(t *testing.T) {
		// The mapper of the test does not know CustomResourceDefinition, so an
		// attempt to apply one would fail while its scope is resolved.
		// Succeeding is the proof that it was skipped.
		applier := testApplier(t)

		definition := renderedObject("apiextensions.k8s.io/v1", CRDKind, "gateways.gateway.networking.k8s.io")
		configMap := renderedObject("v1", "ConfigMap", "gitlab-configmap")

		require.NoError(t, applier.Apply(ctx, testOwner(testNamespace),
			[]*unstructured.Unstructured{definition, configMap}, testLogger(t)))

		assert.NoError(t, applier.Client.Get(ctx,
			types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, &corev1.ConfigMap{}))
	})

	t.Run("reports a kind whose scope cannot be resolved", func(t *testing.T) {
		applier := testApplier(t)

		unknown := renderedObject("example.com/v1", "Widget", "gitlab-widget")

		err := applier.Apply(ctx, testOwner(testNamespace),
			[]*unstructured.Unstructured{unknown}, testLogger(t))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolving the scope of Widget")
	})

	t.Run("leaves the rendered object untouched", func(t *testing.T) {
		// The objects come from the informer cache by way of the renderer, so
		// mutating one in place would leak into the next pass.
		applier := testApplier(t)

		configMap := renderedObject("v1", "ConfigMap", "gitlab-configmap")
		before := configMap.DeepCopy()

		require.NoError(t, applier.Apply(ctx, testOwner(testNamespace),
			[]*unstructured.Unstructured{configMap}, testLogger(t)))

		assert.Equal(t, before, configMap)
	})
}

func TestNeverApplied(t *testing.T) {
	cases := []struct {
		name       string
		apiVersion string
		kind       string
		want       bool
	}{
		{"a definition", "apiextensions.k8s.io/v1", CRDKind, true},
		{"a definition in another group", "example.com/v1", CRDKind, true},
		{"a ClusterRole", "rbac.authorization.k8s.io/v1", "ClusterRole", true},
		{"a Role", "rbac.authorization.k8s.io/v1", "Role", true},
		{"a RoleBinding", "rbac.authorization.k8s.io/v1", "RoleBinding", true},
		// A ServiceAccount grants nothing by itself, so it is applied.
		{"a ServiceAccount", "v1", "ServiceAccount", false},
		{"a ConfigMap", "v1", "ConfigMap", false},
		{"a Deployment", "apps/v1", "Deployment", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want,
				NeverApplied(renderedObject(testCase.apiVersion, testCase.kind, "object")))
		})
	}
}

func TestPruneNulls(t *testing.T) {
	t.Run("removes a null mapping entry at any depth", func(t *testing.T) {
		// Under server-side apply an explicit null becomes a field this manager
		// owns and declares empty, which keeps the server from defaulting it.
		object := map[string]any{
			"top": nil,
			"spec": map[string]any{
				"kept":   "value",
				"absent": nil,
				"nested": map[string]any{"absent": nil},
			},
		}

		pruneNulls(object)

		assert.Equal(t, map[string]any{
			"spec": map[string]any{
				"kept":   "value",
				"nested": map[string]any{},
			},
		}, object)
	})

	t.Run("keeps a null inside a list, which renumbering would break", func(t *testing.T) {
		object := map[string]any{"list": []any{nil, map[string]any{"absent": nil, "kept": "value"}}}

		pruneNulls(object)

		assert.Equal(t, map[string]any{
			"list": []any{nil, map[string]any{"kept": "value"}},
		}, object)
	})

	t.Run("descends into a nested list", func(t *testing.T) {
		object := map[string]any{"outer": []any{[]any{map[string]any{"absent": nil, "kept": 1}}}}

		pruneNulls(object)

		assert.Equal(t, map[string]any{
			"outer": []any{[]any{map[string]any{"kept": 1}}},
		}, object)
	})
}

func TestEffectiveNamespace(t *testing.T) {
	owner := testOwner(testNamespace)

	t.Run("is empty for a cluster-scoped object", func(t *testing.T) {
		obj := renderedObject("v1", "ConfigMap", "object")
		obj.SetNamespace("ignored")

		assert.Empty(t, EffectiveNamespace(owner, obj, false))
	})

	t.Run("keeps the namespace the object names", func(t *testing.T) {
		obj := renderedObject("v1", "ConfigMap", "object")
		obj.SetNamespace("kube-system")

		assert.Equal(t, "kube-system", EffectiveNamespace(owner, obj, true))
	})

	t.Run("falls back to the namespace of the owner", func(t *testing.T) {
		assert.Equal(t, testNamespace,
			EffectiveNamespace(owner, renderedObject("v1", "ConfigMap", "object"), true))
	})
}

func TestOwnable(t *testing.T) {
	owner := testOwner(testNamespace)

	t.Run("is true only inside the namespace of the owner", func(t *testing.T) {
		assert.True(t, Ownable(owner, renderedObject("v1", "ConfigMap", "object"), true))
	})

	t.Run("is false for a cluster-scoped object", func(t *testing.T) {
		assert.False(t, Ownable(owner, renderedObject("v1", "ConfigMap", "object"), false))
	})

	t.Run("is false for an object of another namespace", func(t *testing.T) {
		obj := renderedObject("v1", "ConfigMap", "object")
		obj.SetNamespace("kube-system")

		assert.False(t, Ownable(owner, obj, true))
	})
}
