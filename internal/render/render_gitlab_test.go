package render

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// gitlabChartName is the name of the GitLab umbrella chart.
const gitlabChartName = "gitlab"

// gitlabChartAPIVersions stands in for what a current cluster with the
// Prometheus operator serves. Render replaces the Helm defaults with it, so an
// entry left out reads as absent to the chart. Both forms appear because the
// templates probe both; find the probes with `grep -r 'APIVersions.Has'
// <chart>`.
//
// This list is not what protects the render: a chart upgrade can add a probe
// it has never heard of. TestRenderGitLabChart asserts on the output instead.
var gitlabChartAPIVersions = []string{
	"v1",
	"apps/v1",
	"batch/v1",
	"batch/v1/CronJob",
	"autoscaling/v2",
	"autoscaling/v2/HorizontalPodAutoscaler",
	"networking.k8s.io/v1",
	"networking.k8s.io/v1/Ingress",
	"networking.k8s.io/v1/IngressClass",
	"policy/v1",
	"policy/v1/PodDisruptionBudget",
	"monitoring.coreos.com/v1",
}

// retiredGroupVersions no longer exist in a supported Kubernetes release. An
// object rendered into one of them cannot be applied anywhere, so the render
// is wrong no matter which chart branch produced it.
var retiredGroupVersions = []string{
	"extensions/v1beta1",
	"apps/v1beta1",
	"apps/v1beta2",
	"batch/v1beta1",
	"policy/v1beta1",
	"networking.k8s.io/v1beta1",
	"autoscaling/v2beta1",
	"autoscaling/v2beta2",
	"rbac.authorization.k8s.io/v1beta1",
	"admissionregistration.k8s.io/v1beta1",
	"apiextensions.k8s.io/v1beta1",
}

// gitlabChartValues are the smallest user values that render the full GitLab
// chart, mirroring the minimal values of the controllers/gitlab test suite.
// The certmanager-issuer email is required by the chart; the operator injects
// it from its settings (see CertmanagerIssuerEmail).
const gitlabChartValues = `
certmanager-issuer:
  email: admin@example.com
gitlab:
  toolbox:
    backups:
      cron:
        enabled: true
      objectStorage:
        config:
          secret: backup-storage-secret
          key: config
registry:
  storage:
    secret: registry-storage-secret
    key: config
global:
  redis:
    host: redis.example.com
  psql:
    host: psql.example.com
    password:
      secret: psql-password
      key: password
  pages:
    objectStore:
      connection:
        secret: object-storage-secret
        key: connection
  appConfig:
    object_store:
      enabled: true
      connection:
        secret: object-storage-secret
        key: connection
`

// locateGitLabChart returns the path of the GitLab chart archive under
// HELM_CHARTS for CHART_VERSION, skipping the test when either is not
// available. CI and `task unit-tests` set both variables; locally, run
// `task retrieve-charts` first.
func locateGitLabChart(t *testing.T) string {
	t.Helper()

	chartsDir := os.Getenv("HELM_CHARTS")
	chartVersion := os.Getenv("CHART_VERSION")

	if chartsDir == "" || chartVersion == "" {
		skipOrFailInCI(t, "HELM_CHARTS and CHART_VERSION are not set; run the tests via `task unit-tests`")
	}

	chartPath, err := LocateChart(chartsDir, gitlabChartName, chartVersion)
	if err != nil {
		skipOrFailInCI(t,
			fmt.Sprintf("GitLab chart %s is not available under %s; run `task retrieve-charts`", chartVersion, chartsDir))
	}

	return chartPath
}

// skipOrFailInCI skips the test locally but fails it in CI, where a missing
// chart means a broken pipeline setup, not a developer convenience.
func skipOrFailInCI(t *testing.T, message string) {
	t.Helper()

	if os.Getenv("CI") != "" {
		t.Fatal(message)
	}

	t.Skip(message)
}

// renderGitLabChart renders the full GitLab chart with the minimal values.
func renderGitLabChart(t *testing.T) *Result {
	t.Helper()

	values := support.Values{}
	require.NoError(t, values.AddFromYAML(gitlabChartValues))

	result, err := Render(Request{
		ChartPath:   locateGitLabChart(t),
		ReleaseName: "test",
		Namespace:   "default",
		Values:      values,
		APIVersions: gitlabChartAPIVersions,
	})

	require.NoError(t, err)

	return result
}

