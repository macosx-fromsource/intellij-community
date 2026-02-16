package generator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SequenceGenerator", func() {
	var (
		generator Generator
		err       error
	)

	Describe("Generate", func() {
		var (
			characterSets []string
			content       Content
			result        string
		)

		JustBeforeEach(func() {
			generator, err = NewSequenceGenerator("1000", characterSets)
			Expect(err).NotTo(HaveOccurred())
			content, err = generator.Generate("key")
			Expect(err).NotTo(HaveOccurred())

			result = content.GetString("key")
		})

		Context("with a lower format", func() {
			BeforeEach(func() {
				characterSets = []string{"lower"}
			})

			It("should match", func() {
				Expect(result).To(MatchRegexp("^[a-z]{1000}$"))
			})
		})

		Context("with a upper format", func() {
			BeforeEach(func() {
				characterSets = []string{"upper"}
			})

			It("should match", func() {
				Expect(result).To(MatchRegexp("^[A-Z]{1000}$"))
			})
		})

		Context("with a digit format", func() {
			BeforeEach(func() {
				characterSets = []string{"digit"}
			})

			It("should match", func() {
				Expect(result).To(MatchRegexp("^[0-9]{1000}$"))
			})
		})

		Context("with a symbol format", func() {
			BeforeEach(func() {
				characterSets = []string{"symbol"}
			})

			It("should match", func() {
				Expect(result).To(MatchRegexp("^[!\"#$%&'()*+,\\-./:;<=>?@[\\\\\\]\\^_`{|}~]{1000}$"))
			})
		})

		Context("with all the formats", func() {
			BeforeEach(func() {
				characterSets = []string{"lower", "upper", "digit", "symbol"}
			})

			It("should match", func() {
				Expect(result).To(MatchRegexp("^[a-zA-Z0-9!\"#$%&'()*+,\\-./:;<=>?@[\\\\\\]\\^_`{|}~]{1000}$"))
			})
		})
	})
})
