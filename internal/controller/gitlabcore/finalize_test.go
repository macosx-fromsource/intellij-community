package gitlabcore

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("sweepUnowned", func() {
	When("a GatewayClass carries the release labels", func() {
		It("deletes it", func() {
			core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
			gatewayClass := mockObject("gateway.networking.k8s.io/v1", "GatewayClass", "gitlab-class")
			reconciler := mockReconciler(gatewayClass)

			reconciler.sweepUnowned(context.TODO(), core, GinkgoLogr)

			got := &unstructured.Unstructured{}
			got.SetGroupVersionKind(gatewayClassGVK)
			err := reconciler.Get(context.TODO(), types.NamespacedName{Name: "gitlab-class"}, got)

			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})

	When("a GatewayClass carries another release's labels", func() {
		It("leaves it alone", func() {
			core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})

			other := mockObject("gateway.networking.k8s.io/v1", "GatewayClass", "other-class")
			other.SetLabels(map[string]string{
				render.ReleaseNameLabel:      "other-release",
				render.ReleaseNamespaceLabel: testNamespace,
			})

			reconciler := mockReconciler(other)

			reconciler.sweepUnowned(context.TODO(), core, GinkgoLogr)

			got := &unstructured.Unstructured{}
			got.SetGroupVersionKind(gatewayClassGVK)
			Expect(reconciler.Get(context.TODO(), types.NamespacedName{Name: "other-class"}, got)).To(Succeed())
		})
	})

	When("the cluster does not serve the Gateway API at all", func() {
		It("does not panic or block finalize", func() {
			core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})

			// A mapper with nothing registered: exactly what a cluster with no
			// Gateway API CRDs installed looks like to the RESTMapper.
			reconciler := &Reconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				Log:    GinkgoLogr,
				Scheme: scheme.Scheme,
			}

			Expect(func() {
				reconciler.sweepUnowned(context.TODO(), core, GinkgoLogr)
			}).NotTo(Panic())
		})
	})
})
