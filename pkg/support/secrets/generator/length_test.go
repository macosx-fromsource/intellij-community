package generator

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Length", func() {
	var (
		length Length
		err    error
	)

	Describe("String", func() {
		BeforeEach(func() {
			length = Length(42)
		})

		It("should return the string", func() {
			Expect(length.String()).To(Equal("42"))
		})
	})

	Describe("Uint64", func() {
		BeforeEach(func() {
			length = Length(42)
		})

		It("should return the uint64", func() {
			Expect(length.Uint64()).To(Equal(uint64(42)))
		})
	})

	DescribeTable("ParseLength",
		func(annotation string, expectedLength int, expectedErr error) {
			length, err = ParseLength(annotation)
			if expectedErr == nil {
				Expect(length).To(Equal(Length(expectedLength)))
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(MatchError(expectedErr))
			}
		},
		Entry("When the length is empty", "", 0, strconv.ErrSyntax),
		Entry("When the length is invalid", "invalid", 0, strconv.ErrSyntax),
		Entry("When the length is negative", "-1", 0, strconv.ErrSyntax),
		Entry("When the length is floating", "3.14", 0, strconv.ErrSyntax),
		Entry("When the length is zero", "0", 0, ErrInvalidLength),
		Entry("When the length is valid", "2048", 2048, nil),
		Entry("When the length is too big", "1048577", 0, ErrInvalidLength),
	)
})
