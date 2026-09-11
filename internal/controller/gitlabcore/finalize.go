package gitlabcore

import (
	"context"

	"github.com/go-logr/logr"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// gatewayClassGVK is the GroupVersionKind of the GatewayClass the chart's own
// gatewayclass.yaml template renders. It is the only cluster-scoped kind the
// chart ever applies through the normal object pipeline (see neverApplied):
// everything else cluster-scoped a subchart could render is RBAC or a
// CustomResourceDefinition, and the reconciler never applies either of those
// in the first place.
var gatewayClassGVK = schema.GroupVersionKind{
	Group:   "gateway.networking.k8s.io",
	Version: "v1",
	Kind:    "GatewayClass",
}

// finalize deletes what garbage collection cannot reach and then releases the
// resource.
//
// The objects in the namespace of the resource are owned by it, so Kubernetes
// deletes them on its own. A GatewayClass cannot be, because it is
// cluster-scoped: a namespaced resource may not own a cluster-scoped object,
// so the API server rejects the owner reference. It is found again through
// the release labels the renderer stamps.
//
// The sweep is best effort and never blocks the deletion. What is left is
// logged with the label selector that finds it.
func (r *Reconciler) finalize(ctx context.Context, core *apiv2alpha1.GitLabCore, log logr.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(core, finalizerName) {
		return doNotRequeue()
	}

	log.Info("finalizing GitLabCore")

	r.sweepUnowned(ctx, core, log)

	controllerutil.RemoveFinalizer(core, finalizerName)

	return ctrl.Result{}, r.Update(ctx, core)
}

// sweepUnowned deletes the release's GatewayClass, by its release labels, if
// it has one.
//
// This is a fixed target rather than one discovered by rendering the release
// again: gatewayClassGVK is the only cluster-scoped kind the chart ever
// applies, so there is nothing to render for. That in turn means deleting a
// resource never depends on resolving or rendering its chart at all, dynamic
// chart pull included.
//
// A cluster that does not serve the Gateway API has nothing to sweep, which
// DeleteAllOf reports as an error like any other; that case is expected, so
// it is logged at a low verbosity rather than as a failure to clean up.
func (r *Reconciler) sweepUnowned(ctx context.Context, core *apiv2alpha1.GitLabCore, log logr.Logger) {
	selector := client.MatchingLabels{
		render.ReleaseNameLabel:      core.Name,
		render.ReleaseNamespaceLabel: core.Namespace,
	}

	gatewayClass := &unstructured.Unstructured{}
	gatewayClass.SetGroupVersionKind(gatewayClassGVK)

	err := r.DeleteAllOf(ctx, gatewayClass, selector)

	switch {
	case err == nil:
		log.V(1).Info("swept the release's GatewayClass, if it had one", "selector", selector)
	case apimeta.IsNoMatchError(err):
		log.V(1).Info("the cluster does not serve the Gateway API, nothing to sweep", "selector", selector)
	default:
		log.Error(err, "unable to sweep the release's GatewayClass, leaving it behind", "selector", selector)
	}
}
