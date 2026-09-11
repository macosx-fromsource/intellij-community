package render

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chartloader "helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	chartv2util "helm.sh/helm/v4/pkg/chart/v2/util"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"
)

// testTTL is a cache TTL long enough that it never lapses during a test run,
// for tests that are not about pruning itself.
const testTTL = time.Hour

// packageTestChart packages testChartPath into a "<name>-<version>.tgz" inside
// dir and returns its bytes, name, and version.
func packageTestChart(t *testing.T, dir string) (data []byte, name, version string) {
	t.Helper()

	charter, err := chartloader.Load(testChartPath)
	require.NoError(t, err)

	loadedChart, ok := charter.(*chartv2.Chart)
	require.True(t, ok)

	archivePath, err := chartv2util.Save(loadedChart, dir)
	require.NoError(t, err)

	// #nosec G304 -- trusted input, archivePath was just produced by this test
	data, err = os.ReadFile(archivePath)
	require.NoError(t, err)

	return data, loadedChart.Metadata.Name, loadedChart.Metadata.Version
}

// newChartRepoServer serves an index.yaml built from the given chart, plus the
// chart archive itself, mimicking a Helm chart repository such as
// https://charts.gitlab.io/. requests counts every request the server serves,
// keyed by path.
func newChartRepoServer(t *testing.T, archive []byte, name, version string) (server *httptest.Server, requests map[string]int) {
	t.Helper()

	requests = map[string]int{}
	filename := name + "-" + version + ".tgz"

	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)

	index := repo.NewIndexFile()
	require.NoError(t, index.MustAdd(&chartv2.Metadata{Name: name, Version: version}, filename, server.URL+"/charts/", ""))

	indexBytes, err := yaml.Marshal(index)
	require.NoError(t, err)

	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		_, _ = w.Write(indexBytes)
	})

	mux.HandleFunc("/charts/"+filename, func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		_, _ = w.Write(archive)
	})

	return server, requests
}

