//go:build bridge

package main

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/siphon"
)

// setupSiphon registers the v2alpha1 Siphon reconciler on the manager.
//
// It is gated the same way as the GitLabCore reconciler, and for the same
// reasons: the `bridge` build tag keeps it out of the default build entirely, and
// ENABLE_BRIDGE keeps it off at runtime even in the tagged build, because the
// Siphon definition ships in no release and a watch on a definition the cluster
// does not serve fails the manager on start. The types are already in the scheme;
// addV2Alpha1ToScheme registers the whole group.
func setupSiphon(mgr ctrl.Manager) error {
	log := ctrl.Log.WithName("controllers").WithName("Siphon")

	if !settings.EnableBridge {
		log.Info("Siphon reconciler disabled", "hint", "set ENABLE_BRIDGE=true to enable")

		return nil
	}

	return (&siphon.Reconciler{
		Client:   mgr.GetClient(),
		Log:      log,
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("siphon-controller"),
	}).SetupWithManager(mgr)
}
