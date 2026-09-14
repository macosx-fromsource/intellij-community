package release

import (
	"context"
	"slices"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// SweepTarget is one set of objects to delete by the release labels: a kind, and
// the namespace to delete it in. An empty namespace means the kind is
// cluster-scoped.
type SweepTarget struct {
	GVK       schema.GroupVersionKind
	Namespace string
}

// Sweeper deletes the objects of a release that garbage collection cannot reach.
type Sweeper struct {
	// Client is the cluster the objects are deleted from.
	Client client.Client
}

// Sweep deletes the objects of the release that carry no owner reference, by
// their release labels.
//
// The objects in the namespace of the owner are owned by it, so Kubernetes
// deletes them on its own. The rest cannot be owned by a namespaced resource and
// would outlive it: the cluster-scoped objects of the release, and the ones a
// chart renders into another namespace, such as the cert-manager leader election
// Role in kube-system. They are found again through the release labels the
// renderer stamps.
//
// The targets come from rendering the release once more, because nothing records
// what was applied. Rendering is offline and deterministic, so the same
// specification yields the same targets.
//
// The sweep is best effort and never blocks the deletion. A chart archive the
// Operator no longer carries, or values that no longer render, must not leave
// behind a resource that cannot be deleted. What is left is logged with the label
// selector that finds it.
func (s *Sweeper) Sweep(ctx context.Context, owner client.Object, renderRelease func() (*render.Result, error), log logr.Logger) {
	selector := client.MatchingLabels{
		render.ReleaseNameLabel:      owner.GetName(),
		render.ReleaseNamespaceLabel: owner.GetNamespace(),
	}

	result, err := renderRelease()
	if err != nil {
		log.Error(err, "unable to render the release, leaving the unowned objects behind",
			"selector", selector)

		return
	}

	for _, target := range s.UnownedTargets(owner, result, log) {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(target.GVK)

		options := []client.DeleteAllOfOption{selector}

		// A namespaced kind is only deletable inside one namespace: the API
		// server serves no collection delete across namespaces.
		if target.Namespace != "" {
			options = append(options, client.InNamespace(target.Namespace))
		}

		if err := s.Client.DeleteAllOf(ctx, object, options...); err != nil {
			log.Error(err, "unable to delete the unowned objects of a kind",
				"kind", target.GVK.Kind, "namespace", target.Namespace, "selector", selector)

			continue
		}

		log.V(1).Info("deleted the unowned objects of a kind",
			"kind", target.GVK.Kind, "namespace", target.Namespace)
	}
}

// UnownedTargets returns the distinct kinds of the release that carry no owner
// reference, each with the namespace to delete it in.
//
// Hooks are included: one whose delete policy left it in place is unowned too,
// and it carries the same release labels. Objects in the namespace of the owner
// are left out, because deleting the owner collects them.
func (s *Sweeper) UnownedTargets(owner client.Object, result *render.Result, log logr.Logger) []SweepTarget {
	candidates := slices.Clone(result.Objects)

	for _, hook := range result.Hooks {
		candidates = append(candidates, hook.Object)
	}

	targets := []SweepTarget{}

	for _, object := range candidates {
		// What a reconciler never applies, it never deletes either.
		if NeverApplied(object) {
			continue
		}

		namespaced, err := s.Client.IsObjectNamespaced(object)
		if err != nil {
			// A kind the cluster does not serve has nothing to sweep.
			log.V(1).Info("unable to resolve the scope of a rendered kind, skipping it",
				"kind", object.GroupVersionKind().String(), "error", err.Error())

			continue
		}

		if Ownable(owner, object, namespaced) {
			continue
		}

		target := SweepTarget{
			GVK:       object.GroupVersionKind(),
			Namespace: EffectiveNamespace(owner, object, namespaced),
		}

		if !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}

	return targets
}
