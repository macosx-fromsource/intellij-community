//go:build bridge

package main

import (
	"log/slog"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/bridge"
)

// setupBridge registers the bridge (backend-for-frontend) server on the manager.
// It is only compiled into the binary when the `bridge` build tag is set, so the
// feature is entirely absent from the default (public) build. Even in the tagged
// build it stays gated behind the ENABLE_BRIDGE runtime flag.
func setupBridge(mgr ctrl.Manager) error {
	log := ctrl.Log.WithName("bridge")

	if !settings.EnableBridge {
		log.Info("bridge server disabled", "hint", "set ENABLE_BRIDGE=true to enable")

		return nil
	}

	return mgr.Add(&bridge.Server{
		RESTConfig: mgr.GetConfig(),
		Scheme:     mgr.GetScheme(),
		Addr:       settings.BridgeBindAddress,
		Log:        slog.New(logr.ToSlogHandler(log)),
	})
}
