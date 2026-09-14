package siphon

import (
	"context"

	"github.com/go-logr/logr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// fieldOwner is the field manager of everything the reconciler applies.
const fieldOwner = client.FieldOwner("siphon-controller")

// applier writes the rendered objects on behalf of the resource.
func (r *Reconciler) applier() *release.Applier {
	return &release.Applier{Client: r.Client, Scheme: r.Scheme, FieldOwner: fieldOwner}
}

// applyObjects applies the rendered objects of the release, in the order the
// renderer returns them, which is the Helm install order.
//
// The RBAC of the chart is skipped, as release.NeverApplied decides. What the
// chart renders there is the Role and the RoleBinding of its own migration wait,
// which the Operator turns off anyway, so nothing of the release needs them.
func (r *Reconciler) applyObjects(ctx context.Context, siphon *apiv2alpha1.Siphon, result *render.Result, log logr.Logger) error {
	skipped, applicable := objects.Partition(result.Objects, release.NeverApplied)

	if len(skipped)+len(result.CRDs) > 0 {
		log.Info("skipping the definitions and the RBAC of the chart, which the Operator never applies",
			"rendered", len(skipped), "bundled definitions", len(result.CRDs))
	}

	return r.applier().Apply(ctx, siphon, applicable, log)
}
