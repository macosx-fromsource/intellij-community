package controllers

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
)

// ssarReactor returns a reactor for "create selfsubjectaccessreviews" that allows or
// denies the review based on decide, mirroring controllers/settings/kube_test.go.
func ssarReactor(decide func(attrs authorizationv1.ResourceAttributes) bool) clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (bool, runtime.Object, error) {
		review := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)

		return true, &authorizationv1.SelfSubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: decide(*review.Spec.ResourceAttributes),
			},
		}, nil
	}
}

var _ = Describe("checkPermission", func() {
	var (
		client *fake.Clientset
		r      *GitLabReconciler
	)

	BeforeEach(func() {
		client = fake.NewClientset()
		r = &GitLabReconciler{Log: ctrl.Log}
	})

	When("the ServiceAccount has the required permissions", func() {
		It("returns true", func() {
			client.PrependReactor("create", "selfsubjectaccessreviews",
				ssarReactor(func(_ authorizationv1.ResourceAttributes) bool { return true }))

			Expect(r.checkPermission(client, "gateway.networking.k8s.io", "HTTPRoute", "httproutes", "gitlab-system")).To(BeTrue())
		})
	})

	When("the ServiceAccount lacks a required permission", func() {
		It("returns false", func() {
			client.PrependReactor("create", "selfsubjectaccessreviews",
				ssarReactor(func(_ authorizationv1.ResourceAttributes) bool { return false }))

			Expect(r.checkPermission(client, "gateway.networking.k8s.io", "HTTPRoute", "httproutes", "gitlab-system")).To(BeFalse())
		})
	})
})

var _ = Describe("isKindAvailable", func() {
	var r *GitLabReconciler

	BeforeEach(func() {
		r = &GitLabReconciler{Log: ctrl.Log}
	})

	When("the cluster serves the Kind", func() {
		It("returns true", func() {
			Expect(r.isKindAvailable("apps", "v1", "Deployment")).To(BeTrue())
		})
	})

	When("the group version is served, but not the Kind", func() {
		It("returns false", func() {
			// apps/v1 is served by every cluster, but it doesn't have this Kind: this is
			// exactly the case that made the Operator crash before it started checking
			// Kind availability instead of just group version support.
			Expect(r.isKindAvailable("apps", "v1", "TotallyBogusKind")).To(BeFalse())
		})
	})

	When("the group is not served at all", func() {
		It("returns false", func() {
			Expect(r.isKindAvailable("totally.bogus.example.com", "v1", "TotallyBogusKind")).To(BeFalse())
		})
	})
})

// SetupWithManager against a cluster that only partially installs the Gateway API. This
// reproduces gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/2298: an ingress
// controller can install CRDs for some Kinds of gateway.networking.k8s.io (here,
// GatewayClass and Gateway) without installing the rest (HTTPRoute, TCPRoute,
// BackendTLSPolicy). Before the Operator checked Kind availability individually, this made
// it register a watch for a Kind the cluster never serves, which leaves the controller's
// cache permanently unable to sync and eventually crashes the manager once
// CacheSyncTimeout elapses.
var _ = Describe("SetupWithManager with a partial Gateway API installation", func() {
	var (
		partialEnv *envtest.Environment
		partialCfg *rest.Config
	)

	BeforeEach(func() {
		partialEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "config", "crd", "bases"),
				filepath.Join("..", "config", "crd", "test", "gateway-api-partial"),
			},
		}

		var err error

		partialCfg, err = partialEnv.Start()
		Expect(err).NotTo(HaveOccurred())

		// isKindAvailable, and therefore SetupWithManager, reads its Kubernetes config
		// through this package-level singleton, so it has to point at the partial
		// cluster for the duration of this spec. Restore the shared suite config
		// afterwards so later specs keep seeing the fully-populated cluster.
		settings.SetEnvTestConfig(partialCfg)
	})

	AfterEach(func() {
		settings.SetEnvTestConfig(cfg)
		Expect(partialEnv.Stop()).To(Succeed())
	})

	It("completes without error", func() {
		// The shared suite manager (suite_test.go) already registered a "gitlab"
		// controller; skip the process-wide name-uniqueness check controller-runtime
		// otherwise enforces for Prometheus metric labels.
		skipNameValidation := true
		mgr, err := ctrl.NewManager(partialCfg, ctrl.Options{
			Scheme:     scheme.Scheme,
			Controller: config.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())

		r := &GitLabReconciler{Log: ctrl.Log, Scheme: mgr.GetScheme()}
		r.Client = mgr.GetClient()

		Expect(r.SetupWithManager(mgr)).To(Succeed())
	})

	It("considers Gateway available, and HTTPRoute, TCPRoute and BackendTLSPolicy unavailable", func() {
		r := &GitLabReconciler{Log: ctrl.Log}

		Expect(r.isKindAvailable("gateway.networking.k8s.io", "v1", "GatewayClass")).To(BeTrue())
		Expect(r.isKindAvailable("gateway.networking.k8s.io", "v1", "Gateway")).To(BeTrue())

		Expect(r.isKindAvailable("gateway.networking.k8s.io", "v1", "HTTPRoute")).To(BeFalse())
		Expect(r.isKindAvailable("gateway.networking.k8s.io", "v1", "TCPRoute")).To(BeFalse())
		Expect(r.isKindAvailable("gateway.networking.k8s.io", "v1", "BackendTLSPolicy")).To(BeFalse())
	})
})
