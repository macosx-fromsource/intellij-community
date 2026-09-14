package siphon

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// renderTestRelease renders the chart for a resource, which also validates the
// derived values against the values.schema.json the chart ships: the schema
// forbids unknown keys at the top level, and the chart fails a split render whose
// preconditions are unmet, so a successful render is itself the assertion.
func renderTestRelease(t *testing.T, siphon *apiv2alpha1.Siphon) *render.Result {
	t.Helper()

	requireChart(t)

	result, err := renderRelease(siphon, chartsDirectory(), testRelease(), discoveredCapabilities())
	require.NoError(t, err)

	return result
}

// documentOf returns the body of a ConfigMap the release renders, by key.
func documentOf(t *testing.T, result *render.Result, name, key string) string {
	t.Helper()

	configMap := objects.First(result.Objects, objects.And(
		objects.ByKind("ConfigMap"), objects.ByName(name)))
	require.NotNil(t, configMap, "the release renders no ConfigMap %q", name)

	body, found, err := unstructured.NestedString(configMap.Object, "data", key)
	require.NoError(t, err)
	require.True(t, found, "the ConfigMap %q has no %q", name, key)

	return body
}

func TestRenderRelease(t *testing.T) {
	t.Run("renders exactly the three workloads of the topology", func(t *testing.T) {
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		names := []string{}
		for _, obj := range objects.Filter(result.Objects, objects.ByKind("Deployment")) {
			names = append(names, obj.GetName())
		}

		assert.ElementsMatch(t, []string{
			producerDeployment(siphon.Name),
			consumerDeployment(siphon.Name),
			reconcilerDeployment(siphon.Name),
		}, names)
	})

	t.Run("renders the connection base and the layout", func(t *testing.T) {
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		connection := documentOf(t, result, siphon.Name+"-siphon-connection", "connection.yaml")
		layout := documentOf(t, result, siphon.Name+"-siphon-layout", "layout.yaml")

		assert.Contains(t, connection, testSourceHost)
		assert.Contains(t, connection, "application_name: "+producerID)
		assert.Contains(t, layout, "stream_name: "+streamName)
		assert.Contains(t, layout, producerID)
	})

	t.Run("performs no migration wait", func(t *testing.T) {
		// The chart's wait selects the migrations Job by a
		// gitlab.com/target-version label, which the GitLab chart does emit from
		// 10.3 on, but its Role and RoleBinding are RBAC, which the Operator
		// never applies, so the init container cannot list Jobs to match it.
		// Left on, every pod blocks to its timeout and restarts, forever. It
		// defaults on in split mode once global.gitlabVersion is set, which it is
		// here, so this is the case that matters.
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		for _, obj := range result.Objects {
			assert.NotContains(t, obj.GetName(), "wait-for-migrations", obj.GetKind())
		}

		for _, obj := range objects.Filter(result.Objects, objects.ByKind("Deployment")) {
			containers, found, err := unstructured.NestedSlice(
				obj.Object, "spec", "template", "spec", "initContainers")
			require.NoError(t, err)
			require.True(t, found)

			for _, item := range containers {
				container, ok := item.(map[string]interface{})
				require.True(t, ok)

				assert.NotEqual(t, "wait-for-migrations", container["name"], obj.GetName())
			}
		}
	})

	t.Run("mounts the tables image on the init container", func(t *testing.T) {
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		producer := objects.First(result.Objects, objects.And(
			objects.ByKind("Deployment"), objects.ByName(producerDeployment(siphon.Name))))
		require.NotNil(t, producer)

		volumes, found, err := unstructured.NestedSlice(
			producer.Object, "spec", "template", "spec", "volumes")
		require.NoError(t, err)
		require.True(t, found)

		var reference string

		for _, item := range volumes {
			volume, ok := item.(map[string]interface{})
			require.True(t, ok)

			if volume["name"] != tablesVolumeName {
				continue
			}

			image, ok := volume["image"].(map[string]interface{})
			require.True(t, ok, "the chart must emit an image volume, which mountTables swaps")

			reference, ok = image["reference"].(string)
			require.True(t, ok)
		}

		assert.Equal(t, testRelease().TablesImage, reference)
	})

	t.Run("renders no Secret, because the chart creates none", func(t *testing.T) {
		result := renderTestRelease(t, newSiphon())

		assert.Empty(t, objects.Filter(result.Objects, objects.ByKind("Secret")))
	})

	t.Run("stamps the release labels the sweep finds objects by", func(t *testing.T) {
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		require.NotEmpty(t, result.Objects)

		for _, obj := range result.Objects {
			labels := obj.GetLabels()

			assert.Equal(t, siphon.Name, labels[render.ReleaseNameLabel], obj.GetName())
			assert.Equal(t, siphon.Namespace, labels[render.ReleaseNamespaceLabel], obj.GetName())
		}
	})

	t.Run("reports a chart version the Operator does not carry", func(t *testing.T) {
		requireChart(t)

		siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
			s.Spec.Chart.Version = "0.0.1"
		})

		_, err := renderRelease(siphon, chartsDirectory(), testRelease(), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "the Operator carries")
		assert.Contains(t, err.Error(), chartVersion())
	})

	t.Run("reports a missing chart version", func(t *testing.T) {
		siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
			s.Spec.Chart.Version = ""
		})

		_, err := renderRelease(siphon, chartsDirectory(), testRelease(), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.chart.version is required")
	})

	t.Run("reports free-form values the chart schema rejects", func(t *testing.T) {
		// The schema forbids unknown keys at the top level, so a typo in the
		// escape hatch fails the render rather than deploying something odd.
		requireChart(t)

		siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
			s.Spec.Chart.Values = apiv2alpha1.ChartValues{
				Object: map[string]interface{}{"notAChartValue": true},
			}
		})

		_, err := renderRelease(siphon, chartsDirectory(), testRelease(),
			discoveredCapabilities())

		require.Error(t, err)
		assert.True(t,
			strings.Contains(err.Error(), "notAChartValue") ||
				strings.Contains(err.Error(), "schema"),
			"expected a schema violation, got %v", err)
	})
}

func TestClusterFacts(t *testing.T) {
	t.Run("renders a PodMonitor only where the cluster serves the API", func(t *testing.T) {
		assert.False(t, clusterFacts(discoveredCapabilities()).ServesPodMonitor)
		assert.True(t, clusterFacts(
			discoveredCapabilities(podMonitorAPIVersionKind)).ServesPodMonitor)
	})

	t.Run("assumes nothing without discovery", func(t *testing.T) {
		assert.Equal(t, Cluster{}, clusterFacts(nil))
	})
}
