package generator

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Size", func() {
	var (
		size Size
		err  error
	)

	Describe("String", func() {
		BeforeEach(func() {
			size = Size(256)
		})

		It("should return the string", func() {
			Expect(size.String()).To(Equal("256"))
		})
	})

	Describe("Uint64", func() {
		BeforeEach(func() {
			size = Size(256)
		})

		It("should return the uint64", func() {
			Expect(size.Uint64()).To(Equal(uint64(256)))
		})
	})

	DescribeTable("ParseSize",
		func(annotation string, expectedSize int, expectedErr error) {
			size, err = ParseSize(annotation)
			if expectedErr == nil {
				Expect(size).To(Equal(Size(expectedSize)))
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(MatchError(expectedErr))
			}
		},
		Entry("When the size is empty", "", 0, strconv.ErrSyntax),
		Entry("When the size is invalid", "invalid", 0, strconv.ErrSyntax),
		Entry("When the size is negative", "-1", 0, strconv.ErrSyntax),
		Entry("When the size is floating", "3.14", 0, strconv.ErrSyntax),
		Entry("When the size is zero", "0", 0, nil),
		Entry("When the size is valid", "256", 256, nil),
		Entry("When the size is valid", "384", 384, nil),
		Entry("When the size is valid", "521", 521, nil),
		Entry("When the size is valid", "1024", 1024, nil),
		Entry("When the size is valid", "2048", 2048, nil),
		Entry("When the size is valid", "4096", 4096, nil),
		Entry("When the size is not valid", "2049", 0, ErrInvalidSize),
	)
})
