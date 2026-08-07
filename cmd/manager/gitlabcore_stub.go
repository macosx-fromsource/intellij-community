//go:build !bridge

package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// addV2Alpha1ToScheme is a no-op in the default build: nothing reconciles the
// apps.gitlab.com/v2alpha1 types there, so registering them would buy nothing.
func addV2Alpha1ToScheme(_ *runtime.Scheme) {}

// setupGitLabCore is a no-op in the default build. The v2alpha1 GitLabCore
// reconciler is only compiled in when the binary is built with the `bridge` build
// tag; see gitlabcore.go. It rides with the bridge, which drives the v2alpha1
// resources, and its definition reaches no public installation.
func setupGitLabCore(_ ctrl.Manager) error {
	return nil
}
