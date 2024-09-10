/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	kubectlscheme "k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	appsv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts/populate"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = kubectlscheme.Scheme
	setupLog = ctrl.Log.WithName("setup")
)

//nolint:wsl
func init() {
	settings.Load()
	runtime.Must(appsv1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr          string
		secureMetrics        bool
		enableMetricsFilter  bool
		enableLeaderElection bool
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080",
		"The address the metric endpoint binds to.")
	flag.BoolVar(&enableMetricsFilter, "enable-metrics-auth-filter", true,
		"If set, the metrics endpoint is is served with a authentication and authorization filter.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	// Add the zap logger flag set to the CLI.
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	err := charts.PopulateGlobalCatalog(
		populate.WithLogger(logger),
		populate.WithSearchPath(settings.HelmChartsDirectory),
	)

	if err != nil {
		setupLog.Error(err, "unable to populate global catalog")
		os.Exit(1)
	}

	operatorScope := "namespace"

	watchNamespace, err := getWatchNamespace()
	if err != nil {
		operatorScope = "cluster"

		setupLog.Info("unable to get WATCH_NAMESPACE, " +
			"the manager will watch and manage resources in all namespaces")
	}

	setupLog.Info("setting operator scope", "scope", operatorScope)

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
	}

	if enableMetricsFilter {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsServerOptions,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		LeaderElection:             enableLeaderElection,
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaderElectionID:           "852d23b0.gitlab.com",
		Cache: cache.Options{
			DefaultNamespaces: watchNamespace,
		},
		HealthProbeBindAddress: settings.HealthProbeBindAddress,
		ReadinessEndpointName:  settings.ReadinessEndpointName,
		LivenessEndpointName:   settings.LivenessEndpointName,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", settings.HealthzCheck); err != nil {
		setupLog.Info("unable to configure healthcheck", err)
	}

	if err := mgr.AddReadyzCheck("readyz", settings.ReadyzCheck); err != nil {
		setupLog.Info("unable to configure readiness check")
	}

	if err = (&controllers.GitLabReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("GitLab"),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("gitlab-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GitLab")
		os.Exit(1)
	}

	if err = (&appsv1beta1.GitLab{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "GitLab")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// Report Operator as "alive" to probe.
	settings.AliveStatus = nil

	// Report Operator as "ready" to probe.
	settings.ReadyStatus = nil

	setupLog.Info("starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// getWatchNamespace returns the Namespace the operator should be watching for changes.
func getWatchNamespace() (map[string]cache.Config, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return map[string]cache.Config{}, fmt.Errorf("%s not set", watchNamespaceEnvVar)
	}

	return map[string]cache.Config{ns: {}}, nil
}
