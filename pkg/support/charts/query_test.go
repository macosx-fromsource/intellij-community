package charts

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Criterion", func() {
	Describe("WithName", func() {
		It("matches Chart name", func() {
			Expect(WithName("test")(newTestChart("test", "", ""))).To(BeTrue())
			Expect(WithName("test")(newTestChart("not-test", "", ""))).To(BeFalse())
		})
	})

	Describe("WithVersion", func() {
		It("matches Chart version", func() {
			Expect(WithVersion("1")(newTestChart("", "1", ""))).To(BeTrue())
			Expect(WithVersion("1")(newTestChart("", "2", ""))).To(BeFalse())
		})
	})

	Describe("WithAppVersion", func() {
		It("matches Chart appVersion", func() {
			Expect(WithAppVersion("1")(newTestChart("", "", "1"))).To(BeTrue())
			Expect(WithAppVersion("1")(newTestChart("", "", "2"))).To(BeFalse())
		})
	})

	Describe("All", func() {
		Expect(
			All(
				WithName("test"),
				WithVersion("1"),
				WithAppVersion("1"),
			)(newTestChart("test", "1", "1"))).To(BeTrue())
		Expect(
			All(
				WithName("test"),
				WithVersion("1"),
				WithAppVersion("1"),
			)(newTestChart("test", "1", "2"))).To(BeFalse())
	})

	Describe("Any", func() {
		Expect(
			Any(
				WithName("test"),
				WithVersion("1"),
				WithAppVersion("1"),
			)(newTestChart("test", "", ""))).To(BeTrue())
		Expect(
			Any(
				WithName("test"),
				WithVersion("1"),
				WithAppVersion("1"),
			)(newTestChart("", "", ""))).To(BeFalse())
	})

	Describe("None", func() {
		Expect(
			None(
				WithName("test"),
				WithVersion("1"),
				WithAppVersion("1"),
			)(newTestChart("", "", ""))).To(BeTrue())
		Expect(
			None(
				WithName("test"),
				WithVersion("1"),
				WithAppVersion("1"),
			)(newTestChart("test", "", ""))).To(BeFalse())
	})
})
