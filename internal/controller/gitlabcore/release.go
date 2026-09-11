package gitlabcore

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-logr/logr"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/capabilities"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

// chartName is the name of the GitLab umbrella chart that backs a GitLabCore.
const chartName = "gitlab"

// renderRelease templates the GitLab chart for the resource.
//
// The chart is looked up in the charts directory of the Operator first, the
// one the image bakes in and HELM_CHARTS points at. When that directory does
// not carry the requested version and settings.DynamicChartPullEnabled
// allows it (the default), the chart is pulled from
// settings.DynamicChartRepository instead of failing outright; see
// locateOrPullChart.
//
// The release name is the name of the resource and the release namespace its
// namespace, which is what makes a rendered object collide with the objects of
// another GitLabCore only when the two share both.
func renderRelease(core *apiv2alpha1.GitLabCore, chartsDir string, discovered *capabilities.Capabilities, log logr.Logger) (*render.Result, error) {
	return renderReleaseAt(core, chartsDir, core.Spec.Chart.Version, discovered, log)
}

// renderReleaseAt templates the chart at an explicit version, rather than the
// one the spec asks for. A zero-downtime upgrade that steps through the minor
// versions between the deployed one and the target renders each intermediate
// version this way. The values are the same at every version: EffectiveValues
// derives them from the spec, which carries no chart or image version.
func renderReleaseAt(core *apiv2alpha1.GitLabCore, chartsDir, version string, discovered *capabilities.Capabilities, log logr.Logger) (*render.Result, error) {
	if version == "" {
		return nil, fmt.Errorf("spec.chart.version is required; the Operator carries %s", availableChartVersions())
	}

	chartPath, err := locateOrPullChart(chartsDir, version, log)
	if err != nil {
		return nil, err
	}

	values, err := EffectiveValues(core)
	if err != nil {
		return nil, err
	}

	request := render.Request{
		ChartPath:   chartPath,
		ReleaseName: core.Name,
		Namespace:   core.Namespace,
		Values:      values,
	}

	// Left unset, the chart renders against the Helm defaults, which describe
	// the client-go scheme rather than a cluster: PodDisruptionBudget then
	// lands in policy/v1beta1, which no current cluster serves.
	if discovered != nil {
		request.KubeVersion = discovered.KubeVersion
		request.APIVersions = discovered.APIVersions
	}

	return render.Render(request)
}

// locateOrPullChart finds the GitLab chart archive at the requested version,
// falling back to a dynamic pull from settings.DynamicChartRepository when
// HelmChartsDirectory does not carry it and settings.DynamicChartPullEnabled
// allows it. This is the only place a v2alpha1 resource reaches out to a
// chart repository over the network; the v1beta1 controller has no
// equivalent and always fails on a chart it does not carry.
//
// A pulled chart is cached under settings.DynamicChartCacheDirectory, pruned
// after settings.DynamicChartCacheTTL of disuse (see Reconcile, which sweeps
// it independently of this function), and not signature- or
// provenance-verified; see render.PullChart. settings.DynamicChartRepository
// must be an "https://" URL unless settings.DynamicChartAllowHTTP overrides
// that. Debug-level detail about that cache maintenance goes through log,
// the reconciler's own logger, so it respects its verbosity setting rather
// than going to a logger of its own.
func locateOrPullChart(chartsDir, version string, log logr.Logger) (string, error) {
	chartPath, err := render.LocateChart(chartsDir, chartName, version)
	if err == nil {
		return chartPath, nil
	}

	if !settings.DynamicChartPullEnabled {
		return "", fmt.Errorf("%w; the Operator carries %s", err, availableChartVersions())
	}

	chartPath, pullErr := render.PullChart(settings.DynamicChartRepository, chartName, version,
		settings.DynamicChartCacheDirectory, settings.DynamicChartCacheTTL, settings.DynamicChartAllowHTTP, slog.New(logr.ToSlogHandler(log)))
	if pullErr != nil {
		return "", fmt.Errorf("%w; the Operator carries %s", pullErr, availableChartVersions())
	}

	return chartPath, nil
}

// pruneDynamicChartCache sweeps settings.DynamicChartCacheDirectory once per
// reconcile (see Reconcile), independent of whether this pass actually pulled
// or looked up a chart. render.PullChart only prunes as a side effect of a
// pull or a cache hit, so a resource that always renders from the bundled
// charts directory would otherwise never drive a sweep at all, and a chart
// pulled once and then abandoned (e.g. spec.chart.version moves back to a
// bundled version) would never age out.
func pruneDynamicChartCache(log logr.Logger) {
	render.PruneChartCache(settings.DynamicChartCacheDirectory, settings.DynamicChartCacheTTL, slog.New(logr.ToSlogHandler(log)))
}

// remoteChartVersions returns nextChartVersion's source of the versions
// settings.DynamicChartRepository carries, which it prefers over the bundled
// catalog when picking the intermediate of a multi-hop upgrade. It is nil
// when settings.DynamicChartPullEnabled is off, so nextChartVersion falls
// back to the catalog alone and never reaches out over the network on a
// cluster configured to stay offline, the same gate locateOrPullChart applies
// to a single-hop pull.
func remoteChartVersions(log logr.Logger) func() ([]string, error) {
	if !settings.DynamicChartPullEnabled {
		return nil
	}

	return func() ([]string, error) {
		return render.RemoteChartVersions(settings.DynamicChartRepository, chartName, settings.DynamicChartAllowHTTP, slog.New(logr.ToSlogHandler(log)))
	}
}

// capabilities reads the facts of the target cluster the chart branches on.
//
// The cluster is the only source. GITLAB_OPERATOR_KUBERNETES_VERSION and
// GITLAB_OPERATOR_KUBERNETES_API_VERSIONS configure the frozen renderer of the
// v1beta1 path and do not reach here: a render that claims capabilities the
// cluster does not have produces objects that cannot be applied, and the apply
// is what this reconciler does with the result.
func (r *Reconciler) capabilities() (*capabilities.Capabilities, error) {
	return capabilities.Discover(r.Discovery)
}

// availableChartVersions lists the chart versions the Operator carries, for the
// error that reports an unusable spec.chart.version.
func availableChartVersions() string {
	versions := charts.GlobalCatalog().Versions(chartName)
	if len(versions) == 0 {
		return "no GitLab chart version"
	}

	return strings.Join(versions, ", ")
}
