package gitlabcore

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("setCondition", func() {
	When("a condition is recorded", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		core.Generation = 7

		setCondition(core, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered, "rendered")

		It("carries the generation it observed", func() {
			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionInitialized)

			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(reasonChartRendered))
			Expect(condition.ObservedGeneration).To(Equal(int64(7)))
		})

		It("replaces the condition of the same type instead of appending one", func() {
			setCondition(core, ConditionInitialized, metav1.ConditionFalse, reasonHooksFailed, "the hooks failed")

			Expect(core.Status.Conditions).To(HaveLen(1))
			Expect(apimeta.IsStatusConditionTrue(core.Status.Conditions, ConditionInitialized)).To(BeFalse())
		})
	})
})

var _ = Describe("workloadsReady", func() {
	rendered := []*unstructured.Unstructured{
		mockObject("apps/v1", "Deployment", "gitlab-webservice"),
		mockObject("v1", "ConfigMap", "gitlab-configmap"),
	}

	When("every rendered workload has its replicas ready", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler(mockWorkload("Deployment", "gitlab-webservice", 2, 2))

		ready, pending, err := reconciler.workloadsReady(context.TODO(), core,
			&render.Result{Objects: rendered})

		It("reports the release as ready", func() {
			Expect(err).To(BeNil())
			Expect(ready).To(BeTrue())
			Expect(pending).To(BeEmpty())
		})
	})

	When("a workload has fewer replicas ready than it wants", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler(mockWorkload("Deployment", "gitlab-webservice", 2, 1))

		ready, pending, err := reconciler.workloadsReady(context.TODO(), core,
			&render.Result{Objects: rendered})

		It("names the workload that is not ready", func() {
			Expect(err).To(BeNil())
			Expect(ready).To(BeFalse())
			Expect(pending).To(Equal("Deployment gitlab-webservice"))
		})
	})

	When("a rendered workload does not exist yet", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler()

		ready, pending, err := reconciler.workloadsReady(context.TODO(), core,
			&render.Result{Objects: rendered})

		It("reports it as not ready rather than as an error", func() {
			Expect(err).To(BeNil())
			Expect(ready).To(BeFalse())
			Expect(pending).To(Equal("Deployment gitlab-webservice"))
		})
	})

	When("a workload is scaled to zero", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		reconciler := mockReconciler(mockWorkload("Deployment", "gitlab-webservice", 0, 0))

		ready, _, err := reconciler.workloadsReady(context.TODO(), core,
			&render.Result{Objects: rendered})

		It("counts as ready, because nothing is expected to run", func() {
			Expect(err).To(BeNil())
			Expect(ready).To(BeTrue())
		})
	})
})
