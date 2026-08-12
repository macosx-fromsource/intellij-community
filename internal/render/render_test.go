package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	// testChartPath is this package's own copy of the fixture chart. It
	// contains a foreign custom resource template (foreigncr.yaml) that no
	// compiled-in scheme knows.
	testChartPath = "testdata/chart/test"

	// hookedChartPath is the local fixture chart exercising hooks and the
	// warning paths.
	hookedChartPath = "testdata/chart/hooked"
)

func TestRender(t *testing.T) {
	t.Run("rendering the test chart with a foreign custom resource", func(t *testing.T) {
		values := support.Values{}
		_ = values.SetValue("foreignCR.create", true)
		_ = values.SetValue("application.create", true)

		result, err := Render(Request{
			ChartPath:   testChartPath,
			ReleaseName: "myrel",
			Namespace:   "testns",
			Values:      values,
		})

		require.NoError(t, err)

		t.Run("renders without errors or warnings", func(t *testing.T) {
			assert.Empty(t, result.Warnings)
			assert.NotEmpty(t, result.Objects)
		})

		t.Run("keeps foreign custom resources", func(t *testing.T) {
			foreign := findByKind(result, "ForeignResource")

			require.NotNil(t, foreign)
			assert.Equal(t, "example.com/v1alpha1", foreign.GetAPIVersion())
		})

		t.Run("applies the release name and namespace", func(t *testing.T) {
			foreign := findByKind(result, "ForeignResource")

			require.NotNil(t, foreign)
			assert.Equal(t, "myrel-test", foreign.GetName())
			assert.Equal(t, "testns", foreign.GetNamespace())
		})

		t.Run("sorts objects by kind in the Helm install order", func(t *testing.T) {
			kinds := []string{}

			for _, obj := range result.Objects {
				kinds = append(kinds, obj.GetKind())
			}

			assert.Less(t, kindIndex(t, kinds, "ServiceAccount"), kindIndex(t, kinds, "Secret"))
			assert.Less(t, kindIndex(t, kinds, "Secret"), kindIndex(t, kinds, "Service"))
			assert.Less(t, kindIndex(t, kinds, "Service"), kindIndex(t, kinds, "Deployment"))

			// Unknown kinds sort after known ones, alphabetically.
			assert.Less(t, kindIndex(t, kinds, "Deployment"), kindIndex(t, kinds, "Application"))
			assert.Less(t, kindIndex(t, kinds, "Application"), kindIndex(t, kinds, "ForeignResource"))
		})

		t.Run("keeps the notes of the top-level chart out of the objects", func(t *testing.T) {
			assert.Contains(t, result.Notes, "Get the application URL")
			assert.Contains(t, result.Notes, "instance=myrel", "the notes carry the release name")
			assert.Contains(t, result.Notes, "testns", "the notes carry the namespace")

			for _, obj := range result.Objects {
				assert.NotContains(t, obj.GetName(), "NOTES")
			}
		})

		t.Run("decodes the integers of a manifest as int64", func(t *testing.T) {
			// The unstructured accessors reject a float64 where they expect
			// an integer, so a manifest decoded through plain YAML would
			// make spec.replicas unreadable for every caller.
			deployment := findByKind(result, "Deployment")
			require.NotNil(t, deployment)

			replicas, found, err := unstructured.NestedFieldNoCopy(deployment.Object, "spec", "replicas")
			require.NoError(t, err)
			require.True(t, found, "the fixture chart sets an explicit replica count")
			assert.IsType(t, int64(0), replicas)

			count, found, err := unstructured.NestedInt64(deployment.Object, "spec", "replicas")
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, int64(1), count)

			// Nested inside a list, where the decoder recurses separately.
			containers, _, err := unstructured.NestedSlice(deployment.Object,
				"spec", "template", "spec", "containers")
			require.NoError(t, err)
			require.NotEmpty(t, containers)

			ports, _, err := unstructured.NestedSlice(containers[0].(map[string]interface{}), "ports")
			require.NoError(t, err)
			require.NotEmpty(t, ports)

			assert.IsType(t, int64(0), ports[0].(map[string]interface{})["containerPort"])
		})

		t.Run("renders deterministically", func(t *testing.T) {
			again, err := Render(Request{
				ChartPath:   testChartPath,
				ReleaseName: "myrel",
				Namespace:   "testns",
				Values:      values,
			})

			require.NoError(t, err)
			require.Len(t, again.Objects, len(result.Objects))

			for i, obj := range result.Objects {
				assert.Equal(t, obj.GetKind(), again.Objects[i].GetKind())
				assert.Equal(t, obj.GetName(), again.Objects[i].GetName())
			}
		})
	})

	t.Run("rendering a chart with hooks", func(t *testing.T) {
		result, err := Render(Request{
			ChartPath:   hookedChartPath,
			ReleaseName: "myrel",
			Namespace:   "testns",
		})

		require.NoError(t, err)

		t.Run("separates hooks from regular objects", func(t *testing.T) {
			require.Len(t, result.Hooks, 4)
			require.Len(t, result.Objects, 1)
			assert.Equal(t, "ConfigMap", result.Objects[0].GetKind())
		})

		t.Run("sorts hooks by kind in the Helm install order", func(t *testing.T) {
			kinds := []string{}

			for _, hook := range result.Hooks {
				kinds = append(kinds, hook.Object.GetKind())
			}

			assert.Equal(t, []string{"ServiceAccount", "ConfigMap", "Pod", "Job"}, kinds)
		})

		t.Run("models the hook with its events and object", func(t *testing.T) {
			hook := findHookByKind(result, "Job")

			require.NotNil(t, hook)
			assert.Equal(t, []string{"pre-install", "pre-upgrade"}, hook.Events)
			assert.Equal(t, "myrel-hooked-migrate", hook.Object.GetName())
			assert.Contains(t, hook.Path, "hook-job.yaml")
		})

		t.Run("parses the hook weight and delete policies", func(t *testing.T) {
			hook := findHookByKind(result, "Job")

			require.NotNil(t, hook)
			assert.Equal(t, 5, hook.Weight)
			assert.Equal(t, []string{"before-hook-creation", "hook-succeeded"}, hook.DeletePolicies)
		})

		t.Run("normalizes events and accepts negative weights", func(t *testing.T) {
			hook := findHookByKind(result, "ServiceAccount")

			require.NotNil(t, hook)
			assert.Equal(t, []string{"pre-install", "pre-upgrade"}, hook.Events)
			assert.Equal(t, -5, hook.Weight)
			assert.Empty(t, hook.DeletePolicies)
		})

		t.Run("defaults the weight to zero and the policies to none", func(t *testing.T) {
			hook := findHookByKind(result, "Pod")

			require.NotNil(t, hook)
			assert.Equal(t, []string{"test"}, hook.Events)
			assert.Zero(t, hook.Weight)
			assert.Empty(t, hook.DeletePolicies)
		})

		t.Run("resolves the test-success alias and unparseable weights", func(t *testing.T) {
			hook := findHookByKind(result, "ConfigMap")

			require.NotNil(t, hook)
			assert.Equal(t, []string{"test"}, hook.Events)
			assert.Zero(t, hook.Weight)
		})

		t.Run("selects and orders hooks for an event", func(t *testing.T) {
			hooks := result.HooksFor("pre-upgrade")

			require.Len(t, hooks, 2)
			assert.Equal(t, "ServiceAccount", hooks[0].Object.GetKind())
			assert.Equal(t, "Job", hooks[1].Object.GetKind())

			for _, hook := range hooks {
				assert.True(t, hook.HasEvent("pre-upgrade"))
			}
		})

		t.Run("breaks weight ties by object name", func(t *testing.T) {
			hooks := result.HooksFor("test")

			require.Len(t, hooks, 2)
			assert.Equal(t, "myrel-hooked-smoke", hooks[0].Object.GetName())
			assert.Equal(t, "myrel-hooked-smoke-data", hooks[1].Object.GetName())
			assert.Empty(t, result.HooksFor("post-install"))
		})

		t.Run("leaves the hooks of the result untouched", func(t *testing.T) {
			before := []string{}

			for _, hook := range result.Hooks {
				before = append(before, hook.Object.GetName())
			}

			_ = result.HooksFor("pre-upgrade")
			_ = result.HooksFor("test")

			for i, hook := range result.Hooks {
				assert.Equal(t, before[i], hook.Object.GetName())
			}
		})

		t.Run("reports no notes when the chart ships none", func(t *testing.T) {
			assert.Empty(t, result.Notes)
		})

		t.Run("loads the definitions of the crds directory", func(t *testing.T) {
			require.Len(t, result.CRDs, 1)
			assert.Equal(t, "CustomResourceDefinition", result.CRDs[0].GetKind())
			assert.Equal(t, "testresources.example.com", result.CRDs[0].GetName())
			assert.Nil(t, findByKind(result, "CustomResourceDefinition"))
		})
	})

	t.Run("rendering manifests with unusable hook annotations", func(t *testing.T) {
		values := support.Values{}
		_ = values.SetValue("invalidHooks.create", true)

		result, err := Render(Request{
			ChartPath:   hookedChartPath,
			ReleaseName: "myrel",
			Values:      values,
		})

		require.NoError(t, err)

		t.Run("drops them with typed warnings like the Helm SDK", func(t *testing.T) {
			require.Len(t, result.Warnings, 2)

			for _, warning := range result.Warnings {
				assert.Equal(t, WarningInvalidHook, warning.Reason)
				assert.NotEmpty(t, warning.String())
			}

			assert.Len(t, result.Hooks, 4)
			assert.Len(t, result.Objects, 1)
		})
	})

	t.Run("rendering a manifest with an unquoted hook weight", func(t *testing.T) {
		values := support.Values{}
		_ = values.SetValue("unquotedWeight.create", true)

		result, err := Render(Request{
			ChartPath:   hookedChartPath,
			ReleaseName: "myrel",
			Values:      values,
		})

		require.NoError(t, err)

		t.Run("accepts it as a convenience, unlike the Helm SDK", func(t *testing.T) {
			hooks := result.HooksFor("post-install")

			require.Len(t, hooks, 1)
			assert.Equal(t, "myrel-hooked-unquoted", hooks[0].Object.GetName())
			assert.Equal(t, 7, hooks[0].Weight)
		})
	})

	t.Run("rendering without a namespace", func(t *testing.T) {
		result, err := Render(Request{
			ChartPath:   hookedChartPath,
			ReleaseName: "myrel",
		})

		require.NoError(t, err)

		t.Run("falls back to the default namespace", func(t *testing.T) {
			require.NotEmpty(t, result.Objects)
			assert.Equal(t, "default", result.Objects[0].GetNamespace())
		})
	})

	t.Run("rendering documents that are not Kubernetes objects", func(t *testing.T) {
		values := support.Values{}
		_ = values.SetValue("broken.create", true)
		_ = values.SetValue("noTypeMeta.create", true)

		result, err := Render(Request{
			ChartPath:   hookedChartPath,
			ReleaseName: "myrel",
			Values:      values,
		})

		t.Run("drops them with typed warnings instead of failing", func(t *testing.T) {
			require.NoError(t, err)
			assert.Len(t, result.Objects, 1)
			require.Len(t, result.Warnings, 2)

			reasons := map[WarningReason]bool{}

			for _, warning := range result.Warnings {
				reasons[warning.Reason] = true

				assert.NotEmpty(t, warning.String())
			}

			assert.Contains(t, reasons, WarningUnparseableDocument)
			assert.Contains(t, reasons, WarningMissingTypeMeta)
		})
	})

	t.Run("the request is incomplete", func(t *testing.T) {
		t.Run("requires a chart path", func(t *testing.T) {
			_, err := Render(Request{ReleaseName: "myrel"})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "ChartPath")
		})

		t.Run("requires a release name", func(t *testing.T) {
			_, err := Render(Request{ChartPath: testChartPath})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "ReleaseName")
		})
	})
}

