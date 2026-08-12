package objects

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// fixtureChartPath is the hermetic fixture chart of the render package, used
// where a test needs rendered rather than hand-built objects without
// depending on a retrieved GitLab chart.
const fixtureChartPath = "../testdata/chart/test"

func TestSetPaused(t *testing.T) {
	t.Run("sets spec.paused on a Deployment", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, SetPaused(deployment, true))

		paused, found, err := unstructured.NestedBool(deployment.Object, "spec", "paused")

		require.NoError(t, err)
		assert.True(t, found)
		assert.True(t, paused)
	})

	t.Run("rejects other kinds", func(t *testing.T) {
		job := testJob()

		err := SetPaused(job, true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Job")
	})
}

func TestReplicas(t *testing.T) {
	t.Run("SetReplicas sets spec.replicas", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, SetReplicas(deployment, 0))

		replicas, found, err := unstructured.NestedInt64(deployment.Object, "spec", "replicas")

		require.NoError(t, err)
		assert.True(t, found)
		assert.Zero(t, replicas)
	})

	t.Run("UnsetReplicas removes spec.replicas", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, SetReplicas(deployment, 2))
		require.NoError(t, UnsetReplicas(deployment))

		_, found, err := unstructured.NestedInt64(deployment.Object, "spec", "replicas")

		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("UnsetReplicas tolerates a missing spec", func(t *testing.T) {
		deployment := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		}}

		require.NoError(t, UnsetReplicas(deployment))
	})

	t.Run("UnsetReplicas preserves the sibling spec fields", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, SetReplicas(deployment, 2))

		template, _, err := unstructured.NestedFieldNoCopy(deployment.Object, "spec", "template")
		require.NoError(t, err)

		require.NoError(t, UnsetReplicas(deployment))

		// The pod template read before the call is still the map inside the
		// object, so handles stay valid: a write through the old handle is
		// visible in the object.
		templateMap, ok := template.(map[string]interface{})
		require.True(t, ok)

		templateMap["marker"] = "still-aliased"

		marker, _, err := unstructured.NestedString(deployment.Object, "spec", "template", "marker")

		require.NoError(t, err)
		assert.Equal(t, "still-aliased", marker)
	})

	t.Run("SetReplicas rejects negative counts", func(t *testing.T) {
		require.Error(t, SetReplicas(testDeployment(), -1))
	})
}

func TestGetReplicas(t *testing.T) {
	t.Run("distinguishes an absent field from an explicit count", func(t *testing.T) {
		deployment := testDeployment()

		_, found, err := GetReplicas(deployment)

		require.NoError(t, err)
		assert.False(t, found)

		require.NoError(t, SetReplicas(deployment, 0))

		replicas, found, err := GetReplicas(deployment)

		require.NoError(t, err)
		assert.True(t, found)
		assert.Zero(t, replicas)
	})

	t.Run("reads the count of a freshly rendered Deployment", func(t *testing.T) {
		// The hand-built objects above store an int64 by construction. Only
		// a real render exercises the decoder, which makes GetReplicas fail
		// on every scale-pinning Deployment if it yields float64.
		values := support.Values{}
		require.NoError(t, values.SetValue("replicaCount", 3))

		result, err := render.Render(render.Request{
			ChartPath:   fixtureChartPath,
			ReleaseName: "myrel",
			Namespace:   "testns",
			Values:      values,
		})
		require.NoError(t, err)

		deployment := First(result.Objects, ByKind(deploymentKind))
		require.NotNil(t, deployment)

		replicas, found, err := GetReplicas(deployment)

		require.NoError(t, err)
		assert.True(t, found, "the fixture chart pins the replica count")
		assert.Equal(t, int64(3), replicas)
	})

	t.Run("rejects other kinds", func(t *testing.T) {
		_, _, err := GetReplicas(testJob())

		require.Error(t, err)
	})
}

func TestSetPodTemplateAnnotation(t *testing.T) {
	t.Run("sets the annotation on the pod template", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, SetPodTemplateAnnotation(deployment, "gitlab.com/last-restart", "20260806000000"))

		annotations, found, err := unstructured.NestedStringMap(
			deployment.Object, "spec", "template", "metadata", "annotations")

		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "20260806000000", annotations["gitlab.com/last-restart"])
	})

	t.Run("accepts Jobs", func(t *testing.T) {
		job := testJob()

		require.NoError(t, SetPodTemplateAnnotation(job, "checksum/secret-rails", "abc"))
	})

	t.Run("rejects non-workload kinds", func(t *testing.T) {
		service := testObject("Service", "svc", nil)

		require.Error(t, SetPodTemplateAnnotation(service, "key", "value"))
	})
}

