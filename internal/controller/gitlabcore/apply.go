package gitlabcore

import (
	"context"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// fieldOwner is the field manager of everything the reconciler applies.
const fieldOwner = client.FieldOwner("gitlabcore-controller")

// neverApplied matches the objects of a release that the Operator leaves to
// whoever administers the cluster: the definitions, because they are
// cluster-wide and outlive every release, and the RBAC, because applying it
// would turn the right to write a GitLabCore into the right to grant any
// permission the Operator holds. See release.NeverApplied.
var neverApplied = release.NeverApplied

// applier writes the rendered objects on behalf of the resource.
func (r *Reconciler) applier() *release.Applier {
	return &release.Applier{Client: r.Client, Scheme: r.Scheme, FieldOwner: fieldOwner}
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

	return r.applyObjectList(ctx, core, applicable, log)
}

// applyObjectList applies a set of rendered objects in order, skipping what
// neverApplied matches. A zero-downtime upgrade applies the release in phases
// and calls it with a subset, for example everything but the workloads it holds
// back to roll out after the migrations.
func (r *Reconciler) applyObjectList(ctx context.Context, core *apiv2alpha1.GitLabCore, objs []*unstructured.Unstructured, log logr.Logger) error {
	return r.applier().Apply(ctx, core, objs, log)
}

// applyObject applies one rendered object, server-side.
func (r *Reconciler) applyObject(ctx context.Context, core *apiv2alpha1.GitLabCore, obj *unstructured.Unstructured, log logr.Logger) error {
	return r.applier().ApplyOne(ctx, core, obj, log)
}
