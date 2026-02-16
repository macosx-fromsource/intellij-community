package support

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hash", func() {
	Context("NameWithHashSuffix", func() {
		It("should calculate the correct name with hash suffix", func() {
			name := "myjob"
			hash := "qwertyuiop"

			By("Requesting a hash suffix less than the hash length")

			result, err := NameWithHashSuffix(name, hash, 5)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("myjob-yuiop"))

			By("Requesting a hash suffix equal to the hash length")

			result, err = NameWithHashSuffix(name, hash, len(hash))
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("myjob-qwertyuiop"))

			By("Setting a suffix length greater than the object name")

			result, err = NameWithHashSuffix(name, hash, 13)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(""))
		})
	})
})
