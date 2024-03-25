package generator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Algorithm", func() {
	var a Algorithm
	var err error

	Describe("String", func() {
		BeforeEach(func() {
			a = RSA
		})

		It("should return the string", func() {
			Expect(a.String()).To(Equal("rsa"))
		})
	})

	Describe("ParseAlgorithm", func() {
		Context("with a valid name", func() {
			BeforeEach(func() {
				_, err = ParseAlgorithm("rsa")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with an invalid name", func() {
			BeforeEach(func() {
				_, err = ParseAlgorithm("probably-never-an-algorithm")
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ErrInvalidAlgorithm))
			})
		})
	})
})
