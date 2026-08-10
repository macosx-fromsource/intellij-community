//go:build !bridge

package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

// setupBridge is a no-op in the default build. The bridge (backend-for-frontend)
// server is only compiled in when the binary is built with the `bridge` build
// tag; see bridge.go. This keeps the bridge, its SPA, and the Huma dependency out
// of public operator images.
func setupBridge(_ ctrl.Manager) error {
	return nil
}
