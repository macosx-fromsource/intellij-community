package settings

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Load", func() {
	// Load reads every setting from the environment on each call and mutates
	// the package's own vars, so every case restores what it touches for the
	// rest of the suite.
	var originalTTL time.Duration

	BeforeEach(func() {
		originalTTL = DynamicChartCacheTTL
	})

	AfterEach(func() {
		DynamicChartCacheTTL = originalTTL

		Expect(os.Unsetenv("DYNAMIC_CHART_CACHE_TTL")).To(Succeed())
	})

	When("DYNAMIC_CHART_CACHE_TTL is unset", func() {
		It("leaves the setting at whatever it already was", func() {
			DynamicChartCacheTTL = 42 * time.Minute

			Expect(os.Unsetenv("DYNAMIC_CHART_CACHE_TTL")).To(Succeed())

			Load()

			Expect(DynamicChartCacheTTL).To(Equal(42 * time.Minute))
		})
	})

	When("DYNAMIC_CHART_CACHE_TTL carries a plausible typo, missing its unit", func() {
		It("keeps the previous value rather than silently taking a zero duration", func() {
			DynamicChartCacheTTL = 42 * time.Minute

			Expect(os.Setenv("DYNAMIC_CHART_CACHE_TTL", "30")).To(Succeed())

			Load()

			Expect(DynamicChartCacheTTL).To(Equal(42 * time.Minute))
		})
	})

	When("DYNAMIC_CHART_CACHE_TTL is a valid duration", func() {
		It("takes it", func() {
			Expect(os.Setenv("DYNAMIC_CHART_CACHE_TTL", "45s")).To(Succeed())

			Load()

			Expect(DynamicChartCacheTTL).To(Equal(45 * time.Second))
		})
	})
})
