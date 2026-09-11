package gitlabcore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	chartloader "helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	chartv2util "helm.sh/helm/v4/pkg/chart/v2/util"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
)

// testPullVersion is the version every dynamic-pull test uses for a chart
// that is never bundled, only ever served by writeMinimalChart and
// serveChartRepository.
const testPullVersion = "9.9.9"

// writeMinimalChart packages a bare chart named chartName at testPullVersion
// into dir and returns the path of the resulting archive.
func writeMinimalChart(dir string) string {
	chartDir := filepath.Join(dir, "src", chartName)
	Expect(os.MkdirAll(chartDir, 0o750)).To(Succeed())

	chartYAML := fmt.Sprintf("apiVersion: v2\nname: %s\nversion: %s\n", chartName, testPullVersion)
	Expect(os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0o600)).To(Succeed())

	charter, err := chartloader.LoadDir(chartDir)
	Expect(err).To(BeNil())

	loadedChart, ok := charter.(*chartv2.Chart)
	Expect(ok).To(BeTrue())

	archivePath, err := chartv2util.Save(loadedChart, filepath.Join(dir, "archive"))
	Expect(err).To(BeNil())

	return archivePath
}

// serveChartRepository starts a Helm chart repository serving the chart
// archived at archivePath, under chartName and testPullVersion.
func serveChartRepository(archivePath string) *httptest.Server {
	archive, err := os.ReadFile(archivePath) // #nosec G304 -- trusted input, produced by this test
	Expect(err).To(BeNil())

	filename := fmt.Sprintf("%s-%s.tgz", chartName, testPullVersion)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	index := repo.NewIndexFile()
	Expect(index.MustAdd(&chartv2.Metadata{Name: chartName, Version: testPullVersion}, filename, server.URL+"/charts/", "")).To(Succeed())

	indexBytes, err := yaml.Marshal(index)
	Expect(err).To(BeNil())

	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(indexBytes)
	})

	mux.HandleFunc("/charts/"+filename, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})

	return server
}

var _ = Describe("locateOrPullChart", func() {
	// This is the only place a v2alpha1 GitLabCore reaches out to a chart
	// repository over the network, and it does so by default, so every case
	// that changes the setting restores it for the rest of the suite (see
	// TestGitLabCore).
	var (
		originalEnabled   bool
		originalRepo      string
		originalCacheDir  string
		originalTTL       time.Duration
		originalAllowHTTP bool
	)

	BeforeEach(func() {
		originalEnabled = settings.DynamicChartPullEnabled
		originalRepo = settings.DynamicChartRepository
		originalCacheDir = settings.DynamicChartCacheDirectory
		originalTTL = settings.DynamicChartCacheTTL
		originalAllowHTTP = settings.DynamicChartAllowHTTP
	})

	AfterEach(func() {
		settings.DynamicChartPullEnabled = originalEnabled
		settings.DynamicChartRepository = originalRepo
		settings.DynamicChartCacheDirectory = originalCacheDir
		settings.DynamicChartCacheTTL = originalTTL
		settings.DynamicChartAllowHTTP = originalAllowHTTP
	})

	When("the version is bundled locally", func() {
		It("never reaches out to the network, regardless of the setting", func() {
			settings.DynamicChartPullEnabled = true
			settings.DynamicChartRepository = "http://127.0.0.1:0" // unreachable

			chartPath, err := locateOrPullChart(chartsDirectory(), chartVersion(), logr.Discard())

			Expect(err).To(BeNil())
			Expect(chartPath).To(Equal(filepath.Join(chartsDirectory(), fmt.Sprintf("%s-%s.tgz", chartName, chartVersion()))))
		})
	})

	When("the version is not bundled and dynamic pulling is disabled", func() {
		It("reports the versions the Operator carries, without reaching out to the network", func() {
			settings.DynamicChartPullEnabled = false
			settings.DynamicChartRepository = "http://127.0.0.1:0" // unreachable

			_, err := locateOrPullChart(chartsDirectory(), "0.0.1", logr.Discard())

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("chart gitlab version 0.0.1 not found"))
			Expect(err.Error()).To(ContainSubstring(chartVersion()))
		})
	})

	When("the version is not bundled and dynamic pulling is enabled (the default)", func() {
		It("pulls it from the configured repository and caches it", func() {
			workDir := GinkgoT().TempDir()
			archivePath := writeMinimalChart(workDir)
			server := serveChartRepository(archivePath)
			DeferCleanup(server.Close)

			settings.DynamicChartPullEnabled = true
			settings.DynamicChartRepository = server.URL
			settings.DynamicChartCacheDirectory = filepath.Join(workDir, "cache")
			settings.DynamicChartAllowHTTP = true

			chartPath, err := locateOrPullChart(chartsDirectory(), testPullVersion, logr.Discard())

			Expect(err).To(BeNil())
			Expect(chartPath).To(Equal(filepath.Join(settings.DynamicChartCacheDirectory, chartName+"-"+testPullVersion+".tgz")))
			Expect(chartPath).To(BeAnExistingFile())
		})

		It("reports the versions the Operator carries when the repository does not have it either", func() {
			workDir := GinkgoT().TempDir()
			archivePath := writeMinimalChart(workDir)
			server := serveChartRepository(archivePath)
			DeferCleanup(server.Close)

			settings.DynamicChartPullEnabled = true
			settings.DynamicChartRepository = server.URL
			settings.DynamicChartCacheDirectory = filepath.Join(workDir, "cache")
			settings.DynamicChartAllowHTTP = true

			_, err := locateOrPullChart(chartsDirectory(), "0.0.1", logr.Discard())

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("0.0.1"))
			Expect(err.Error()).To(ContainSubstring(chartVersion()))
		})

		It("refuses a plain http repository unless DynamicChartAllowHTTP overrides that", func() {
			workDir := GinkgoT().TempDir()
			archivePath := writeMinimalChart(workDir)
			server := serveChartRepository(archivePath)
			DeferCleanup(server.Close)

			settings.DynamicChartPullEnabled = true
			settings.DynamicChartRepository = server.URL
			settings.DynamicChartCacheDirectory = filepath.Join(workDir, "cache")
			settings.DynamicChartAllowHTTP = false

			_, err := locateOrPullChart(chartsDirectory(), testPullVersion, logr.Discard())

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not allowed"))
		})
	})
})

