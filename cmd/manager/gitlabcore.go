//go:build bridge

package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/gitlabcore"
)

// addV2Alpha1ToScheme registers the apps.gitlab.com/v2alpha1 types. GitLabCore is
// a definition of its own rather than a version of GitLab, so no conversion links
// the two. Registering the types watches nothing by itself; setupGitLabCore
// decides that.
func addV2Alpha1ToScheme(scheme *runtime.Scheme) {
	utilruntime.Must(appsv2alpha1.AddToScheme(scheme))
}

// setupGitLabCore registers the v2alpha1 GitLabCore reconciler on the manager.
//
// The reconciler rides with the bridge, which drives the v2alpha1 resources, so
// it is gated the same way twice over: the `bridge` build tag keeps it out of the
// default (public) build entirely, and ENABLE_BRIDGE keeps it off at runtime even
// in the tagged build. The runtime gate is not only a feature switch: the
// GitLabCore definition ships in no release (see ADR 26), and a watch on a
// definition the cluster does not serve fails the manager on start.
func setupGitLabCore(mgr ctrl.Manager) error {
	log := ctrl.Log.WithName("controllers").WithName("GitLabCore")

	if !settings.EnableBridge {
		log.Info("GitLabCore reconciler disabled", "hint", "set ENABLE_BRIDGE=true to enable")

		return nil
	}

	return (&gitlabcore.Reconciler{
		Client:   mgr.GetClient(),
		Log:      log,
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("gitlabcore-controller"),
	}).SetupWithManager(mgr)
}
