package render

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"
)

// defaultPullTimeout bounds each HTTP round trip a chart pull makes: one for
// the repository index, one for the chart archive itself. A reconcile that
// hangs on a slow or unreachable repository would otherwise stall the
// controller worker indefinitely.
const defaultPullTimeout = 30 * time.Second

// maxChartIndexBytes and maxChartArchiveBytes bound how much of a response
// fetchLimited reads into memory before rejecting it, so a misconfigured
// mirror or a compromised repository cannot exhaust the pod's memory with an
// oversized (or unbounded) response before anything validates a single byte
// of it. Both have generous headroom over what a legitimate response looks
// like today: https://charts.gitlab.io/'s index.yaml is around 1.8 MB and
// grows with every chart release, and the umbrella chart archive itself is
// a little over 1 MB.
const (
	maxChartIndexBytes   = 32 * 1024 * 1024
	maxChartArchiveBytes = 64 * 1024 * 1024
)

// chartIndexCacheTTL is how long fetchChartIndex reuses a repository's index
// after fetching it, rather than downloading and re-parsing it again. A
// multi-hop upgrade renders, and so calls PullChart, roughly every
// defaultRequeueDelay, and the version-lookup path (RemoteChartVersions)
// fetches the very same index again on top of that; without this, both pay
// for the fetch and the YAML parse of an index that only grows over time on
// every single call.
const chartIndexCacheTTL = 5 * time.Minute

// chartIndexCache is the in-memory cache fetchChartIndex reads and writes,
// keyed by repository URL. It is package-level rather than plumbed through
// as an argument because PullChart and RemoteChartVersions are the only
// entry points into this file, and neither owns any longer-lived state of
// its own to hang a cache off of. A sync.Map is enough: the only access
// pattern is a lookup or a whole-entry overwrite, keyed by a handful of
// distinct repository URLs at most.
var chartIndexCache sync.Map // repositoryURL string -> *cachedChartIndex

// cachedChartIndex is one chartIndexCache entry.
type cachedChartIndex struct {
	index     *repo.IndexFile
	fetchedAt time.Time
}

// cachedIndexFor returns the cached index of repositoryURL, or nil if there
// is none or the one on file is older than chartIndexCacheTTL. An expired
// entry is dropped rather than left in place, so a repository URL used once
// and then abandoned (a spec.chart.version that moves on, in practice) does
// not sit in the cache forever.
func cachedIndexFor(repositoryURL string) *repo.IndexFile {
	cached, ok := chartIndexCache.Load(repositoryURL)
	if !ok {
		return nil
	}

	entry, _ := cached.(*cachedChartIndex)
	if entry == nil || time.Since(entry.fetchedAt) > chartIndexCacheTTL {
		chartIndexCache.Delete(repositoryURL)

		return nil
	}

	return entry.index
}

// cacheIndex records index as the current cached index of repositoryURL,
// fetched now.
func cacheIndex(repositoryURL string, index *repo.IndexFile) {
	chartIndexCache.Store(repositoryURL, &cachedChartIndex{index: index, fetchedAt: time.Now()})
}

// errResponseTooLarge reports that fetchLimited stopped reading a response
// because it reached the byte limit it was given, before any of it was used
// for anything. It is its own type so a caller can tell this apart from a
// transient network failure: retrying against the same URL runs into the
// same limit again, so this is always permanent.
type errResponseTooLarge struct {
	url string
	max int64
}

func (e *errResponseTooLarge) Error() string {
	return fmt.Sprintf("the response from %s exceeds the %d byte limit", e.url, e.max)
}

// fetchLimited performs an HTTP GET against href and returns its body,
// rejecting a response larger than maxBytes before it is read into memory in
// full (see errResponseTooLarge). helm's own getter.HTTPGetter.Get has no
// such cap — it does an unbounded io.Copy into a bytes.Buffer — which is
// what this replaces it with for the two responses on this path.
func fetchLimited(client *http.Client, href string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, href, nil)
	if err != nil {
		return nil, fmt.Errorf("building the request for %s: %w", href, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", href, resp.Status)
	}

	// Read one byte past the limit: io.LimitReader silently truncates rather
	// than erroring, so this is what tells a response that lands exactly on
	// maxBytes apart from one that overshoots it.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading the response from %s: %w", href, err)
	}

	if int64(len(body)) > maxBytes {
		return nil, &errResponseTooLarge{url: href, max: maxBytes}
	}

	return body, nil
}