func TestPullChart(t *testing.T) {
	t.Run("downloads and caches a chart resolved through the repository index", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, requests := newChartRepoServer(t, archive, name, version)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		chartPath, err := PullChart(server.URL, name, version, cacheDir, testTTL, true, nil)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(cacheDir, name+"-"+version+".tgz"), chartPath)

		// #nosec G304 -- trusted input, chartPath was just produced by PullChart
		got, err := os.ReadFile(chartPath)
		require.NoError(t, err)
		assert.Equal(t, archive, got)

		assert.Equal(t, 1, requests["/index.yaml"])
		assert.Equal(t, 1, requests["/charts/"+name+"-"+version+".tgz"])
	})

	t.Run("serves a cached chart without another request to the repository", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, requests := newChartRepoServer(t, archive, name, version)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		_, err := PullChart(server.URL, name, version, cacheDir, testTTL, true, nil)
		require.NoError(t, err)

		chartPath, err := PullChart(server.URL, name, version, cacheDir, testTTL, true, nil)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(cacheDir, name+"-"+version+".tgz"), chartPath)
		assert.Equal(t, 1, requests["/index.yaml"])
		assert.Equal(t, 1, requests["/charts/"+name+"-"+version+".tgz"])
	})

	t.Run("fails when the repository index has no such chart version", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, _ := newChartRepoServer(t, archive, name, version)

		_, err := PullChart(server.URL, name, "9.9.9", filepath.Join(t.TempDir(), "cache"), testTTL, true, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "9.9.9")
	})

	t.Run("fails without caching when the download does not load as a chart", func(t *testing.T) {
		server, requests := newChartRepoServer(t, []byte("not a chart"), "broken", "1.0.0")
		cacheDir := filepath.Join(t.TempDir(), "cache")

		_, err := PullChart(server.URL, "broken", "1.0.0", cacheDir, testTTL, true, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not load as a chart")
		assert.Equal(t, 1, requests["/charts/broken-1.0.0.tgz"])

		entries, err := os.ReadDir(cacheDir)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("fails when no repository is configured", func(t *testing.T) {
		_, err := PullChart("", "gitlab", "1.0.0", t.TempDir(), testTTL, true, nil)

		require.Error(t, err)
	})

	t.Run("fails on a plain http repository by default, without a network call", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, requests := newChartRepoServer(t, archive, name, version)

		_, err := PullChart(server.URL, name, version, filepath.Join(t.TempDir(), "cache"), testTTL, false, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
		assert.Empty(t, requests)
	})

	t.Run("pulls from a plain http repository when allowInsecureHTTP overrides the default", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, _ := newChartRepoServer(t, archive, name, version)

		_, err := PullChart(server.URL, name, version, filepath.Join(t.TempDir(), "cache"), testTTL, true, nil)

		require.NoError(t, err)
	})

	t.Run("fails on a repository URL with an unsupported scheme", func(t *testing.T) {
		_, err := PullChart("oci://example.com/charts", "gitlab", "1.0.0", t.TempDir(), testTTL, true, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})

	t.Run("fails without caching when the chart archive exceeds the size limit", func(t *testing.T) {
		name, version := "gitlab", "1.0.0"
		filename := name + "-" + version + ".tgz"

		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		index := repo.NewIndexFile()
		require.NoError(t, index.MustAdd(&chartv2.Metadata{Name: name, Version: version}, filename, server.URL+"/charts/", ""))

		indexBytes, err := yaml.Marshal(index)
		require.NoError(t, err)

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(indexBytes)
		})

		mux.HandleFunc("/charts/"+filename, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(make([]byte, maxChartArchiveBytes+1))
		})

		cacheDir := filepath.Join(t.TempDir(), "cache")

		_, err = PullChart(server.URL, name, version, cacheDir, testTTL, true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
		assert.Contains(t, err.Error(), "exceeds")

		entries, err := os.ReadDir(cacheDir)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("prunes a chart archive idle past the ttl, but keeps a fresh one", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, _ := newChartRepoServer(t, archive, name, version)
		cacheDir := filepath.Join(t.TempDir(), "cache")
		require.NoError(t, os.MkdirAll(cacheDir, 0o750))

		stalePath := filepath.Join(cacheDir, "stale-1.0.0.tgz")
		require.NoError(t, os.WriteFile(stalePath, []byte("stale"), 0o600))
		require.NoError(t, os.Chtimes(stalePath, time.Time{}, time.Now().Add(-2*time.Hour)))

		freshPath := filepath.Join(cacheDir, "fresh-1.0.0.tgz")
		require.NoError(t, os.WriteFile(freshPath, []byte("fresh"), 0o600))

		_, err := PullChart(server.URL, name, version, cacheDir, time.Hour, true, nil)

		require.NoError(t, err)
		assert.NoFileExists(t, stalePath)
		assert.FileExists(t, freshPath)
	})

	t.Run("does not prune when the ttl is non-positive", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, _ := newChartRepoServer(t, archive, name, version)
		cacheDir := filepath.Join(t.TempDir(), "cache")
		require.NoError(t, os.MkdirAll(cacheDir, 0o750))

		stalePath := filepath.Join(cacheDir, "stale-1.0.0.tgz")
		require.NoError(t, os.WriteFile(stalePath, []byte("stale"), 0o600))
		require.NoError(t, os.Chtimes(stalePath, time.Time{}, time.Now().Add(-2*time.Hour)))

		_, err := PullChart(server.URL, name, version, cacheDir, 0, true, nil)

		require.NoError(t, err)
		assert.FileExists(t, stalePath)
	})

	t.Run("touches a cache hit so a later prune does not treat it as idle", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, _ := newChartRepoServer(t, archive, name, version)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		_, err := PullChart(server.URL, name, version, cacheDir, testTTL, true, nil)
		require.NoError(t, err)

		chartPath := filepath.Join(cacheDir, name+"-"+version+".tgz")
		require.NoError(t, os.Chtimes(chartPath, time.Time{}, time.Now().Add(-2*time.Hour)))

		// A ttl shorter than the age set above would prune the entry were it
		// not for the touch a cache hit performs first.
		_, err = PullChart(server.URL, name, version, cacheDir, time.Hour, true, nil)

		require.NoError(t, err)
		assert.FileExists(t, chartPath)

		info, err := os.Stat(chartPath)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), info.ModTime(), time.Minute)
	})

	t.Run("survives a pull racing a concurrent prune of the same cache entry", func(t *testing.T) {
		// This drives the exact race the "touch can lose a narrow race with a
		// concurrent prune" comment on PullChart describes: a background
		// goroutine prunes cacheDir on a hair-trigger ttl while the main
		// goroutine keeps pulling the same chart, so some iterations land
		// between PullChart's own os.Stat cache-hit check and the touch that
		// follows it. Run with -race, this is what backs that comment's claim
		// that the fallback (re-pull rather than a stale or broken result)
		// actually holds, rather than only reasoning about it.
		archive, name, version := packageTestChart(t, t.TempDir())
		server, _ := newChartRepoServer(t, archive, name, version)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		stop := make(chan struct{})

		var wg sync.WaitGroup

		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					PruneChartCache(cacheDir, time.Nanosecond, nil)
				}
			}
		}()

		// The invariant under test is PullChart's own behavior, not that the
		// file is still there once it returns: with a ttl this aggressive,
		// the background goroutine can (correctly) remove it again before
		// this loop gets back around to asserting anything about it.
		for i := 0; i < 50; i++ {
			chartPath, err := PullChart(server.URL, name, version, cacheDir, time.Nanosecond, true, nil)

			require.NoError(t, err)
			assert.Equal(t, filepath.Join(cacheDir, name+"-"+version+".tgz"), chartPath)
		}

		close(stop)
		wg.Wait()
	})
}

