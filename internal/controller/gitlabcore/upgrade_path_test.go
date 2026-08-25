package gitlabcore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Masterminds/semver/v3"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

var _ = Describe("zeroDowntimePath", func() {
	DescribeTable("reports whether an upgrade is zero-downtime",
		func(from, to string, expected bool) {
			Expect(zeroDowntimePath(semver.MustParse(from), semver.MustParse(to))).To(Equal(expected))
		},
		Entry("a single minor step", "10.0.0", "10.1.0", true),
		Entry("a patch within the same minor", "10.0.0", "10.0.5", true),
		Entry("the same version", "10.1.3", "10.1.3", true),
		Entry("two minors at once", "10.0.0", "10.2.0", false),
		Entry("the roll to the next major", "10.11.0", "11.0.0", true),
		Entry("a minor step across the major", "10.11.0", "11.1.0", false),
		Entry("a major step from an earlier minor", "10.5.0", "11.0.0", false),
		Entry("a downgrade", "10.5.0", "10.4.0", false),
	)
})

var _ = Describe("nextChartVersion", func() {
	DescribeTable("returns the version an upgrade converges to next",
		func(deployed, target string, available []string, expected string) {
			next, err := nextChartVersion(catalogWith(available...), deployed, target)

			Expect(err).To(BeNil())
			Expect(next).To(Equal(expected))
		},
		Entry("the target itself for a single minor step",
			"10.0.8", "10.1.6", []string{"10.0.8", "10.1.6"}, "10.1.6"),
		Entry("the target itself for a patch bump",
			"10.0.8", "10.0.9", []string{"10.0.8", "10.0.9"}, "10.0.9"),
		Entry("the next minor for a two-minor upgrade",
			"10.0.8", "10.2.4", []string{"10.0.8", "10.1.6", "10.2.4"}, "10.1.6"),
		Entry("the highest patch of the next minor",
			"10.0.8", "10.2.4", []string{"10.0.8", "10.1.1", "10.1.6", "10.2.4"}, "10.1.6"),
		Entry("the first minor of the next major across the roll",
			"10.11.2", "11.1.0", []string{"10.11.2", "11.0.3", "11.1.0"}, "11.0.3"),
	)

	When("the required intermediate version is not on disk", func() {
		It("fails loudly rather than skipping a minor", func() {
			_, err := nextChartVersion(catalogWith("10.0.8", "10.2.4"), "10.0.8", "10.2.4")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("10.1"))
		})
	})
})

var _ = Describe("isUpgrade", func() {
	DescribeTable("reports whether the resource asks for a higher version",
		func(deployed, target string, expected bool) {
			core := &apiv2alpha1.GitLabCore{}
			core.Status.Version = deployed
			core.Spec.Chart.Version = target

			Expect(isUpgrade(core)).To(Equal(expected))
		},
		Entry("a fresh install", "", "10.0.8", false),
		Entry("a steady state", "10.0.8", "10.0.8", false),
		Entry("a minor upgrade", "10.0.8", "10.1.6", true),
		Entry("a patch upgrade", "10.0.8", "10.0.9", true),
		Entry("a multi-minor upgrade", "10.0.8", "10.2.4", true),
		Entry("a downgrade", "10.1.6", "10.0.8", false),
	)
})

// catalogWith builds a chart catalog carrying the gitlab chart at each version.
func catalogWith(versions ...string) charts.Catalog {
	cat := charts.Catalog{}

	for _, version := range versions {
		cat.Append(&chart.Chart{Metadata: &chart.Metadata{Name: chartName, Version: version}})
	}

	return cat
}