func TestReleaseLabels(t *testing.T) {
	values := support.Values{}
	_ = values.SetValue("foreignCR.create", true)

	result, err := Render(Request{
		ChartPath:   testChartPath,
		ReleaseName: "myrel",
		Namespace:   "testns",
		Values:      values,
	})

	require.NoError(t, err)

	t.Run("marks every object and hook", func(t *testing.T) {
		require.NotEmpty(t, result.Objects)

		for _, obj := range result.Objects {
			assert.Equal(t, "myrel", obj.GetLabels()[ReleaseNameLabel], "%s %s", obj.GetKind(), obj.GetName())
			assert.Equal(t, "testns", obj.GetLabels()[ReleaseNamespaceLabel])
		}
	})

	t.Run("keeps the labels of the chart", func(t *testing.T) {
		deployment := findByKind(result, "Deployment")

		require.NotNil(t, deployment)
		assert.NotEmpty(t, deployment.GetLabels()["app.kubernetes.io/name"])
	})

	t.Run("leaves the selector and the pod template alone", func(t *testing.T) {
		// A selector is immutable after creation, and a change to the pod
		// template labels rolls every pod on every reconcile.
		deployment := findByKind(result, "Deployment")
		require.NotNil(t, deployment)

		selector, _, err := unstructured.NestedStringMap(deployment.Object, "spec", "selector", "matchLabels")
		require.NoError(t, err)
		assert.NotContains(t, selector, ReleaseNameLabel)

		podLabels, _, err := unstructured.NestedStringMap(
			deployment.Object, "spec", "template", "metadata", "labels")
		require.NoError(t, err)
		assert.NotContains(t, podLabels, ReleaseNameLabel)
	})

	t.Run("marks the hooks too", func(t *testing.T) {
		hooked, err := Render(Request{
			ChartPath:   hookedChartPath,
			ReleaseName: "myrel",
			Namespace:   "testns",
		})

		require.NoError(t, err)
		require.NotEmpty(t, hooked.Hooks)

		for _, hook := range hooked.Hooks {
			assert.Equal(t, "myrel", hook.Object.GetLabels()[ReleaseNameLabel])
		}

		// Definitions are shared between releases and never pruned, so they
		// are left unmarked.
		require.NotEmpty(t, hooked.CRDs)
		assert.NotContains(t, hooked.CRDs[0].GetLabels(), ReleaseNameLabel)
	})

	t.Run("rejects a release name that cannot be a label value", func(t *testing.T) {
		_, err := Render(Request{
			ChartPath:   testChartPath,
			ReleaseName: strings.Repeat("a", 64),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), ReleaseNameLabel)
	})
}

