package gitlabcore

import (
	"context"
	"slices"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// sweepTarget is one set of objects to delete by the release labels: a kind,
// and the namespace to delete it in. An empty namespace means the kind is
// cluster-scoped.
type sweepTarget struct {
	gvk       schema.GroupVersionKind
	namespace string
}

// finalize deletes what garbage collection cannot reach and then releases the
// resource.
//
// The objects in the namespace of the resource are owned by it, so Kubernetes
// deletes them on its own. The rest cannot be owned by a namespaced resource and
// would outlive it: the cluster-scoped objects of the release, and the ones the
// chart renders into another namespace, such as the cert-manager leader election
// Role in kube-system. They are found again through the release labels the
// renderer stamps.
//
// The sweep is best effort and never blocks the deletion. A chart archive that
// the Operator no longer carries, or values that no longer render, must not
// leave behind a resource that cannot be deleted. What is left is logged with
// the label selector that finds it.
func (r *Reconciler) finalize(ctx context.Context, core *apiv2alpha1.GitLabCore, log logr.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(core, finalizerName) {
		return doNotRequeue()
	}

	log.Info("finalizing GitLabCore")

	r.sweepUnowned(ctx, core, log)

	controllerutil.RemoveFinalizer(core, finalizerName)

	return ctrl.Result{}, r.Update(ctx, core)
}

// sweepUnowned deletes the objects of the release that carry no owner reference,
// by their release labels.
//
// The targets come from rendering the release once more, because nothing records
// what was applied. Rendering is offline and deterministic, so the same
// specification yields the same targets; a render that fails leaves the objects
// behind, which is logged rather than returned.
func (r *Reconciler) sweepUnowned(ctx context.Context, core *apiv2alpha1.GitLabCore, log logr.Logger) {
	selector := client.MatchingLabels{
		render.ReleaseNameLabel:      core.Name,
		render.ReleaseNamespaceLabel: core.Namespace,
	}

	discovered, err := r.capabilities()
	if err != nil {
		log.Error(err, "unable to read the cluster capabilities, leaving the unowned objects behind",
			"selector", selector)

		return
	}

	release, err := renderRelease(core, settings.HelmChartsDirectory, discovered)
	if err != nil {
		log.Error(err, "unable to render the release, leaving the unowned objects behind",
			"selector", selector)

		return
	}

	for _, target := range r.unownedTargets(core, release, log) {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(target.gvk)

		options := []client.DeleteAllOfOption{selector}

		// A namespaced kind is only deletable inside one namespace: the API
		// server serves no collection delete across namespaces.
		if target.namespace != "" {
			options = append(options, client.InNamespace(target.namespace))
		}

		if err := r.DeleteAllOf(ctx, object, options...); err != nil {
			log.Error(err, "unable to delete the unowned objects of a kind",
				"kind", target.gvk.Kind, "namespace", target.namespace, "selector", selector)

			continue
		}

		log.V(1).Info("deleted the unowned objects of a kind",
			"kind", target.gvk.Kind, "namespace", target.namespace)
	}
}

// unownedTargets returns the distinct kinds of the release that carry no owner
// reference, each with the namespace to delete it in.
//
// Hooks are included: one whose delete policy left it in place is unowned too,
// and it carries the same release labels. Objects in the namespace of the
// resource are left out, because deleting the resource collects them.
func (r *Reconciler) unownedTargets(core *apiv2alpha1.GitLabCore, release *render.Result, log logr.Logger) []sweepTarget {
	candidates := slices.Clone(release.Objects)

	for _, hook := range release.Hooks {
		candidates = append(candidates, hook.Object)
	}

	targets := []sweepTarget{}

	for _, object := range candidates {
		// What the reconciler never applies, it never deletes either.
		if neverApplied(object) {
			continue
		}

		namespaced, err := r.IsObjectNamespaced(object)
		if err != nil {
			// A kind the cluster does not serve has nothing to sweep.
			log.V(1).Info("unable to resolve the scope of a rendered kind, skipping it",
				"kind", object.GroupVersionKind().String(), "error", err.Error())

			continue
		}

		if ownable(core, object, namespaced) {
			continue
		}

		target := sweepTarget{
			gvk:       object.GroupVersionKind(),
			namespace: effectiveNamespace(core, object, namespaced),
		}

		if !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}

	return targets
}
