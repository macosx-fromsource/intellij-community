//go:build !bridge

package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

// setupSiphon is a no-op in the default build. The v2alpha1 Siphon reconciler is
// only compiled in when the binary is built with the `bridge` build tag; see
// siphon.go. It rides with the bridge, which drives the v2alpha1 resources, and
// its definition reaches no public installation.
func setupSiphon(_ ctrl.Manager) error {
	return nil
}
