package charts

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

var _ = Describe("Catalog", func() {
	It("is empty after initialization", func() {
		Expect(Catalog{}).To(BeEmpty())
	})

	Describe("Append", func() {
		It("appends new Chart", func() {
			c := &Catalog{}

			c.Append(newTestChart("test", "1", ""))
			c.Append(newTestChart("test", "2", ""))
			c.Append(newTestChart("test", "3", ""))

			Expect(*c).To(HaveLen(3))
		})

		It("does not append when Chart metadata is empty", func() {
			c := &Catalog{}

			c.Append(&chart.Chart{})

			Expect(*c).To(BeEmpty())
		})

		It("does not append when a Chart with the same name and version exists", func() {
			c := &Catalog{}

			c.Append(newTestChart("test", "1", ""))
			c.Append(newTestChart("test", "1", ""))

			Expect(*c).To(HaveLen(1))
		})
	})
})
