package gitlabcore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("hooksAreCurrent", func() {
	When("the resource has no conditions yet", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})

		It("runs the hooks, because nothing says they ran", func() {
			Expect(hooksAreCurrent(core)).To(BeFalse())
		})
	})

	When("the hooks completed for the generation of the resource", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		core.Generation = 3

		setCondition(core, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered, "ran")

		It("skips them", func() {
			Expect(hooksAreCurrent(core)).To(BeTrue())
		})
	})

	When("the specification changed after the hooks completed", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		core.Generation = 3

		setCondition(core, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered, "ran")

		core.Generation = 4

		It("runs them again, because the release is a different one", func() {
			Expect(hooksAreCurrent(core)).To(BeFalse())
		})
	})

	When("the hooks of the last pass failed", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		core.Generation = 3

		setCondition(core, ConditionInitialized, metav1.ConditionFalse, reasonHooksFailed, "boom")

		It("runs them again", func() {
			Expect(hooksAreCurrent(core)).To(BeFalse())
		})
	})

	When("a pass failed while applying, after the hooks had run", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		core.Generation = 3

		setCondition(core, ConditionInitialized, metav1.ConditionTrue, reasonChartRendered, "ran")
		setCondition(core, ConditionAvailable, metav1.ConditionFalse, reasonApplyFailed, "boom")

		It("retries the apply without rerunning them", func() {
			Expect(hooksAreCurrent(core)).To(BeTrue())
		})
	})
})
