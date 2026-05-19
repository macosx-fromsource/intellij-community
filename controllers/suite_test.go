package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	kubectlscheme "k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gitlabv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts/populate"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

const (
	Namespace      = "default"
	PollTimeout    = 30 * time.Second
	PollInterval   = PollTimeout / 100
	envAPIVersions = "GITLAB_OPERATOR_KUBERNETES_API_VERSIONS"
)

func TestAPIs(t *testing.T) {
	if skip := os.Getenv("SKIP_ENVTEST"); skip == "yes" {
		defer GinkgoRecover()

		Skip("skipping cluster-related tests")
	}

	// helm/builder.go initializes against the user's default kubeconfig (not
	// envtest's), so capabilities discovery falls back to defaults. Seed the
	// API versions that chart templates check via Capabilities.APIVersions.Has:
	//   - monitoring.coreos.com/v1: enables ServiceMonitor/PrometheusRule rendering.
	//   - autoscaling/v2/HorizontalPodAutoscaler: makes gitlab.hpa.apiVersion
	//     resolve to autoscaling/v2 instead of falling through to v2beta1, which
	//     k8s.io/client-go no longer registers since v0.36.
	resetEnv := setAPIVersionsEnv("monitoring.coreos.com/v1,autoscaling/v2/HorizontalPodAutoscaler")
	defer resetEnv()

	settings.Load()

	_ = charts.PopulateGlobalCatalog(
		populate.WithSearchPath(settings.HelmChartsDirectory))

	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("Bootstrapping test environment")

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "config", "crd", "test", "gateway-api"),
		},
	}

	var err error

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	settings.SetEnvTestConfig(cfg)

	err = gitlabv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = gitlabv1beta1.AddToScheme(kubectlscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				Namespace: {},
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&GitLabReconciler{
		Client:   k8sManager.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("GitLab"),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorder("gitlab-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()

		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	By("Creating secrets referenced by minimal values")
	createMinimalSecrets()
})

var _ = AfterSuite(func() {
	cancel()
	settings.UnsetEnvTestConfig()
	By("Tearing down the test environment")
	Eventually(func() error {
		return testEnv.Stop()
	}, time.Minute, time.Second).Should(Succeed())
})

func setAPIVersionsEnv(value string) func() {
	beforeVal := os.Getenv(envAPIVersions)

	setEnvOrPanic(envAPIVersions, value)

	return func() {
		setEnvOrPanic(envAPIVersions, beforeVal)
	}
}

func setEnvOrPanic(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		panic(err)
	}
}
