package gitlabcore

import (
	"fmt"
	"strings"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/capabilities"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

// chartName is the name of the GitLab umbrella chart that backs a GitLabCore.
const chartName = "gitlab"

// renderRelease templates the GitLab chart for the resource.
//
// The chart comes from the charts directory of the Operator, the one the image
// bakes in and HELM_CHARTS points at. Nothing is pulled over the network, so a
// version the Operator does not carry is a configuration error rather than a
// download.
//
// The release name is the name of the resource and the release namespace its
// namespace, which is what makes a rendered object collide with the objects of
// another GitLabCore only when the two share both.
func renderRelease(core *apiv2alpha1.GitLabCore, chartsDir string, discovered *capabilities.Capabilities) (*render.Result, error) {
	version := core.Spec.Chart.Version
	if version == "" {
		return nil, fmt.Errorf("spec.chart.version is required; the Operator carries %s", availableChartVersions())
	}

	chartPath, err := render.LocateChart(chartsDir, chartName, version)
	if err != nil {
		return nil, fmt.Errorf("%w; the Operator carries %s", err, availableChartVersions())
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