func TestDecodeDocument(t *testing.T) {
	t.Run("decoding the numbers of a document", func(t *testing.T) {
		obj, warning := decodeDocument("numbers.yaml", `
apiVersion: v1
kind: ConfigMap
metadata:
  name: numbers
spec:
  whole: 3
  negative: -2
  huge: 9007199254740993
  fraction: 1.5
`)

		require.Nil(t, warning)

		t.Run("keeps whole numbers exact and integral", func(t *testing.T) {
			for field, want := range map[string]int64{
				"whole":    3,
				"negative": -2,
				// Beyond the float64 mantissa: a float64 round trip would
				// silently return 9007199254740992.
				"huge": 9007199254740993,
			} {
				value, found, err := unstructured.NestedInt64(obj.Object, "spec", field)

				require.NoError(t, err, "field %s", field)
				require.True(t, found, "field %s", field)
				assert.Equal(t, want, value, "field %s", field)
			}
		})

		t.Run("leaves genuine fractions as float64", func(t *testing.T) {
			value, found, err := unstructured.NestedFloat64(obj.Object, "spec", "fraction")

			require.NoError(t, err)
			require.True(t, found)
			assert.InDelta(t, 1.5, value, 0)
		})
	})

	t.Run("reporting documents that are not Kubernetes objects", func(t *testing.T) {
		t.Run("rejects invalid YAML before the JSON decode sees it", func(t *testing.T) {
			_, warning := decodeDocument("broken.yaml", "this is not: [valid yaml")

			require.NotNil(t, warning)
			assert.Equal(t, WarningUnparseableDocument, warning.Reason)
			require.Error(t, warning.Err)
		})

		t.Run("rejects a document that is not a mapping", func(t *testing.T) {
			_, warning := decodeDocument("scalar.yaml", "just a string")

			require.NotNil(t, warning)
			assert.Equal(t, WarningUnparseableDocument, warning.Reason)
		})

		t.Run("separates a missing apiVersion or kind from a parse failure", func(t *testing.T) {
			_, warning := decodeDocument("nometa.yaml", "metadata:\n  name: no-type-meta\n")

			require.NotNil(t, warning)
			assert.Equal(t, WarningMissingTypeMeta, warning.Reason)
			assert.NoError(t, warning.Err)
		})
	})
}