func TestTouchChartCacheEntry(t *testing.T) {
	t.Run("fails on a path that does not exist", func(t *testing.T) {
		// This is what PullChart's cache-hit fallback actually depends on: an
		// entry a concurrent prune removed between PullChart's os.Stat and
		// this call must report an error, or the fallback that re-pulls in
		// that case would never trigger.
		err := touchChartCacheEntry(filepath.Join(t.TempDir(), "gone.tgz"), nil)

		require.Error(t, err)
	})

	t.Run("resets the modification time of an existing entry", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cached.tgz")
		require.NoError(t, os.WriteFile(path, []byte("chart"), 0o600))
		require.NoError(t, os.Chtimes(path, time.Time{}, time.Now().Add(-time.Hour)))

		require.NoError(t, touchChartCacheEntry(path, nil))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), info.ModTime(), time.Minute)
	})
}

func TestPullChartErrorClassification(t *testing.T) {
	t.Run("no repository configured is permanent", func(t *testing.T) {
		_, err := PullChart("", "gitlab", "1.0.0", t.TempDir(), testTTL, true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
	})

	t.Run("a disallowed scheme is permanent", func(t *testing.T) {
		_, err := PullChart("oci://example.com/charts", "gitlab", "1.0.0", t.TempDir(), testTTL, true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
	})

	t.Run("a version the repository does not carry is permanent", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, _ := newChartRepoServer(t, archive, name, version)

		_, err := PullChart(server.URL, name, "9.9.9", t.TempDir(), testTTL, true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
	})

	t.Run("a downloaded archive that does not load as a chart is permanent", func(t *testing.T) {
		server, _ := newChartRepoServer(t, []byte("not a chart"), "broken", "1.0.0")

		_, err := PullChart(server.URL, "broken", "1.0.0", t.TempDir(), testTTL, true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
	})

	t.Run("an unreachable repository is transient", func(t *testing.T) {
		_, err := PullChart("http://127.0.0.1:1", "gitlab", "1.0.0", t.TempDir(), testTTL, true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.True(t, pullErr.Transient())
	})
}

func TestRemoteChartVersions(t *testing.T) {
	t.Run("lists the versions a chart repository index carries", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		index := repo.NewIndexFile()
		require.NoError(t, index.MustAdd(&chartv2.Metadata{Name: "gitlab", Version: "10.1.1"}, "gitlab-10.1.1.tgz", server.URL+"/charts/", ""))
		require.NoError(t, index.MustAdd(&chartv2.Metadata{Name: "gitlab", Version: "10.1.6"}, "gitlab-10.1.6.tgz", server.URL+"/charts/", ""))
		require.NoError(t, index.MustAdd(&chartv2.Metadata{Name: "other", Version: "1.0.0"}, "other-1.0.0.tgz", server.URL+"/charts/", ""))

		indexBytes, err := yaml.Marshal(index)
		require.NoError(t, err)

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(indexBytes)
		})

		versions, err := RemoteChartVersions(server.URL, "gitlab", true, nil)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"10.1.1", "10.1.6"}, versions)
	})

	t.Run("returns an empty result for a chart the repository does not carry", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("apiVersion: v1\nentries: {}\n"))
		})

		versions, err := RemoteChartVersions(server.URL, "gitlab", true, nil)

		require.NoError(t, err)
		assert.Empty(t, versions)
	})

	t.Run("fails when no repository is configured", func(t *testing.T) {
		_, err := RemoteChartVersions("", "gitlab", true, nil)

		require.Error(t, err)
	})

	t.Run("fails on a plain http repository by default, without a network call", func(t *testing.T) {
		requested := false

		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			requested = true
			_, _ = w.Write([]byte("apiVersion: v1\nentries: {}\n"))
		})

		_, err := RemoteChartVersions(server.URL, "gitlab", false, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
		assert.False(t, requested)
	})

	t.Run("fails permanently when no repository is configured", func(t *testing.T) {
		_, err := RemoteChartVersions("", "gitlab", true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
	})

	t.Run("fails transiently against an unreachable repository", func(t *testing.T) {
		_, err := RemoteChartVersions("http://127.0.0.1:1", "gitlab", true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.True(t, pullErr.Transient())
	})

	t.Run("fails permanently when the index exceeds the size limit", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(make([]byte, maxChartIndexBytes+1))
		})

		_, err := RemoteChartVersions(server.URL, "gitlab", true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
		assert.Contains(t, err.Error(), "exceeds")
	})
}