// allowedChartRepositorySchemes are the repository URL schemes PullChart and
// RemoteChartVersions accept without an explicit override. Plain "http" is
// deliberately excluded, since it puts a chart index and archive on the wire
// unencrypted; a caller must set allowInsecureHTTP to allow it.
//
// OCI registries are out of scope: this package has no OCI client, so there
// is nothing yet to add an "oci" exception for. When one is added, note that
// its own plain-HTTP/insecure option, not the "oci://" scheme itself, is
// what needs gating — the scheme alone says nothing about wire security.
var allowedChartRepositorySchemes = map[string]bool{
	"https": true,
}

// PullError reports that PullChart or RemoteChartVersions could not use a
// chart repository. Transient distinguishes a failure a retry might not
// repeat (the repository was unreachable or timed out) from a permanent one
// (a bad configuration, or a repository response that will not change on its
// own), so a caller can requeue the former instead of treating every pull
// failure as unrecoverable.
type PullError struct {
	err       error
	transient bool
}

func newPullError(transient bool, err error) *PullError {
	return &PullError{err: err, transient: transient}
}

// Transient reports whether the failure might not recur on a later attempt
// against the same configuration.
func (e *PullError) Transient() bool {
	return e.transient
}

func (e *PullError) Error() string {
	return e.err.Error()
}

func (e *PullError) Unwrap() error {
	return e.err
}