func TestInitContainerEnv(t *testing.T) {
	t.Run("UpsertInitContainerEnv adds a new variable", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, UpsertInitContainerEnv(deployment, "dependencies", "BYPASS_SCHEMA_VERSION", "true"))
		assert.Equal(t, map[string]string{
			"CONFIG_TEMPLATE_DIRECTORY": "/var/opt/gitlab/templates",
			"BYPASS_SCHEMA_VERSION":     "true",
		}, containerEnv(t, deployment, "initContainers", "dependencies"))
	})

	t.Run("UpsertInitContainerEnv replaces an existing variable", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, UpsertInitContainerEnv(deployment, "dependencies", "CONFIG_TEMPLATE_DIRECTORY", "/tmp"))
		assert.Equal(t, map[string]string{
			"CONFIG_TEMPLATE_DIRECTORY": "/tmp",
		}, containerEnv(t, deployment, "initContainers", "dependencies"))
	})

	t.Run("RemoveInitContainerEnv removes the variable", func(t *testing.T) {
		deployment := testDeployment()

		require.NoError(t, UpsertInitContainerEnv(deployment, "dependencies", "BYPASS_SCHEMA_VERSION", "true"))
		require.NoError(t, RemoveInitContainerEnv(deployment, "dependencies", "BYPASS_SCHEMA_VERSION"))
		assert.Equal(t, map[string]string{
			"CONFIG_TEMPLATE_DIRECTORY": "/var/opt/gitlab/templates",
		}, containerEnv(t, deployment, "initContainers", "dependencies"))
	})

	t.Run("fails when the init container is absent", func(t *testing.T) {
		deployment := testDeployment()

		err := UpsertInitContainerEnv(deployment, "certificates", "KEY", "value")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificates")
	})

	t.Run("collapses duplicate entries into the upserted one", func(t *testing.T) {
		deployment := testDeploymentWithEnv([]interface{}{
			map[string]interface{}{"name": "DUP", "value": "one"},
			map[string]interface{}{"name": "KEEP", "value": "kept"},
			map[string]interface{}{"name": "DUP", "value": "two"},
		})

		require.NoError(t, UpsertInitContainerEnv(deployment, "dependencies", "DUP", "replaced"))
		assert.Equal(t, []interface{}{
			map[string]interface{}{"name": "DUP", "value": "replaced"},
			map[string]interface{}{"name": "KEEP", "value": "kept"},
		}, rawInitContainerEnv(t, deployment, "dependencies"))
	})

	t.Run("removes every duplicate occurrence", func(t *testing.T) {
		deployment := testDeploymentWithEnv([]interface{}{
			map[string]interface{}{"name": "DUP", "value": "one"},
			map[string]interface{}{"name": "KEEP", "value": "kept"},
			map[string]interface{}{"name": "DUP", "value": "two"},
		})

		require.NoError(t, RemoveInitContainerEnv(deployment, "dependencies", "DUP"))
		assert.Equal(t, []interface{}{
			map[string]interface{}{"name": "KEEP", "value": "kept"},
		}, rawInitContainerEnv(t, deployment, "dependencies"))
	})

	t.Run("removal leaves an absent env untouched and drops an emptied one", func(t *testing.T) {
		deployment := testDeploymentWithEnv(nil)
		container, err := findContainer(deployment, "initContainers", "dependencies")
		require.NoError(t, err)
		delete(container, "env")

		require.NoError(t, RemoveInitContainerEnv(deployment, "dependencies", "ANY"))
		assert.NotContains(t, container, "env")

		container["env"] = []interface{}{map[string]interface{}{"name": "ONLY", "value": "v"}}

		require.NoError(t, RemoveInitContainerEnv(deployment, "dependencies", "ONLY"))
		assert.NotContains(t, container, "env")
	})

	t.Run("replaces a valueFrom entry with a literal wholesale", func(t *testing.T) {
		deployment := testDeploymentWithEnv([]interface{}{
			map[string]interface{}{
				"name": "FROM_FIELD",
				"valueFrom": map[string]interface{}{
					"fieldRef": map[string]interface{}{"fieldPath": "metadata.name"},
				},
			},
		})

		require.NoError(t, UpsertInitContainerEnv(deployment, "dependencies", "FROM_FIELD", "literal"))
		assert.Equal(t, []interface{}{
			map[string]interface{}{"name": "FROM_FIELD", "value": "literal"},
		}, rawInitContainerEnv(t, deployment, "dependencies"))
	})

	t.Run("fails loudly on malformed shapes", func(t *testing.T) {
		envIsAMap := testDeploymentWithEnv(nil)
		container, err := findContainer(envIsAMap, "initContainers", "dependencies")
		require.NoError(t, err)

		container["env"] = map[string]interface{}{"OOPS": "shape"}

		require.Error(t, UpsertInitContainerEnv(envIsAMap, "dependencies", "KEY", "value"))
		require.Error(t, RemoveInitContainerEnv(envIsAMap, "dependencies", "KEY"))

		containersNotAList := testDeployment()
		require.NoError(t, unstructured.SetNestedField(
			containersNotAList.Object, "not-a-list", "spec", "template", "spec", "initContainers"))

		err = UpsertInitContainerEnv(containersNotAList, "dependencies", "KEY", "value")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a list")

		malformedEntry := testDeployment()
		require.NoError(t, unstructured.SetNestedField(
			malformedEntry.Object, []interface{}{"not-a-map"}, "spec", "template", "spec", "initContainers"))

		require.Error(t, UpsertInitContainerEnv(malformedEntry, "dependencies", "KEY", "value"))
	})
}

