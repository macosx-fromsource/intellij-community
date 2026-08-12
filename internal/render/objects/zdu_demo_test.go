package objects

// This test demonstrates the zero-downtime upgrade choreography on the full
// GitLab chart: render once, partition the objects into apply phases, and
// derive phase-specific variants without touching a cluster. It is the model
// for the future GitLabHelmRelease strategy package.

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// zduChartValues mirror the minimal values of the full-chart render test.
const zduChartValues = `
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

// renderGitLabChart renders the full GitLab chart, skipping the test when the
// chart archive is not available locally and failing in CI, where a missing
// chart means a broken pipeline setup. CI and `task unit-tests` set
// HELM_CHARTS and CHART_VERSION; locally, run `task retrieve-charts` first.
func renderGitLabChart(t *testing.T) *render.Result {
	t.Helper()

	chartsDir := os.Getenv("HELM_CHARTS")
	chartVersion := os.Getenv("CHART_VERSION")

	if chartsDir == "" || chartVersion == "" {
		skipOrFailInCI(t, "HELM_CHARTS and CHART_VERSION are not set; run the tests via `task unit-tests`")
	}

	chartPath, err := render.LocateChart(chartsDir, "gitlab", chartVersion)
	if err != nil {
		skipOrFailInCI(t,
			fmt.Sprintf("GitLab chart %s is not available under %s; run `task retrieve-charts`", chartVersion, chartsDir))
	}

	values := support.Values{}
	require.NoError(t, values.AddFromYAML(zduChartValues))

	result, err := render.Render(render.Request{
		ChartPath:   chartPath,
		ReleaseName: "test",
		Namespace:   "default",
		Values:      values,
	})

	require.NoError(t, err)

	return result
}

// skipOrFailInCI skips the test locally but fails it in CI.
func skipOrFailInCI(t *testing.T, message string) {
	t.Helper()

	if os.Getenv("CI") != "" {
		t.Fatal(message)
	}

	t.Skip(message)
}

func TestZeroDowntimeUpgradePartitioning(t *testing.T) {
	result := renderGitLabChart(t)

	// The snapshot proves at the end that no phase mutated the shared
	// render output: every phase works on deep copies.
	snapshot := make([]*unstructured.Unstructured, 0, len(result.Objects))

	for _, obj := range result.Objects {
		snapshot = append(snapshot, obj.DeepCopy())
	}

	// The upgrade hash stands in for the CR uid and generation that the
	// v1beta1 controller feeds into support.NameWithHashSuffix.
	const upgradeHash = "0f21c-7"

	migrationsJob := First(result.Objects, And(
		ByKind("Job"),
		ByComponent("migrations"),
	))
	require.NotNil(t, migrationsJob, "the chart renders the migrations Job as a regular object")

	t.Run("every rendered Deployment reports its scale", func(t *testing.T) {
		// The phases below only look at the HPA-managed Deployments, where
		// spec.replicas is absent and a decoding bug stays invisible. Most
		// Deployments of the chart pin a count, so cover all of them.
		deployments := Filter(result.Objects, ByKind(deploymentKind))
		require.NotEmpty(t, deployments)

		pinned := 0

		for _, deployment := range deployments {
			replicas, found, err := GetReplicas(deployment)
			require.NoError(t, err, "deployment %s", deployment.GetName())

			if found {
				pinned++

				assert.GreaterOrEqual(t, replicas, int64(0), "deployment %s", deployment.GetName())
			}
		}

		assert.NotZero(t, pinned, "the chart pins the scale of at least one Deployment")
	})

	t.Run("phase 1 derives the pre-migrations Job", func(t *testing.T) {
		preMigrations := migrationsJob.DeepCopy()

		nameWithSuffix, err := support.NameWithHashSuffix(preMigrations.GetName(), upgradeHash, 5)
		require.NoError(t, err)

		preMigrations.SetName(fmt.Sprintf("%s-pre", nameWithSuffix))
		require.NoError(t, UpsertEnvInAllContainers(preMigrations, "SKIP_POST_DEPLOYMENT_MIGRATIONS", "true"))

		assert.NotEqual(t, migrationsJob.GetName(), preMigrations.GetName())
		assertEveryContainerHasEnv(t, preMigrations, "SKIP_POST_DEPLOYMENT_MIGRATIONS", "true")
	})

	t.Run("phase 2 selects and prepares the gated workloads", func(t *testing.T) {
		gated, rest := Partition(result.Objects, And(
			ByKind("Deployment"),
			Or(ByComponent("webservice"), ByComponent("sidekiq")),
		))

		require.NotEmpty(t, gated, "webservice and sidekiq Deployments are present")
		require.NotEmpty(t, rest)

		for _, deployment := range gated {
			prepared := deployment.DeepCopy()

			// The chart leaves spec.replicas absent on HPA-managed
			// Deployments; the strategy reads it to decide scale handling.
			_, found, err := GetReplicas(prepared)
			require.NoError(t, err)
			assert.False(t, found, "replicas of %s are HPA-managed", prepared.GetName())

			require.NoError(t, SetPaused(prepared, true))
			require.NoError(t, UpsertInitContainerEnv(
				prepared, "dependencies", "BYPASS_SCHEMA_VERSION", "true"))

			env := containerEnv(t, prepared, "initContainers", "dependencies")
			assert.Equal(t, "true", env["BYPASS_SCHEMA_VERSION"], "deployment %s", prepared.GetName())
		}

		t.Run("the phases are disjoint and complete", func(t *testing.T) {
			assert.Len(t, result.Objects, len(gated)+len(rest))

			// The selectors alias the input, so pointer identity proves
			// that every object lands in exactly one phase set.
			seen := map[*unstructured.Unstructured]bool{}

			for _, obj := range gated {
				seen[obj] = true
			}

			for _, obj := range rest {
				require.False(t, seen[obj], "object %s in both phase sets", obj.GetName())

				seen[obj] = true
			}

			for _, obj := range result.Objects {
				assert.True(t, seen[obj], "object %s missing from the phase sets", obj.GetName())
			}
		})
	})

	t.Run("phase 3 runs the full migrations Job unchanged", func(t *testing.T) {
		assertNoContainerHasEnv(t, migrationsJob, "SKIP_POST_DEPLOYMENT_MIGRATIONS")
	})

	t.Run("the final rollout is a rendered-object mutation", func(t *testing.T) {
		deployment := First(result.Objects, And(
			ByKind("Deployment"),
			ByComponent("webservice"),
		))
		require.NotNil(t, deployment)

		finalized := deployment.DeepCopy()

		require.NoError(t, SetPodTemplateAnnotation(finalized, "gitlab.com/last-restart", "20260806000000"))
		require.NoError(t, UpsertInitContainerEnv(finalized, "dependencies", "BYPASS_SCHEMA_VERSION", "true"))
		require.NoError(t, RemoveInitContainerEnv(finalized, "dependencies", "BYPASS_SCHEMA_VERSION"))

		env := containerEnv(t, finalized, "initContainers", "dependencies")
		assert.NotContains(t, env, "BYPASS_SCHEMA_VERSION")

		annotations, _, err := unstructured.NestedStringMap(
			finalized.Object, "spec", "template", "metadata", "annotations")
		require.NoError(t, err)
		assert.Equal(t, "20260806000000", annotations["gitlab.com/last-restart"])
	})

	t.Run("the shared render output is untouched", func(t *testing.T) {
		require.Len(t, result.Objects, len(snapshot))

		for i, obj := range result.Objects {
			assert.True(t, reflect.DeepEqual(snapshot[i].Object, obj.Object),
				"object %s/%s changed", obj.GetKind(), obj.GetName())
		}
	})
}

// assertEveryContainerHasEnv asserts the env variable on every container of
// the pod template.
func assertEveryContainerHasEnv(t *testing.T, obj *unstructured.Unstructured, envName, envValue string) {
	t.Helper()

	for _, name := range containerNames(t, obj) {
		env := containerEnv(t, obj, "containers", name)

		assert.Equal(t, envValue, env[envName], "container %s", name)
	}
}

// assertNoContainerHasEnv asserts the env variable is absent from every
// container of the pod template.
func assertNoContainerHasEnv(t *testing.T, obj *unstructured.Unstructured, envName string) {
	t.Helper()

	for _, name := range containerNames(t, obj) {
		env := containerEnv(t, obj, "containers", name)

		assert.NotContains(t, env, envName, "container %s", name)
	}
}

// containerNames lists the container names of the pod template.
func containerNames(t *testing.T, obj *unstructured.Unstructured) []string {
	t.Helper()

	containers, err := containerList(obj, "containers")
	require.NoError(t, err)
	require.NotEmpty(t, containers)

	var names []string

	for _, item := range containers {
		container, ok := item.(map[string]interface{})
		require.True(t, ok)

		name, _ := container[nameField].(string)
		names = append(names, name)
	}

	return names
}
