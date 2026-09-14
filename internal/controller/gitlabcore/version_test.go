package gitlabcore

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// labeledJob is a rendered object carrying the target version label, the shape
// the migrations Job of the chart has.
func labeledJob(version string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind("Job")
	obj.SetName("migrations")
	obj.SetLabels(map[string]string{targetVersionLabel: version})

	return obj
}

var _ = Describe("releaseGitLabVersion", func() {
	When("the render carries the target version label", func() {
		release := &render.Result{
			Objects:    []*unstructured.Unstructured{labeledJob("v19.3.2")},
			AppVersion: "v19.0.0",
		}

		It("prefers the label over the appVersion of the chart, without the leading v", func() {
			// The label is what the chart itself computed, so it accounts for a
			// global.gitlabVersion override in the free-form values, which the
			// appVersion does not.
			Expect(releaseGitLabVersion(release)).To(Equal("19.3.2"))
		})
	})

	When("no rendered object carries the label", func() {
		release := &render.Result{
			Objects:    []*unstructured.Unstructured{{}},
			AppVersion: "v19.1.8",
		}

		It("falls back to the appVersion of the chart", func() {
			// Charts before 10.3 do not emit the label, and neither does a
			// release with the migrations component disabled.
			Expect(releaseGitLabVersion(release)).To(Equal("19.1.8"))
		})
	})

	When("the chart declares no appVersion either", func() {
		release := &render.Result{}

		It("is empty rather than a guess", func() {
			Expect(releaseGitLabVersion(release)).To(BeEmpty())
		})
	})

	When("the release is the rendered GitLab chart", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})

		release, err := renderRelease(core, chartsDirectory(), mockCapabilities(), logr.Discard())

		It("resolves a version unrelated to the chart version", func() {
			Expect(err).To(BeNil())

			version := releaseGitLabVersion(release)

			Expect(version).NotTo(BeEmpty())
			Expect(version).NotTo(Equal(core.Spec.Chart.Version))
			Expect(version).NotTo(HavePrefix("v"))
		})
	})
})
