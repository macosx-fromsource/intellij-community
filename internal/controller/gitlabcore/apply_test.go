package gitlabcore

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("applyObjects", func() {
	When("a rendered object is namespaced", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler()

		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")
		result := &render.Result{Objects: []*unstructured.Unstructured{configMap}}

		err := reconciler.applyObjects(context.TODO(), core, result, GinkgoLogr)

		It("creates it in the namespace of the resource", func() {
			Expect(err).To(BeNil())

			created := &corev1.ConfigMap{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created)).To(Succeed())
		})

		It("owns it, so that Kubernetes deletes it with the resource", func() {
			Expect(err).To(BeNil())

			created := &corev1.ConfigMap{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created)).To(Succeed())

			Expect(created.OwnerReferences).To(HaveLen(1))
			Expect(created.OwnerReferences[0].Kind).To(Equal("GitLabCore"))
			Expect(created.OwnerReferences[0].Name).To(Equal(releaseName))
			Expect(created.OwnerReferences[0].Controller).NotTo(BeNil())
			Expect(*created.OwnerReferences[0].Controller).To(BeTrue())
		})

		It("leaves the rendered object untouched", func() {
			Expect(err).To(BeNil())
			Expect(configMap.GetNamespace()).To(BeEmpty())
			Expect(configMap.GetOwnerReferences()).To(BeEmpty())
		})
	})

	When("a rendered object carries explicit nulls", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler()

		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")
		configMap.Object["metadata"].(map[string]any)["annotations"] = nil
		configMap.Object["data"] = map[string]any{"key": "value", "unset": nil}

		result := &render.Result{Objects: []*unstructured.Unstructured{configMap}}

		err := reconciler.applyObjects(context.TODO(), core, result, GinkgoLogr)

		It("applies it without them, so that it owns no empty field", func() {
			Expect(err).To(BeNil())

			created := &corev1.ConfigMap{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created)).To(Succeed())

			Expect(created.Data).To(Equal(map[string]string{"key": "value"}))
		})

		It("applies it server-side, under the field owner of the reconciler", func() {
			Expect(err).To(BeNil())

			created := &corev1.ConfigMap{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created)).To(Succeed())

			Expect(created.ManagedFields).To(HaveLen(1))
			Expect(created.ManagedFields[0].Manager).To(Equal(string(fieldOwner)))
			Expect(created.ManagedFields[0].Operation).To(Equal(metav1.ManagedFieldsOperationApply))
		})
	})

	When("a rendered object is cluster-scoped", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler()

		webhook := mockObject("admissionregistration.k8s.io/v1",
			"ValidatingWebhookConfiguration", "gitlab-webhook")
		result := &render.Result{Objects: []*unstructured.Unstructured{webhook}}

		err := reconciler.applyObjects(context.TODO(), core, result, GinkgoLogr)

		It("creates it without an owner reference, which the API server would reject", func() {
			Expect(err).To(BeNil())

			created := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Name: "gitlab-webhook"}, created)).To(Succeed())

			Expect(created.Namespace).To(BeEmpty())
			Expect(created.OwnerReferences).To(BeEmpty())
		})
	})

	When("the render carries RBAC", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler()

		clusterRole := mockObject("rbac.authorization.k8s.io/v1", "ClusterRole", "gitlab-clusterrole")
		role := mockObject("rbac.authorization.k8s.io/v1", "Role", "gitlab-role")
		role.SetNamespace(testNamespace)

		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")

		result := &render.Result{
			Objects: []*unstructured.Unstructured{clusterRole, role, configMap},
		}

		err := reconciler.applyObjects(context.TODO(), core, result, GinkgoLogr)

		It("applies everything but the RBAC", func() {
			Expect(err).To(BeNil())

			created := &corev1.ConfigMap{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created)).To(Succeed())

			applied := &rbacv1.ClusterRole{}
			Expect(apierrors.IsNotFound(reconciler.Get(context.TODO(),
				types.NamespacedName{Name: "gitlab-clusterrole"}, applied))).To(BeTrue())
		})
	})

	When("a rendered object names a namespace other than the one of the resource", func() {
		// The chart does this: cert-manager renders its leader election Role
		// into kube-system.
		core := CreateMockGitLabCore(releaseName, "other-namespace", support.Values{})
		reconciler := mockReconciler()

		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")
		configMap.SetNamespace(testNamespace)

		result := &render.Result{Objects: []*unstructured.Unstructured{configMap}}

		err := reconciler.applyObjects(context.TODO(), core, result, GinkgoLogr)

		It("applies it there, without an owner reference the API server would reject", func() {
			Expect(err).To(BeNil())

			created := &corev1.ConfigMap{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created)).To(Succeed())

			Expect(created.OwnerReferences).To(BeEmpty())
		})
	})

	When("the render carries definitions", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler()

		definition := mockObject("apiextensions.k8s.io/v1", release.CRDKind, "gateways.gateway.networking.k8s.io")
		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")

		result := &render.Result{
			Objects: []*unstructured.Unstructured{definition, configMap},
			CRDs:    []*unstructured.Unstructured{definition},
		}

		err := reconciler.applyObjects(context.TODO(), core, result, GinkgoLogr)

		// The mapper of the test does not know CustomResourceDefinition, so an
		// attempt to apply the definition would fail while its scope is
		// resolved. Succeeding is the proof that it was skipped.
		It("applies everything but the definitions", func() {
			Expect(err).To(BeNil())

			created := &corev1.ConfigMap{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "gitlab-configmap"}, created)).To(Succeed())
		})
	})
})

// mockReconciler builds a Reconciler on a fake client, for the parts of the
// loop that only need a cluster to write to.
func mockReconciler(objects ...client.Object) *Reconciler {
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithRESTMapper(mockRESTMapper()).
		WithObjects(objects...).
		WithReturnManagedFields().
		Build()

	return &Reconciler{
		Client:   fakeClient,
		Log:      GinkgoLogr,
		Scheme:   scheme.Scheme,
		Recorder: events.NewFakeRecorder(100),
	}
}

// mockObject builds a minimal rendered object.
func mockObject(apiVersion, kind, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}

	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetLabels(map[string]string{
		render.ReleaseNameLabel:      releaseName,
		render.ReleaseNamespaceLabel: testNamespace,
	})

	return obj
}

// observedGenerationSentinel stands in for a controller that has observed the
// current spec. The fake client does not maintain status.observedGeneration, so
// a seeded workload would otherwise read as not-yet-observed under the
// rollout-completion check; a value above any generation the fake client
// assigns keeps that guard satisfied while the replica counts do the testing.
const observedGenerationSentinel int64 = 1 << 40

// mockWorkload builds a live workload with a desired replica count and a count
// of replicas that have finished rolling out. Those replicas are modelled as
// updated, available, and ready, with no old replicas lingering and the spec
// observed, so the workload reads as ready exactly when ready == replicas.
func mockWorkload(kind, name string, replicas, ready int64) *unstructured.Unstructured {
	workload := mockObject("apps/v1", kind, name)
	workload.SetNamespace(testNamespace)

	Expect(unstructured.SetNestedField(workload.Object, replicas, "spec", "replicas")).To(Succeed())
	Expect(unstructured.SetNestedField(workload.Object, observedGenerationSentinel, "status", "observedGeneration")).To(Succeed())

	for _, field := range []string{"readyReplicas", "updatedReplicas", "availableReplicas", "replicas"} {
		Expect(unstructured.SetNestedField(workload.Object, ready, "status", field)).To(Succeed())
	}

	return workload
}