// validateChartRepositoryScheme rejects a repositoryURL whose scheme is not
// in allowedChartRepositorySchemes, unless it is "http" and allowInsecureHTTP
// overrides the default. It runs before any network call, so a disallowed
// repository never gets so much as a DNS lookup.
func validateChartRepositoryScheme(repositoryURL string, allowInsecureHTTP bool) error {
	parsed, err := url.Parse(repositoryURL)
	if err != nil {
		return fmt.Errorf("render: parsing chart repository %q: %w", repositoryURL, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if allowedChartRepositorySchemes[scheme] || (scheme == "http" && allowInsecureHTTP) {
		return nil
	}

	return fmt.Errorf("render: chart repository %q uses protocol %q, which is not allowed by default; "+
		"only https is, set allowInsecureHTTP to allow http", repositoryURL, parsed.Scheme)
}

// PullChart downloads a chart version from a Helm chart repository (an
// "index.yaml" repository such as https://charts.gitlab.io/, not an OCI
// registry) into cacheDir, and returns the local path to the cached archive.
// A cache hit from an earlier pull is returned without a network call.
//
// Every error it returns is a *PullError, so a caller can tell a transient
// failure (worth retrying) from a permanent one.
//
// repositoryURL must be an "https://" URL unless allowInsecureHTTP allows
// "http://" too (see validateChartRepositoryScheme).
//
// It does not verify a signature or provenance file: the repository is
// expected to be trusted out of band, the same trust the Operator already
// places in its bundled charts. Callers that need that guarantee will have to
// wire it in separately.
//
// Every call also sweeps cacheDir for entries unused for longer than ttl (see
// PruneChartCache), so the directory does not grow forever as a controller
// renders more and more chart versions over its lifetime. A cache hit resets
// an entry's clock before the sweep runs, so a version still in use is never
// evicted by its own request; ttl <= 0 disables the sweep. Call PruneChartCache
// directly on a path that never calls PullChart at all, such as a version
// LocateChart already satisfies from the bundled charts directory: nothing
// else drives the sweep on its own on a timer.
//
// This exists for the v2alpha1 GitLabCore controller, which is the only
// caller wired to fall back to it when a version is not bundled with the
// Operator image (render.LocateChart). Nothing else in the Operator pulls
// charts over the network.
//
// log receives debug-level detail about the cache maintenance the call does
// along the way (writes, reuses, and prunes); a nil log discards it, so
// passing one is optional.
func PullChart(repositoryURL, name, version, cacheDir string, ttl time.Duration, allowInsecureHTTP bool, log *slog.Logger) (string, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	if repositoryURL == "" {
		return "", newPullError(false, fmt.Errorf("render: pulling chart %s version %s: no chart repository configured", name, version))
	}

	if err := validateChartRepositoryScheme(repositoryURL, allowInsecureHTTP); err != nil {
		return "", newPullError(false, fmt.Errorf("render: pulling chart %s version %s: %w", name, version, err))
	}

	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", newPullError(false, fmt.Errorf("render: creating chart cache directory %q: %w", cacheDir, err))
	}

	destPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s.tgz", name, version))

	// An earlier pull already cached this exact name and version; a packaged
	// chart is immutable at a given version, so the cache never goes stale.
	// The touch can lose a narrow race with a concurrent prune of this same
	// entry (elsewhere in the cache, another chart's pull sweeps the whole
	// directory): if the entry is gone by the time it is touched, this falls
	// through and re-pulls it rather than handing back a path that no longer
	// exists.
	if _, err := os.Stat(destPath); err == nil {
		if touchErr := touchChartCacheEntry(destPath, log); touchErr == nil {
			PruneChartCache(cacheDir, ttl, log)

			return destPath, nil
		}
	}

	client := &http.Client{Timeout: defaultPullTimeout}

	chartURL, err := resolveChartURL(client, repositoryURL, name, version, log)
	if err != nil {
		return "", err
	}

	archive, err := fetchLimited(client, chartURL, maxChartArchiveBytes)
	if err != nil {
		var tooLarge *errResponseTooLarge

		return "", newPullError(!errors.As(err, &tooLarge),
			fmt.Errorf("render: downloading chart %s version %s from %s: %w", name, version, chartURL, err))
	}

	if err := writeChart(archive, destPath, log); err != nil {
		return "", newPullError(false, fmt.Errorf("render: caching chart %s version %s pulled from %s: %w", name, version, chartURL, err))
	}

	PruneChartCache(cacheDir, ttl, log)

	return destPath, nil
}

// touchChartCacheEntry marks a cache entry as just used by resetting its
// modification time to now, so PruneChartCache does not treat it as idle. A
// nil log discards its debug output, the same as PullChart and
// PruneChartCache.
func touchChartCacheEntry(path string, log *slog.Logger) error {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	now := time.Now()

	if err := os.Chtimes(path, now, now); err != nil {
		return err
	}

	log.Debug("reused a cached chart", "path", path)

	return nil
}

// PruneChartCache removes the chart archives directly under cacheDir whose
// modification time is older than ttl, i.e. those neither pulled nor reused
// for at least that long. A non-positive ttl disables pruning.
//
// PullChart calls this on every pull or cache hit it serves, but nothing
// calls it when neither happens, such as a reconcile whose version LocateChart
// already satisfies locally: callers that want a cached chart pruned even
// when the bundled charts directory keeps serving every request must call
// this directly instead of relying on PullChart to do it as a side effect.
//
// It only ever looks at "*.tgz" entries, so the ".pull-*.tgz" temporary file
// writeChart stages a download through is never a candidate: a concurrent
// pull's in-progress download is safe from a sweep running for another chart.
// A failure to list or remove is not fatal to the caller; it is retried on
// the next call, and is logged rather than returned.
func PruneChartCache(cacheDir string, ttl time.Duration, log *slog.Logger) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	if ttl <= 0 {
		return
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		log.Debug("skipping the chart cache sweep", "dir", cacheDir, "error", err)

		return
	}

	cutoff := time.Now().Add(-ttl)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".tgz") {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}

		path := filepath.Join(cacheDir, name)

		if err := os.Remove(path); err != nil {
			log.Debug("could not prune an idle cached chart", "path", path, "error", err)

			continue
		}

		log.Debug("pruned an idle cached chart", "path", path, "idle", time.Since(info.ModTime()).Round(time.Second))
	}
}

// fetchChartIndex returns the index of a Helm chart repository, from
// chartIndexCache when a fetch within chartIndexCacheTTL is on file, or by
// downloading and parsing it otherwise. Past the cache, the download is the
// only step of this that is ever transient: a malformed repositoryURL, an
// index that fails to parse, or one that exceeds maxChartIndexBytes is not
// going to change on retry.
func fetchChartIndex(client *http.Client, repositoryURL string, log *slog.Logger) (*repo.IndexFile, error) {
	if index := cachedIndexFor(repositoryURL); index != nil {
		log.Debug("reused the cached index of a chart repository", "repository", repositoryURL)

		return index, nil
	}

	indexURL, err := repo.ResolveReferenceURL(repositoryURL, "index.yaml")
	if err != nil {
		return nil, newPullError(false, fmt.Errorf("render: resolving the index URL of chart repository %q: %w", repositoryURL, err))
	}

	rawIndex, err := fetchLimited(client, indexURL, maxChartIndexBytes)
	if err != nil {
		var tooLarge *errResponseTooLarge

		return nil, newPullError(!errors.As(err, &tooLarge),
			fmt.Errorf("render: fetching the index of chart repository %q: %w", repositoryURL, err))
	}

	index := &repo.IndexFile{}
	if err := yaml.Unmarshal(rawIndex, index); err != nil {
		return nil, newPullError(false, fmt.Errorf("render: parsing the index of chart repository %q: %w", repositoryURL, err))
	}

	cacheIndex(repositoryURL, index)

	return index, nil
}

