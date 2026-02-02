package populate

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

var _ = Describe("Populate", func() {
	/* Use the following test scaffolding:
	 *
	 *   chart-1/						# chart-1	v0.1.0
	 *      Chart.yaml
	 *      sub-chart-1/
	 *        Chart.yaml
	 *   more-charts/
	 *      chart-2/					# chart-2	v0.1.2
	 *        Chart.yaml
	 *      chart-2-v2.tar.gz			# chart-2	v0.2.0
	 *      not-chart.tgz
	 *		Note.txt
	 *   not-chart/
	 *     Note.txt
	 *   chart-1-v1.tgz					# chart-1	v0.1.0	(duplicate)
	 *   chart-1-v2.tar.gz				# chart-1	v0.2.0
	 */
	It("populates the Charts with the specified options", func() {
		c := &charts.Catalog{}
		e := c.Populate(
			WithSearchPath("testdata/charts"),
			WithFilePattern("*.tar.gz", "*.tgz"))
		/* The current version of logr can not be integrated with Gingkgo */

		Expect(e).ToNot(HaveOccurred())
		Expect(*c).To(HaveLen(4))
	})

	It("fails to populate with invalid search path", func() {
		c := &charts.Catalog{}
		e := c.Populate(
			WithSearchPath("/i/do/not/exist"),
			WithFilePattern("*.tar.gz", "*.tgz"))

		Expect(e).To(HaveOccurred())
		Expect(e).To(MatchError(ContainSubstring("unable to find any charts")))
		Expect(*c).To(HaveLen(0))
	})

	It("does not fail to populate with invalid search path and valid search path", func() {
		c := &charts.Catalog{}
		e := c.Populate(
			WithSearchPath("testdata/charts", "/i/do/not/exist"),
			WithFilePattern("*.tar.gz", "*.tgz"))

		Expect(e).NotTo(HaveOccurred())
		Expect(*c).To(HaveLen(4))
	})
})

/*
 * This is a catalog test but it uses PopulateOptions and is placed here to
 * avoid circular dependency.
 */
var _ = Describe("Query", func() {
	/* Use the test scaffolding from Populate */
	c := &charts.Catalog{}
	_ = c.Populate(
		WithSearchPath("testdata/charts"),
		WithFilePattern("*.tar.gz", "*.tgz"))

	It("returns the list of available Chart names", func() {
		Expect(c.Names()).To(ConsistOf("chart-1", "chart-2"))
	})

	It("returns the list of available versions of each Chart name", func() {
		Expect(c.Versions("chart-1")).To(ConsistOf("0.1.0", "0.2.0"))
		Expect(c.Versions("chart-2")).To(ConsistOf("0.1.2", "0.2.0"))
	})

	It("returns the list of available appVersions of each Chart name", func() {
		Expect(c.AppVersions("chart-1")).To(ConsistOf("1.0.0", "1.1.0"))
		Expect(c.AppVersions("chart-2")).To(ConsistOf("2.0.0", "2.1.0"))
	})

	It("returns no Chart when the criteria is empty", func() {
		Expect(c.Query()).To(BeEmpty())
	})

	It("returns Charts that match the criteria", func() {
		r := c.Query(charts.WithName("chart-1"), charts.WithVersion("0.1.1"))
		Expect(r).To(BeEmpty())
		Expect(r.First()).To(BeNil())

		r = c.Query(charts.WithName("chart-1"), charts.WithVersion("0.1.0"))
		Expect(r).To(HaveLen(1))
		Expect(r[0].Metadata.Name).To(Equal("chart-1"))
		Expect(r[0].Metadata.Version).To(Equal("0.1.0"))
		Expect(r.First()).NotTo(BeNil())

		r = c.Query(charts.WithName("chart-2"), charts.WithAppVersion("2.1.0"))
		Expect(r).To(HaveLen(1))
		Expect(r[0].Metadata.Name).To(Equal("chart-2"))
		Expect(r[0].Metadata.AppVersion).To(Equal("2.1.0"))

		r = c.Query(charts.WithName("chart-1"))
		Expect(r).To(HaveLen(2))
		Expect(r[0].Metadata.Name).To(Equal("chart-1"))
		Expect(r[1].Metadata.Name).To(Equal("chart-1"))
	})

	It("makes a copy of the original Chart values and its dependencies", func() {
		chrt := &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "test",
				Version: "1.0.0",
			},
			Values: map[string]interface{}{
				"a": "A",
				"b": map[string]interface{}{
					"c": "C",
				},
			},
		}
		depChart := &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "dependency",
				Version: "1.0.0",
			},
			Values: map[string]interface{}{
				"a": "A",
				"b": map[string]interface{}{
					"c": "C",
				},
			},
		}
		depMeta := &chart.Dependency{
			Name:      depChart.Metadata.Name,
			Version:   depChart.Metadata.Version,
			Condition: "dependency.install",
		}

		chrt.SetDependencies(depChart)
		chrt.Metadata.Dependencies = append(chrt.Metadata.Dependencies, depMeta)

		c := &charts.Catalog{
			chrt,
		}

		r1 := c.Query(charts.WithName("test")).First()

		Expect(r1).NotTo(BeIdenticalTo(c))
		Expect(r1.Dependencies()[0]).NotTo(BeIdenticalTo(depChart))
		Expect(r1.Metadata.Dependencies).NotTo(BeIdenticalTo(chrt.Metadata.Dependencies))

		/* imitate Chart rendering with Helm SDK */
		r1.Values["a"] = "A1"
		r1.Values["x"] = "X"
		r1.Values["b"].(map[string]interface{})["c"] = "C2"
		r1.Dependencies()[0].Values["b"].(map[string]interface{})["c"] = "C4"
		// Metadata dependencies are removed when their condition is not met
		r1.Metadata.Dependencies = []*chart.Dependency{}

		Expect(chrt.Values["b"].(map[string]interface{})["c"]).To(Equal("C"))
		Expect(chrt.Metadata.Dependencies).To(HaveLen(1))
		Expect(depChart.Values["b"].(map[string]interface{})["c"]).To(Equal("C"))

		r2 := c.Query(charts.WithName("test")).First()

		Expect(r2).NotTo(BeIdenticalTo(c))
		Expect(r2.Dependencies()[0]).NotTo(BeIdenticalTo(depChart))
		Expect(r2.Metadata.Dependencies).NotTo(BeEmpty())

		Expect(r2.Values).To(HaveKeyWithValue("a", "A"))
		Expect(r2.Values["b"].(map[string]interface{})["c"]).To(Equal("C"))
		Expect(r2.Dependencies()[0].Values["b"].(map[string]interface{})["c"]).To(Equal("C"))
		Expect(r2.Values).NotTo(HaveKeyWithValue("x", "X"))
	})
})

var _ = Describe("GlobalCatalog", func() {
	It("is empty when it is not populated", func() {
		Expect(charts.GlobalCatalog()).To(BeEmpty())
	})

	It("populates the global catalog only once", func() {
		/* Use the test scaffolding from Populate */
		Expect(charts.PopulateGlobalCatalog(
			WithSearchPath("testdata/charts"),
			WithFilePattern("*.tar.gz", "*.tgz"))).ToNot(HaveOccurred())

		Expect(charts.GlobalCatalog()).To(HaveLen(4))

		Expect(charts.PopulateGlobalCatalog()).To(
			MatchError(MatchRegexp("not empty")))
	})

	It("always retruns the same reference", func() {
		a := fmt.Sprintf("%p", charts.GlobalCatalog())
		b := fmt.Sprintf("%p", charts.GlobalCatalog())

		Expect(a).To(BeIdenticalTo(b))
	})
})
