package siphon

import (
	"context"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// slotRetainedEvent is the reason of the warning emitted on deletion.
const slotRetainedEvent = "ReplicationSlotRetained"

// finalize sweeps what garbage collection cannot reach, warns about what the
// Operator cannot remove at all, and then releases the resource.
//
// Everything the chart renders is namespaced and owned, so Kubernetes deletes it,
// and so is the ConfigMap the table definitions were published to. The sweep is
// there for anything a future chart version renders elsewhere.
//
// What no finalizer can remove is outside the cluster: the replication slot on the
// source server, the publication, and the NATS stream. The slot is the dangerous
// one. PostgreSQL retains write-ahead log for an inactive slot indefinitely, so a
// forgotten slot will eventually fill the volume of the source server. The
// Operator did not create any of the three under the reference-only scope of this
// resource and does not remove them, so it says so, loudly, with the names needed
// to do it by hand.
func (r *Reconciler) finalize(ctx context.Context, siphon *apiv2alpha1.Siphon, log logr.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(siphon, finalizerName) {
		return doNotRequeue()
	}

	log.Info("finalizing Siphon")

	r.warnAboutRetainedState(siphon, log)

	sweeper := &release.Sweeper{Client: r.Client}

	sweeper.Sweep(ctx, siphon, func() (*render.Result, error) {
		discovered, err := r.capabilities()
		if err != nil {
			return nil, err
		}

		// The finalizer renders only to learn which kinds to sweep, and the
		// resolved release only affects values, not kinds. So it renders against
		// what the status recorded rather than resolving the reference again,
		// which would fail once the instance is gone.
		resolved := Release{
			GitLabVersion: siphon.Status.GitLabVersion,
			TablesImage:   siphon.Status.TablesImage,
			TablesSource:  apiv2alpha1.TablesSource(siphon.Status.TablesSource),
		}

		return renderRelease(siphon, settings.HelmChartsDirectory, resolved, discovered)
	}, log)

	controllerutil.RemoveFinalizer(siphon, finalizerName)

	return ctrl.Result{}, r.Update(ctx, siphon)
}

// warnAboutRetainedState reports the PostgreSQL and NATS objects that outlive the
// resource, with the statements that remove them.
func (r *Reconciler) warnAboutRetainedState(siphon *apiv2alpha1.Siphon, log logr.Logger) {
	source := siphon.Spec.Source.Host + "/" + siphon.Spec.Source.Database

	message := "the replication slot %q and the publication %q on %s are not removed by the Operator, " +
		"and PostgreSQL retains write-ahead log for an inactive slot indefinitely. Drop them with " +
		"SELECT pg_drop_replication_slot('%s'); DROP PUBLICATION %s; " +
		"The NATS stream %q is retained too."

	r.Recorder.Eventf(siphon, nil, corev1.EventTypeWarning, slotRetainedEvent, "Finalize",
		message, replicationSlot, producerID, source, replicationSlot, producerID, streamName)

	log.Info("the source database objects of this pipeline are retained",
		"replicationSlot", replicationSlot, "publication", producerID,
		"source", source, "stream", streamName,
		"hint", "drop the slot, or it will retain write-ahead log until the volume is full")
}
