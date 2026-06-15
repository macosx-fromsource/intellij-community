package helm

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	applicationv1beta1 "sigs.k8s.io/application/api/v1beta1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

var _ = Describe("Template", func() {
	When("uses a chart", func() {
		It("must render the template and parse objects", func() {
			template, err := loadTemplate()

			Expect(err).To(BeNil())
			Expect(template.Warnings()).To(BeEmpty())
			Expect(template.Objects()).NotTo(BeEmpty())
		})

		It("must decode app.k8s.io/v1beta1 Application as a typed object without warnings", func() {
			values := support.Values{}
			_ = values.AddFromYAMLFile("testdata/chart/values.yaml")
			_ = values.SetValue("application.create", true)

			builder, err := NewBuilder(charts.GlobalCatalog())
			Expect(err).To(BeNil())

			template, err := builder.Render(values)
			Expect(err).To(BeNil())
			Expect(template.Warnings()).To(BeEmpty())

			found := false

			for _, obj := range template.Objects() {
				if _, ok := obj.(*applicationv1beta1.Application); ok {
					found = true
					break
				}
			}

			Expect(found).To(BeTrue(), "expected app.k8s.io/v1beta1 Application to be decoded as a typed object")
		})
	})
})
