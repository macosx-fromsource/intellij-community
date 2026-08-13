package gitlabcore

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// fieldOwner is the field manager of everything the reconciler applies.
const fieldOwner = client.FieldOwner("gitlabcore-controller")

// What the reconciler never applies.
const (
	// crdKind is a definition, in whichever group it is served.
	crdKind = "CustomResourceDefinition"

	// rbacGroup holds Role, RoleBinding, ClusterRole, and ClusterRoleBinding.
	rbacGroup = "rbac.authorization.k8s.io"
)

// neverApplied matches the objects of a release that the Operator leaves to
// whoever administers the cluster.
//
// Definitions are cluster-wide and outlive every release that uses them, so
// installing one from a namespaced resource would let one GitLab instance change
// an API that another instance, and other operators, depend on.
//
// RBAC grants permissions. Applying it would turn the right to write a
// GitLabCore into the right to grant any permission the Operator holds, which
// is cluster-wide. Provisioning it stays a deliberate act of an administrator.
var neverApplied objects.Predicate = func(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == crdKind || obj.GroupVersionKind().Group == rbacGroup
}

// applyObjects applies the rendered objects of the release, in the order the
// renderer returns them, which is the Helm install order.
//
// What neverApplied matches is skipped, and the definitions of the crds/
// directories are skipped with it: the GitLab chart carries definitions in both
// places, the Gateway API and Envoy Gateway ones from templates and the rest
// from crds/.
//
// An object that needs an API the cluster does not serve therefore fails to
// apply, with the kind named in the error; enable such a component only on a
// cluster that already serves its API, or turn it off in spec.chart.values. A
// component whose RBAC is missing starts and fails against the API server,
// which no reconcile can repair either.
//
// The hooks go through the same filter, in runHooks.
func (r *Reconciler) applyObjects(ctx context.Context, core *apiv2alpha1.GitLabCore, result *render.Result, log logr.Logger) error {
	skipped, applicable := objects.Partition(result.Objects, neverApplied)

	if len(skipped)+len(result.CRDs) > 0 {
		log.Info("skipping the definitions and the RBAC of the chart, which the Operator never applies",
			"rendered", len(skipped), "bundled definitions", len(result.CRDs),
			"hint", "have the cluster administrator provision them")
	}

	for _, obj := range applicable {
		if err := r.applyObject(ctx, core, obj, log); err != nil {
			return err
		}
	}

	return nil
}

// applyObject applies one rendered object, server-side.
//
// An object in the namespace of the resource gets a controller reference, so
// that Kubernetes garbage-collects it when the resource is deleted. Everything
// else is applied without one, because the API server rejects an owner it
// cannot resolve: a namespaced resource may own neither a cluster-scoped object
// nor an object of another namespace. The chart renders both, for example the
// cert-manager leader election Role in kube-system. Those objects carry the
// release labels the renderer stamps instead, and finalize sweeps them.
//
// Ownership is forced: the release is what the resource says it is, so a field
// another manager took is taken back rather than reported as a conflict.
func (r *Reconciler) applyObject(ctx context.Context, core *apiv2alpha1.GitLabCore, obj *unstructured.Unstructured, log logr.Logger) error {
	target := obj.DeepCopy()

	pruneNulls(target.Object)

	namespaced, err := r.IsObjectNamespaced(target)
	if err != nil {
		return fmt.Errorf("resolving the scope of %s %q: %w", target.GetKind(), target.GetName(), err)
	}

	if namespaced {
		target.SetNamespace(effectiveNamespace(core, target, namespaced))
	}

	if ownable(core, target, namespaced) {
		if err := controllerutil.SetControllerReference(core, target, r.Scheme); err != nil {
			return fmt.Errorf("owning %s %q: %w", target.GetKind(), target.GetName(), err)
		}
	} else {
		log.V(2).Info("applying an object the resource cannot own, to be swept by its release labels",
			"kind", target.GetKind(), "name", target.GetName(), "namespace", target.GetNamespace())
	}

	if err := r.Apply(ctx, client.ApplyConfigurationFromUnstructured(target),
		fieldOwner, client.ForceOwnership); err != nil {
		return fmt.Errorf("applying %s %q: %w", target.GetKind(), target.GetName(), err)
	}

	log.V(2).Info("applied object",
		"kind", target.GetKind(), "name", target.GetName(), "namespace", target.GetNamespace())

	return nil
}

// pruneNulls removes the explicit nulls of a rendered object, in place.
//
// Chart templates emit them and the renderer preserves them. Under server-side
// apply each one becomes a field this manager owns and declares empty, which
// keeps the server from defaulting it.
//
// Only mapping entries go. A null inside a list stays, because dropping one
// renumbers the list.
func pruneNulls(object map[string]any) {
	for key, value := range object {
		switch typed := value.(type) {
		case nil:
			delete(object, key)
		case map[string]any:
			pruneNulls(typed)
		case []any:
			pruneNullsInList(typed)
		}
	}
}

func pruneNullsInList(list []any) {
	for _, value := range list {
		switch typed := value.(type) {
		case map[string]any:
			pruneNulls(typed)
		case []any:
			pruneNullsInList(typed)
		}
	}
}

// effectiveNamespace returns the namespace a rendered object is applied in, and
// the empty string for a cluster-scoped one.
//
// The renderer stamps no namespace: the GitLab chart sets it on every namespaced
// object of its own, but the dependency charts leave it unset, and such an object
// would be created beside the Operator. It falls back to the namespace of the
// resource, the way Helm falls back to the namespace of the release.
func effectiveNamespace(core *apiv2alpha1.GitLabCore, obj *unstructured.Unstructured, namespaced bool) string {
	if !namespaced {
		return ""
	}

	if namespace := obj.GetNamespace(); namespace != "" {
		return namespace
	}

	return core.Namespace
}

// ownable reports whether the resource can be the owner of the object, which
// only holds inside its own namespace. It decides between the two ways an object
// of a release is cleaned up: garbage collection, or the sweep of finalize.
func ownable(core *apiv2alpha1.GitLabCore, obj *unstructured.Unstructured, namespaced bool) bool {
	return namespaced && effectiveNamespace(core, obj, namespaced) == core.Namespace
}