func TestChartIndexCache(t *testing.T) {
	t.Run("reuses the cached index across two different chart versions", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, requests := newChartRepoServer(t, archive, name, version)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		_, err := PullChart(server.URL, name, version, cacheDir, testTTL, true, nil)
		require.NoError(t, err)

		// A different, uncached version: the archive cache cannot short-circuit
		// this one, so it is the index cache alone that keeps this from making
		// a second request.
		_, err = PullChart(server.URL, name, "9.9.9", cacheDir, testTTL, true, nil)
		require.Error(t, err)

		assert.Equal(t, 1, requests["/index.yaml"])
	})

	t.Run("RemoteChartVersions reuses an index PullChart already fetched", func(t *testing.T) {
		archive, name, version := packageTestChart(t, t.TempDir())
		server, requests := newChartRepoServer(t, archive, name, version)

		_, err := PullChart(server.URL, name, version, filepath.Join(t.TempDir(), "cache"), testTTL, true, nil)
		require.NoError(t, err)

		versions, err := RemoteChartVersions(server.URL, name, true, nil)

		require.NoError(t, err)
		assert.Contains(t, versions, version)
		assert.Equal(t, 1, requests["/index.yaml"])
	})
}

func TestFetchLimited(t *testing.T) {
	newServer := func(t *testing.T, body []byte) *httptest.Server {
		t.Helper()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(body)
		})

		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		return server
	}

	t.Run("accepts a response exactly at the limit", func(t *testing.T) {
		server := newServer(t, make([]byte, 100))

		body, err := fetchLimited(&http.Client{}, server.URL, 100)

		require.NoError(t, err)
		assert.Len(t, body, 100)
	})

	t.Run("rejects a response over the limit before reading it in full", func(t *testing.T) {
		server := newServer(t, make([]byte, 101))

		_, err := fetchLimited(&http.Client{}, server.URL, 100)

		var tooLarge *errResponseTooLarge
		require.ErrorAs(t, err, &tooLarge)
	})

	t.Run("fails on a non-200 status", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := fetchLimited(&http.Client{}, server.URL, 100)

		require.Error(t, err)

		var tooLarge *errResponseTooLarge
		assert.False(t, errors.As(err, &tooLarge))
	})

	t.Run("fails when the request cannot be made at all", func(t *testing.T) {
		_, err := fetchLimited(&http.Client{}, "http://127.0.0.1:1", 100)

		require.Error(t, err)

		var tooLarge *errResponseTooLarge
		assert.False(t, errors.As(err, &tooLarge))
	})
}

func TestFetchChartIndexErrorPaths(t *testing.T) {
	t.Run("fails permanently on an index that does not parse as YAML", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			// A tab is not valid YAML indentation, so this fails to parse
			// rather than merely failing repo.IndexFile's own validation.
			_, _ = w.Write([]byte("apiVersion: v1\nentries:\n\tgitlab: broken\n"))
		})

		_, err := RemoteChartVersions(server.URL, "gitlab", true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.False(t, pullErr.Transient())
		assert.Contains(t, err.Error(), "parsing the index")
	})

	t.Run("fails transiently when the archive URL the index advertises returns a non-200 status", func(t *testing.T) {
		name, version := "gitlab", "1.0.0"
		filename := name + "-" + version + ".tgz"

		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		index := repo.NewIndexFile()
		require.NoError(t, index.MustAdd(&chartv2.Metadata{Name: name, Version: version}, filename, server.URL+"/charts/", ""))

		indexBytes, err := yaml.Marshal(index)
		require.NoError(t, err)

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(indexBytes)
		})

		// No handler for /charts/<filename>: ServeMux answers it 404, which is
		// exactly what a repository whose index has drifted from what it
		// actually serves looks like.
		_, err = PullChart(server.URL, name, version, t.TempDir(), testTTL, true, nil)

		var pullErr *PullError
		require.ErrorAs(t, err, &pullErr)
		assert.True(t, pullErr.Transient())
	})
}
