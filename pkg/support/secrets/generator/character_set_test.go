package generator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CharacterSet", func() {
	var (
		cs  CharacterSet
		err error
	)

	Describe("String", func() {
		BeforeEach(func() {
			cs = Lower
		})

		It("should return the string", func() {
			Expect(cs.String()).To(Equal("abcdefghijklmnopqrstuvwxyz"))
		})
	})

	Describe("ParseCharacterSet", func() {
		Context("with a valid name", func() {
			BeforeEach(func() {
				_, err = ParseCharacterSet("lower")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with an invalid name", func() {
			BeforeEach(func() {
				_, err = ParseCharacterSet("probably-never-a-character-set")
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ErrInvalidCharacterSet))
			})
		})
	})
})