var _ = Describe("pruneDynamicChartCache", func() {
	// Reconcile defers this on every pass, so it is what actually ages a chart
	// out once nothing renders it any more, even a resource that always
	// resolves straight to the bundle and so never calls render.PullChart
	// itself.
	var (
		originalCacheDir string
		originalTTL      time.Duration
	)

	BeforeEach(func() {
		originalCacheDir = settings.DynamicChartCacheDirectory
		originalTTL = settings.DynamicChartCacheTTL
	})

	AfterEach(func() {
		settings.DynamicChartCacheDirectory = originalCacheDir
		settings.DynamicChartCacheTTL = originalTTL
	})

	It("sweeps settings.DynamicChartCacheDirectory by settings.DynamicChartCacheTTL", func() {
		cacheDir := GinkgoT().TempDir()

		stalePath := filepath.Join(cacheDir, "gitlab-9.9.9.tgz")
		Expect(os.WriteFile(stalePath, []byte("stale"), 0o600)).To(Succeed())
		Expect(os.Chtimes(stalePath, time.Time{}, time.Now().Add(-time.Hour))).To(Succeed())

		settings.DynamicChartCacheDirectory = cacheDir
		settings.DynamicChartCacheTTL = time.Minute

		pruneDynamicChartCache(logr.Discard())

		Expect(stalePath).NotTo(BeAnExistingFile())
	})
})

var _ = Describe("remoteChartVersions", func() {
	// nextChartVersion treats a nil provider as "search the bundled catalog
	// only", so this is what makes settings.ENABLE_DYNAMIC_CHART_PULL gate the
	// multi-hop upgrade path the same way it gates a single-hop pull.
	var (
		originalEnabled   bool
		originalRepo      string
		originalAllowHTTP bool
	)

	BeforeEach(func() {
		originalEnabled = settings.DynamicChartPullEnabled
		originalRepo = settings.DynamicChartRepository
		originalAllowHTTP = settings.DynamicChartAllowHTTP
	})

	AfterEach(func() {
		settings.DynamicChartPullEnabled = originalEnabled
		settings.DynamicChartRepository = originalRepo
		settings.DynamicChartAllowHTTP = originalAllowHTTP
	})

	It("is nil when dynamic chart pulling is disabled", func() {
		settings.DynamicChartPullEnabled = false
		settings.DynamicChartRepository = "http://127.0.0.1:0" // unreachable

		Expect(remoteChartVersions(logr.Discard())).To(BeNil())
	})

	It("lists the repository's versions when dynamic chart pulling is enabled", func() {
		workDir := GinkgoT().TempDir()
		archivePath := writeMinimalChart(workDir)
		server := serveChartRepository(archivePath)
		DeferCleanup(server.Close)

		settings.DynamicChartPullEnabled = true
		settings.DynamicChartRepository = server.URL
		settings.DynamicChartAllowHTTP = true

		versions, err := remoteChartVersions(logr.Discard())()

		Expect(err).To(BeNil())
		Expect(versions).To(ContainElement(testPullVersion))
	})
})
