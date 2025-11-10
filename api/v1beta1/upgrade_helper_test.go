package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	
	"github.com/Masterminds/semver/v3"
)

var _ = Describe("Upgrade Helper", func() {
	DescribeTable("Zero downtime check", func(from, to *semver.Version, expected bool){
		Expect(isZeroDowntimePath(from, to)).To(Equal(expected))
	},
	Entry("Passes on unchanged version",
		semver.MustParse("1.0.0"), semver.MustParse("1.0.0"), true),
	Entry("Passes on patch update",
		semver.MustParse("1.0.0"), semver.MustParse("1.0.1"), true),
	Entry("Passes on minor update",
		semver.MustParse("1.0.0"), semver.MustParse("1.1.0"), true),
	Entry("Passes on allowed major update",
		semver.MustParse("1.11.0"), semver.MustParse("2.0.0"), true),
	Entry("Fails on multi-minor update",
		semver.MustParse("1.0.0"), semver.MustParse("1.2.0"), false),
	Entry("Fails on invalid major update",
		semver.MustParse("1.10.0"), semver.MustParse("2.0.0"), false),
	Entry("Fails on multi-major update",
		semver.MustParse("1.11.0"), semver.MustParse("3.0.0"), false),
	Entry("Fails without start version",
		nil, semver.MustParse("0.1.0"), false),
	Entry("Fails without destination version",
		semver.MustParse("0.1.0"), nil, false),
	Entry("Fails on downgrade",
		semver.MustParse("1.1.0"), semver.MustParse("1.0.0"), false),
    )
})