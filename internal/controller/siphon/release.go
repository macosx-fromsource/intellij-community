package siphon

import (
	"fmt"
	"slices"
	"strings"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/capabilities"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

// chartName is the name of the Siphon chart that backs a Siphon resource.
const chartName = "siphon"

// renderRelease templates the Siphon chart for the resource.
//
// The chart comes from the charts directory of the Operator, the one the image
// bakes in and HELM_CHARTS points at. Nothing is pulled over the network, so a
// version the Operator does not carry is a configuration error rather than a
// download.
//
// The release name is the name of the resource and the release namespace its
// namespace, which is what makes a rendered object collide with the objects of
// another Siphon only when the two share both.
func renderRelease(siphon *apiv2alpha1.Siphon, chartsDir string, resolved Release, discovered *capabilities.Capabilities) (*render.Result, error) {
	version := siphon.Spec.Chart.Version
	if version == "" {
		return nil, fmt.Errorf("spec.chart.version is required; the Operator carries %s", availableChartVersions())
	}

	chartPath, err := render.LocateChart(chartsDir, chartName, version)
	if err != nil {
		return nil, fmt.Errorf("%w; the Operator carries %s", err, availableChartVersions())
	}

	values, err := EffectiveValues(siphon, resolved, clusterFacts(discovered))
	if err != nil {
		return nil, err
	}

	request := render.Request{
		ChartPath:   chartPath,
		ReleaseName: siphon.Name,
		Namespace:   siphon.Namespace,
		Values:      values,
	}

	// Left unset, the chart renders against the Helm defaults, which describe
	// the client-go scheme rather than a cluster.
	if discovered != nil {
		request.KubeVersion = discovered.KubeVersion
		request.APIVersions = discovered.APIVersions
	}

	return render.Render(request)
}

// clusterFacts reduces the discovered capabilities to what the values layer
// branches on.
func clusterFacts(discovered *capabilities.Capabilities) Cluster {
	if discovered == nil {
		return Cluster{}
	}

	return Cluster{
		ServesPodMonitor: slices.Contains(discovered.APIVersions, podMonitorAPIVersionKind),
	}
}

// capabilities reads the facts of the target cluster the chart branches on, and
// the version the tables source resolves from.
func (r *Reconciler) capabilities() (*capabilities.Capabilities, error) {
	return capabilities.Discover(r.Discovery)
}

// availableChartVersions lists the chart versions the Operator carries, for the
// error that reports an unusable spec.chart.version.
func availableChartVersions() string {
	versions := charts.GlobalCatalog().Versions(chartName)
	if len(versions) == 0 {
		return "no Siphon chart version"
	}

	return strings.Join(versions, ", ")
}