func TestRenderGitLabChart(t *testing.T) {
	result := renderGitLabChart(t)

	t.Run("renders without warnings", func(t *testing.T) {
		for _, warning := range result.Warnings {
			assert.Fail(t, "unexpected warning", warning.String())
		}
	})

	t.Run("renders the core workloads as regular objects", func(t *testing.T) {
		assert.NotNil(t, findObject(result, "Deployment", "webservice"), "webservice Deployment")
		assert.NotNil(t, findObject(result, "Deployment", "sidekiq"), "sidekiq Deployment")
		assert.NotNil(t, findObject(result, "Job", "migrations"), "migrations Job")
		assert.NotNil(t, findObject(result, "StatefulSet", "gitaly"), "gitaly StatefulSet")
	})

	t.Run("renders the capability-gated kinds on their current api version", func(t *testing.T) {
		// These three kinds pick their group version from
		// Capabilities.APIVersions.Has and fall back to a retired group,
		// which renders without an error and only fails on apply.
		for _, expected := range []struct{ kind, apiVersion string }{
			{"PodDisruptionBudget", "policy/v1"},
			{"HorizontalPodAutoscaler", "autoscaling/v2"},
			{"CronJob", "batch/v1"},
		} {
			obj := findByKind(result, expected.kind)

			if assert.NotNil(t, obj, expected.kind) {
				assert.Equal(t, expected.apiVersion, obj.GetAPIVersion(), expected.kind)
			}
		}
	})

	t.Run("renders nothing into a retired group version", func(t *testing.T) {
		// The pins above cover only the kinds known to branch today. This
		// covers the render as a whole, so a probe added by a later chart
		// version fails here even though nothing named it.
		rendered := slices.Clone(result.Objects)
		rendered = append(rendered, result.CRDs...)

		for _, hook := range result.Hooks {
			rendered = append(rendered, hook.Object)
		}

		require.NotEmpty(t, rendered)

		for _, obj := range rendered {
			assert.NotContains(t, retiredGroupVersions, obj.GetAPIVersion(),
				"%s %s renders into a group version no supported cluster serves",
				obj.GetKind(), obj.GetName())
		}
	})

	t.Run("renders at least one of every core kind", func(t *testing.T) {
		// The chart delegates Secret creation to the shared-secrets hooks and
		// exposes HTTP endpoints through the Gateway API instead of Ingress.
		for _, kind := range []string{"ConfigMap", "Service", "ServiceAccount", "Gateway", "HTTPRoute"} {
			assert.NotNil(t, findByKind(result, kind), kind)
		}
	})

	t.Run("keeps the objects sorted by kind", func(t *testing.T) {
		kinds := []string{}

		for _, obj := range result.Objects {
			kinds = append(kinds, obj.GetKind())
		}

		assert.Less(t, kindIndex(t, kinds, "ServiceAccount"), kindIndex(t, kinds, "ConfigMap"))
		assert.Less(t, kindIndex(t, kinds, "ConfigMap"), kindIndex(t, kinds, "Service"))
		assert.Less(t, kindIndex(t, kinds, "Service"), kindIndex(t, kinds, "Deployment"))
		assert.Less(t, kindIndex(t, kinds, "Deployment"), kindIndex(t, kinds, "Job"))
	})

	t.Run("keeps the notes of the umbrella chart", func(t *testing.T) {
		// The chart uses its notes to surface deprecations and next steps.
		assert.NotEmpty(t, result.Notes)
		assert.Contains(t, result.Notes, "===")
	})

	t.Run("keeps hooks and objects disjoint", func(t *testing.T) {
		for _, obj := range result.Objects {
			_, found := obj.GetAnnotations()["helm.sh/hook"]

			assert.False(t, found, "object %s/%s carries a hook annotation", obj.GetKind(), obj.GetName())
		}
	})

	t.Run("loads the definitions the subcharts ship under crds", func(t *testing.T) {
		require.NotEmpty(t, result.CRDs)

		kinds := map[string]bool{}

		for _, crd := range result.CRDs {
			kinds[crd.GetKind()] = true
		}

		assert.True(t, kinds["CustomResourceDefinition"])
	})

	t.Run("models the shared-secrets hooks with their metadata", func(t *testing.T) {
		require.NotEmpty(t, result.Hooks)

		// The template path is the unambiguous identity: the self-signed
		// variant renders from self-signed-cert-job.yml with a similar name.
		job := findHookByPath(result, "shared-secrets/job.yaml")

		require.NotNil(t, job, "shared-secrets Job hook")
		assert.Equal(t, "Job", job.Object.GetKind())
		assert.True(t, job.HasEvent("pre-install"))
		assert.True(t, job.HasEvent("pre-upgrade"))
		assert.Zero(t, job.Weight)
		assert.Contains(t, job.DeletePolicies, "hook-succeeded")
		assert.Contains(t, job.DeletePolicies, "before-hook-creation")

		serviceAccount := findHook(result, "ServiceAccount", "shared-secrets")

		require.NotNil(t, serviceAccount, "shared-secrets ServiceAccount hook")
		assert.Equal(t, -5, serviceAccount.Weight)
	})

	t.Run("orders the pre-upgrade hooks for execution", func(t *testing.T) {
		hooks := result.HooksFor("pre-upgrade")

		require.NotEmpty(t, hooks)

		previous := hooks[0]

		for _, hook := range hooks[1:] {
			assert.True(t, hook.HasEvent("pre-upgrade"))
			assert.GreaterOrEqual(t, hook.Weight, previous.Weight, "weights are non-decreasing")

			if hook.Weight == previous.Weight {
				assert.GreaterOrEqual(t, hook.Object.GetName(), previous.Object.GetName(),
					"names break ties within a weight")
			}

			previous = hook
		}
	})

	t.Run("keeps test hooks out of the upgrade events", func(t *testing.T) {
		for _, hook := range result.HooksFor("test") {
			assert.False(t, hook.HasEvent("pre-upgrade"))
			assert.False(t, hook.HasEvent("post-upgrade"))
		}
	})

	t.Run("renders deterministically", func(t *testing.T) {
		again := renderGitLabChart(t)

		require.Len(t, again.Objects, len(result.Objects))
		require.Len(t, again.Hooks, len(result.Hooks))

		for i, obj := range result.Objects {
			assert.Equal(t, objectKey(obj.GetKind(), obj.GetName()), objectKey(again.Objects[i].GetKind(), again.Objects[i].GetName()))
		}

		// Hooks are compared by template path, which is deterministic and
		// distinct per hook: the helm test Pod carries a randomized name
		// suffix by chart design, so names cannot be compared.
		for i, hook := range result.Hooks {
			assert.Equal(t, hook.Path, again.Hooks[i].Path)
			assert.Equal(t, hook.Weight, again.Hooks[i].Weight)
			assert.Equal(t, hook.Events, again.Hooks[i].Events)
		}
	})
}

// findObject returns the first object of the given kind whose name contains
// the given fragment.
func findObject(result *Result, kind, nameFragment string) *unstructured.Unstructured {
	for _, obj := range result.Objects {
		if obj.GetKind() == kind && strings.Contains(obj.GetName(), nameFragment) {
			return obj
		}
	}

	return nil
}

// findHook returns the first hook of the given object kind whose name
// contains the given fragment.
func findHook(result *Result, kind, nameFragment string) *Hook {
	for i := range result.Hooks {
		obj := result.Hooks[i].Object

		if obj.GetKind() == kind && strings.Contains(obj.GetName(), nameFragment) {
			return &result.Hooks[i]
		}
	}

	return nil
}

// findHookByPath returns the first hook whose template path contains the
// given fragment.
func findHookByPath(result *Result, pathFragment string) *Hook {
	for i := range result.Hooks {
		if strings.Contains(result.Hooks[i].Path, pathFragment) {
			return &result.Hooks[i]
		}
	}

	return nil
}

// objectKey formats a kind and name for comparison.
func objectKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}
