/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package release holds the Helm-release mechanics the v2alpha1 reconcilers
// share: applying what internal/render produced, deciding whether the workloads
// of a release are up, sweeping what garbage collection cannot reach, and
// merging the free-form chart values over the derived ones.
//
// Every entry point takes the owning resource as a client.Object, so nothing
// here knows which kind it is serving. What stays with each reconciler is the
// vocabulary: the condition types, the phases, the reasons, and the shape of its
// own reconcile loop.
package release

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// What a reconciler never applies.
const (
	// CRDKind is a definition, in whichever group it is served.
	CRDKind = "CustomResourceDefinition"

	// rbacGroup holds Role, RoleBinding, ClusterRole, and ClusterRoleBinding.
	rbacGroup = "rbac.authorization.k8s.io"
)

// NeverApplied matches the objects of a release that the Operator leaves to
// whoever administers the cluster.
//
// Definitions are cluster-wide and outlive every release that uses them, so
// installing one from a namespaced resource would let one instance change an API
// that another instance, and other operators, depend on.
//
// RBAC grants permissions. Applying it would turn the right to write one of
// these resources into the right to grant any permission the Operator holds,
// which is cluster-wide. Provisioning it stays a deliberate act of an
// administrator.
var NeverApplied objects.Predicate = func(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == CRDKind || obj.GroupVersionKind().Group == rbacGroup
}

// Applier writes rendered objects to the cluster on behalf of one resource.
//
// The client is a named field rather than an embedded one: client.Client has an
// Apply method of its own, and embedding it would make Applier.Apply shadow the
// very call it makes.
type Applier struct {
	// Client is the cluster the objects are applied to.
	Client client.Client

	// Scheme resolves the owner into an owner reference.
	Scheme *runtime.Scheme

	// FieldOwner is the field manager of everything applied. Give each
	// reconciler one of its own, so server-side apply attributes a field to the
	// reconciler that wrote it.
	FieldOwner client.FieldOwner
}

// Apply applies a set of rendered objects in the order given, which for a whole
// release is the Helm install order. What NeverApplied matches is skipped.
func (a *Applier) Apply(ctx context.Context, owner client.Object, objs []*unstructured.Unstructured, log logr.Logger) error {
	for _, obj := range objs {
		if NeverApplied(obj) {
			continue
		}

		if err := a.ApplyOne(ctx, owner, obj, log); err != nil {
			return err
		}
	}

	return nil
}

// ApplyOne applies one rendered object, server-side.
//
// An object in the namespace of the owner gets a controller reference, so that
// Kubernetes garbage-collects it when the owner is deleted. Everything else is
// applied without one, because the API server rejects an owner it cannot
// resolve: a namespaced resource may own neither a cluster-scoped object nor an
// object of another namespace. A chart renders both, for example the cert-manager
// leader election Role in kube-system. Those objects carry the release labels the
// renderer stamps instead, and Sweeper removes them.
//
// Ownership is forced: the release is what the resource says it is, so a field
// another manager took is taken back rather than reported as a conflict.
func (a *Applier) ApplyOne(ctx context.Context, owner client.Object, obj *unstructured.Unstructured, log logr.Logger) error {
	target := obj.DeepCopy()

	pruneNulls(target.Object)

	namespaced, err := a.Client.IsObjectNamespaced(target)
	if err != nil {
		return fmt.Errorf("resolving the scope of %s %q: %w", target.GetKind(), target.GetName(), err)
	}

	if namespaced {
		target.SetNamespace(EffectiveNamespace(owner, target, namespaced))
	}

	if Ownable(owner, target, namespaced) {
		if err := controllerutil.SetControllerReference(owner, target, a.Scheme); err != nil {
			return fmt.Errorf("owning %s %q: %w", target.GetKind(), target.GetName(), err)
		}
	} else {
		log.V(2).Info("applying an object the resource cannot own, to be swept by its release labels",
			"kind", target.GetKind(), "name", target.GetName(), "namespace", target.GetNamespace())
	}

	if err := a.Client.Apply(ctx, client.ApplyConfigurationFromUnstructured(target),
		a.FieldOwner, client.ForceOwnership); err != nil {
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

// EffectiveNamespace returns the namespace a rendered object is applied in, and
// the empty string for a cluster-scoped one.
//
// The renderer stamps no namespace: the GitLab chart sets it on every namespaced
// object of its own, but the dependency charts leave it unset, and such an object
// would be created beside the Operator. It falls back to the namespace of the
// owner, the way Helm falls back to the namespace of the release.
func EffectiveNamespace(owner client.Object, obj *unstructured.Unstructured, namespaced bool) string {
	if !namespaced {
		return ""
	}

	if namespace := obj.GetNamespace(); namespace != "" {
		return namespace
	}

	return owner.GetNamespace()
}

// Ownable reports whether the owner can hold a reference on the object, which
// only holds inside its own namespace. It decides between the two ways an object
// of a release is cleaned up: garbage collection, or the sweep of Sweeper.
func Ownable(owner client.Object, obj *unstructured.Unstructured, namespaced bool) bool {
	return namespaced && EffectiveNamespace(owner, obj, namespaced) == owner.GetNamespace()
}
