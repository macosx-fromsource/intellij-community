package objects

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// imageVolumeDeployment builds a Deployment shaped like the Siphon chart renders
// in split mode: an OCI image volume mounted at the root of the image tree on an
// init container, and an unrelated volume that must survive untouched.
func imageVolumeDeployment() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "siphon-producer"},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"initContainers": []interface{}{
						map[string]interface{}{
							"name": "generate-cdc-config",
							"volumeMounts": []interface{}{
								map[string]interface{}{
									"name":      "siphon-tables",
									"mountPath": "/etc/tables",
									"readOnly":  true,
								},
								map[string]interface{}{
									"name":      "dynamic-config",
									"mountPath": "/etc/config/dynamic",
								},
							},
						},
					},
					"containers": []interface{}{
						map[string]interface{}{
							"name": "siphon",
							"volumeMounts": []interface{}{
								map[string]interface{}{
									"name":      "dynamic-config",
									"mountPath": "/etc/config/dynamic",
								},
							},
						},
					},
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "siphon-tables",
							"image": map[string]interface{}{
								"reference":  "registry.example.com/tables:v1",
								"pullPolicy": "IfNotPresent",
							},
						},
						map[string]interface{}{
							"name":     "dynamic-config",
							"emptyDir": map[string]interface{}{},
						},
					},
				},
			},
		},
	}}
}

// volumeByName returns the named volume of a pod template.
func volumeByName(t *testing.T, obj *unstructured.Unstructured, name string) map[string]interface{} {
	t.Helper()

	volumes, err := volumeList(obj)
	require.NoError(t, err)

	for _, item := range volumes {
		volume, ok := item.(map[string]interface{})
		require.True(t, ok)

		if volume["name"] == name {
			return volume
		}
	}

	t.Fatalf("no volume %q", name)

	return nil
}

// mountPath returns the mount path of the named volume in the named container.
func mountPath(t *testing.T, obj *unstructured.Unstructured, listName, containerName, volumeName string) string {
	t.Helper()

	container, err := findContainer(obj, listName, containerName)
	require.NoError(t, err)

	mounts, ok := container["volumeMounts"].([]interface{})
	require.True(t, ok)

	for _, item := range mounts {
		mount, ok := item.(map[string]interface{})
		require.True(t, ok)

		if mount["name"] == volumeName {
			path, ok := mount["mountPath"].(string)
			require.True(t, ok)

			return path
		}
	}

	t.Fatalf("container %q does not mount %q", containerName, volumeName)

	return ""
}

func TestReplaceImageVolumeWithConfigMap(t *testing.T) {
	t.Run("swaps the source and repoints the mount", func(t *testing.T) {
		deployment := imageVolumeDeployment()

		require.NoError(t, ReplaceImageVolumeWithConfigMap(
			deployment, "siphon-tables", "siphon-tables-cm", "/etc/tables/db/siphon/tables"))

		volume := volumeByName(t, deployment, "siphon-tables")

		assert.NotContains(t, volume, "image", "the image source must go, a volume carries exactly one")
		assert.Equal(t, map[string]interface{}{"name": "siphon-tables-cm"}, volume["configMap"])

		assert.Equal(t, "/etc/tables/db/siphon/tables",
			mountPath(t, deployment, "initContainers", "generate-cdc-config", "siphon-tables"))
	})

	t.Run("leaves other volumes and mounts alone", func(t *testing.T) {
		deployment := imageVolumeDeployment()

		require.NoError(t, ReplaceImageVolumeWithConfigMap(
			deployment, "siphon-tables", "siphon-tables-cm", "/etc/tables/db/siphon/tables"))

		other := volumeByName(t, deployment, "dynamic-config")

		assert.Contains(t, other, "emptyDir")
		assert.NotContains(t, other, "configMap")

		assert.Equal(t, "/etc/config/dynamic",
			mountPath(t, deployment, "initContainers", "generate-cdc-config", "dynamic-config"))
		assert.Equal(t, "/etc/config/dynamic",
			mountPath(t, deployment, "containers", "siphon", "dynamic-config"))
	})

	t.Run("repoints a mount in the containers as well as the init containers", func(t *testing.T) {
		deployment := imageVolumeDeployment()

		container, err := findContainer(deployment, "containers", "siphon")
		require.NoError(t, err)

		container["volumeMounts"] = append(container["volumeMounts"].([]interface{}),
			map[string]interface{}{"name": "siphon-tables", "mountPath": "/etc/tables"})

		require.NoError(t, ReplaceImageVolumeWithConfigMap(
			deployment, "siphon-tables", "siphon-tables-cm", "/etc/tables/db/siphon/tables"))

		assert.Equal(t, "/etc/tables/db/siphon/tables",
			mountPath(t, deployment, "containers", "siphon", "siphon-tables"))
	})

	t.Run("leaves a workload without the volume untouched", func(t *testing.T) {
		deployment := imageVolumeDeployment()

		before := deployment.DeepCopy()

		require.NoError(t, ReplaceImageVolumeWithConfigMap(
			deployment, "absent", "siphon-tables-cm", "/etc/tables/db/siphon/tables"))

		assert.Equal(t, before, deployment)
	})

	t.Run("leaves a workload without volumes untouched", func(t *testing.T) {
		deployment := testDeployment()

		before := deployment.DeepCopy()

		require.NoError(t, ReplaceImageVolumeWithConfigMap(
			deployment, "siphon-tables", "siphon-tables-cm", "/etc/tables/db/siphon/tables"))

		assert.Equal(t, before, deployment)
	})

	t.Run("rejects a volumes field that is not a list", func(t *testing.T) {
		deployment := imageVolumeDeployment()

		require.NoError(t, unstructured.SetNestedField(
			deployment.Object, "not-a-list", "spec", "template", "spec", "volumes"))

		err := ReplaceImageVolumeWithConfigMap(
			deployment, "siphon-tables", "siphon-tables-cm", "/etc/tables/db/siphon/tables")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not a list")
	})

	t.Run("rejects a kind whose pod template is not at spec.template", func(t *testing.T) {
		cronJob := imageVolumeDeployment()
		cronJob.SetKind("CronJob")

		err := ReplaceImageVolumeWithConfigMap(
			cronJob, "siphon-tables", "siphon-tables-cm", "/etc/tables/db/siphon/tables")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected one of")
	})
}

func TestRepointVolumeMount(t *testing.T) {
	t.Run("rejects a volumeMounts field that is not a list", func(t *testing.T) {
		deployment := imageVolumeDeployment()

		container, err := findContainer(deployment, "containers", "siphon")
		require.NoError(t, err)

		container["volumeMounts"] = "not-a-list"

		err = RepointVolumeMount(deployment, "siphon-tables", "/etc/tables/db/siphon/tables")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not a list")
	})

	t.Run("leaves a container that mounts nothing alone", func(t *testing.T) {
		deployment := imageVolumeDeployment()

		container, err := findContainer(deployment, "containers", "siphon")
		require.NoError(t, err)

		delete(container, "volumeMounts")

		require.NoError(t, RepointVolumeMount(deployment, "siphon-tables", "/etc/tables/db/siphon/tables"))
	})
}
