package gitlabcore

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// pullError builds a *render.PullError for a test the way internal/render
// actually would, rather than fabricating one: an unreachable address fails
// PullChart fast and deterministically as a Transient error, and an unset
// repository fails it as a permanent one.
func pullError(transient bool) error {
	if transient {
		_, err := render.PullChart("http://127.0.0.1:1", "gitlab", "1.0.0", GinkgoT().TempDir(), 0, true, nil)

		return err
	}

	_, err := render.PullChart("", "gitlab", "1.0.0", GinkgoT().TempDir(), 0, true, nil)

	return err
}

var _ = Describe("handleRenderError", func() {
	var (
		core       *apiv2alpha1.GitLabCore
		reconciler *Reconciler
		recorder   *events.FakeRecorder
	)

	BeforeEach(func() {
		core = CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		recorder = events.NewFakeRecorder(10)
		reconciler = &Reconciler{Recorder: recorder}
	})

	When("the error is a transient chart pull failure", func() {
		It("requeues instead of failing the resource", func() {
			result, err := reconciler.handleRenderError(core, pullError(true), GinkgoLogr)

			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))
			Expect(core.Status.Phase).NotTo(Equal(PhaseFailed))
		})

		It("records a ChartPullFailed condition and event, not a config error", func() {
			_, _ = reconciler.handleRenderError(core, pullError(true), GinkgoLogr)

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionInitialized)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal(reasonChartPullFailed))
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))

			Eventually(recorder.Events).Should(Receive(ContainSubstring("ChartPullFailed")))
		})
	})

	When("the error is a permanent chart pull failure", func() {
		It("fails the resource without requeuing", func() {
			result, err := reconciler.handleRenderError(core, pullError(false), GinkgoLogr)

			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(core.Status.Phase).To(Equal(PhaseFailed))
		})

		It("records a ChartPullFailed condition, not a generic config error", func() {
			_, _ = reconciler.handleRenderError(core, pullError(false), GinkgoLogr)

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionInitialized)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal(reasonChartPullFailed))

			Eventually(recorder.Events).Should(Receive(ContainSubstring("ChartPullFailed")))
		})
	})

	When("the error is not a chart pull failure", func() {
		It("fails the resource as a configuration error, without requeuing", func() {
			result, err := reconciler.handleRenderError(core, errors.New("spec.chart.version is required"), GinkgoLogr)

			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(core.Status.Phase).To(Equal(PhaseFailed))

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionInitialized)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal(reasonRenderFailed))

			Eventually(recorder.Events).Should(Receive(ContainSubstring("ConfigError")))
		})
	})
})