func TestLocateChart(t *testing.T) {
	t.Run("finds a packaged chart by name and version", func(t *testing.T) {
		dir := t.TempDir()
		chartPath := filepath.Join(dir, "mychart-1.2.3.tgz")
		require.NoError(t, os.WriteFile(chartPath, []byte{}, 0o600))

		located, err := LocateChart(dir, "mychart", "1.2.3")

		require.NoError(t, err)
		assert.Equal(t, chartPath, located)
	})

	t.Run("fails with the expected path when the chart is missing", func(t *testing.T) {
		_, err := LocateChart(t.TempDir(), "mychart", "1.2.3")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "mychart-1.2.3.tgz")
	})
}

// findHookByKind returns the first rendered hook of the given object kind.
func findHookByKind(result *Result, kind string) *Hook {
	for i := range result.Hooks {
		if result.Hooks[i].Object.GetKind() == kind {
			return &result.Hooks[i]
		}
	}

	return nil
}

// findByKind returns the first rendered object of the given kind.
func findByKind(result *Result, kind string) *unstructured.Unstructured {
	for _, candidate := range result.Objects {
		if candidate.GetKind() == kind {
			return candidate
		}
	}

	return nil
}

// kindIndex returns the position of the first occurrence of the kind and
// fails the test when the kind is absent, so ordering assertions cannot pass
// vacuously.
func kindIndex(t *testing.T, kinds []string, kind string) int {
	t.Helper()

	for i, candidate := range kinds {
		if candidate == kind {
			return i
		}
	}

	require.Failf(t, "kind not rendered", "no object of kind %s", kind)

	return -1
}