// resolveChartURL fetches and parses the repository index and resolves it to
// the absolute download URL of the requested chart version. Every failure
// here past the index fetch itself is permanent: the index already loaded,
// so retrying without the repository or the requested version changing
// finds the same thing.
func resolveChartURL(client *http.Client, repositoryURL, name, version string, log *slog.Logger) (string, error) {
	index, err := fetchChartIndex(client, repositoryURL, log)
	if err != nil {
		return "", err
	}

	chartVersion, err := index.Get(name, version)
	if err != nil {
		return "", newPullError(false, fmt.Errorf("render: chart %s version %s not found in repository %q: %w", name, version, repositoryURL, err))
	}

	if len(chartVersion.URLs) == 0 {
		return "", newPullError(false, fmt.Errorf("render: chart %s version %s has no download URL in repository %q", name, version, repositoryURL))
	}

	chartURL, err := repo.ResolveReferenceURL(repositoryURL, chartVersion.URLs[0])
	if err != nil {
		return "", newPullError(false, fmt.Errorf("render: resolving the download URL of chart %s version %s: %w", name, version, err))
	}

	return chartURL, nil
}

// RemoteChartVersions returns the versions a Helm chart repository (an
// "index.yaml" repository such as https://charts.gitlab.io/, not an OCI
// registry) advertises for name, in the order its index lists them, which
// Helm does not guarantee is sorted. A repository with no entry for name at
// all is not an error: it returns an empty, nil-error result. Every error it
// does return is a *PullError, same as PullChart.
//
// repositoryURL is subject to the same protocol restriction as PullChart:
// only "https://" by default, "http://" too when allowInsecureHTTP is set
// (see validateChartRepositoryScheme).
//
// This exists for the v2alpha1 GitLabCore controller's multi-hop
// zero-downtime upgrade path, which needs to know what intermediate versions
// a repository has without committing to downloading one. PullChart remains
// the only function that actually caches a chart archive on disk.
func RemoteChartVersions(repositoryURL, name string, allowInsecureHTTP bool, log *slog.Logger) ([]string, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	if repositoryURL == "" {
		return nil, newPullError(false, fmt.Errorf("render: listing versions of chart %s: no chart repository configured", name))
	}

	if err := validateChartRepositoryScheme(repositoryURL, allowInsecureHTTP); err != nil {
		return nil, newPullError(false, fmt.Errorf("render: listing versions of chart %s: %w", name, err))
	}

	client := &http.Client{Timeout: defaultPullTimeout}

	index, err := fetchChartIndex(client, repositoryURL, log)
	if err != nil {
		return nil, err
	}

	entries := index.Entries[name]
	versions := make([]string, 0, len(entries))

	for _, entry := range entries {
		versions = append(versions, entry.Version)
	}

	log.Debug("listed the versions a chart repository carries", "repository", repositoryURL, "chart", name, "count", len(versions))

	return versions, nil
}

// writeChart validates that data loads as a chart archive and, only then,
// writes it to destPath. It stages the write through a temporary file in the
// same directory and renames it into place, so a reader never observes a
// partially written cache entry, and a download that fails to parse never
// pollutes the cache at all.
func writeChart(data []byte, destPath string, log *slog.Logger) error {
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".pull-*.tgz")
	if err != nil {
		return fmt.Errorf("creating a temporary file: %w", err)
	}

	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("writing the downloaded archive: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing the downloaded archive: %w", err)
	}

	if _, err := loadChart(tmpPath); err != nil {
		return fmt.Errorf("the downloaded archive does not load as a chart: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("moving the downloaded archive into the cache: %w", err)
	}

	log.Debug("wrote a pulled chart to the cache", "path", destPath, "bytes", len(data))

	return nil
}
