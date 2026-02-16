package helm

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template", func() {
	When("uses a chart", func() {
		It("must render the template and parse objects", func() {
			template, err := loadTemplate()

			Expect(err).To(BeNil())
			Expect(template.Warnings()).To(BeEmpty())
			Expect(template.Objects()).NotTo(BeEmpty())
		})
	})
})