func TestUpsertEnvInAllContainers(t *testing.T) {
	t.Run("sets the variable on every container", func(t *testing.T) {
		job := testJob()

		require.NoError(t, UpsertEnvInAllContainers(job, "SKIP_POST_DEPLOYMENT_MIGRATIONS", "true"))

		for _, name := range []string{"migrations", "helper"} {
			env := containerEnv(t, job, "containers", name)

			assert.Equal(t, "true", env["SKIP_POST_DEPLOYMENT_MIGRATIONS"], "container %s", name)
		}
	})

	t.Run("fails without containers", func(t *testing.T) {
		deployment := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		}}

		require.Error(t, UpsertEnvInAllContainers(deployment, "KEY", "value"))
	})
}

// testDeployment builds a Deployment with a "dependencies" init container,
// mirroring the shape the GitLab chart renders for webservice and sidekiq.
func testDeployment() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name": "gitlab-webservice-default",
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"initContainers": []interface{}{
						map[string]interface{}{
							"name": "dependencies",
							"env": []interface{}{
								map[string]interface{}{
									"name":  "CONFIG_TEMPLATE_DIRECTORY",
									"value": "/var/opt/gitlab/templates",
								},
							},
						},
					},
					"containers": []interface{}{
						map[string]interface{}{"name": "webservice"},
					},
				},
			},
		},
	}}
}

// testDeploymentWithEnv builds a Deployment whose "dependencies" init
// container carries the given env list.
func testDeploymentWithEnv(env []interface{}) *unstructured.Unstructured {
	deployment := testDeployment()

	container, err := findContainer(deployment, "initContainers", "dependencies")
	if err != nil {
		panic(err)
	}

	container["env"] = env

	return deployment
}

// rawInitContainerEnv returns the raw env list of the named init container,
// preserving duplicates, ordering, and valueFrom entries.
func rawInitContainerEnv(t *testing.T, obj *unstructured.Unstructured, containerName string) []interface{} {
	t.Helper()

	container, err := findContainer(obj, "initContainers", containerName)
	require.NoError(t, err)

	env, _ := container["env"].([]interface{})

	return env
}

// testJob builds a Job with two containers without env entries.
func testJob() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name": "gitlab-migrations",
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "migrations"},
						map[string]interface{}{"name": "helper"},
					},
				},
			},
		},
	}}
}

// containerEnv returns the env entries of the named container as a plain map.
func containerEnv(t *testing.T, obj *unstructured.Unstructured, listName, containerName string) map[string]string {
	t.Helper()

	container, err := findContainer(obj, listName, containerName)
	require.NoError(t, err)

	env, _ := container["env"].([]interface{})
	variables := map[string]string{}

	for _, item := range env {
		variable, ok := item.(map[string]interface{})
		require.True(t, ok)

		name, _ := variable["name"].(string)
		value, _ := variable["value"].(string)
		variables[name] = value
	}

	return variables
}
