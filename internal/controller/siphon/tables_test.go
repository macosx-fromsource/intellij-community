package siphon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

func TestResolveTablesSource(t *testing.T) {
	// Auto extracts everywhere. The version of the cluster is only one of the two
	// preconditions for mounting the image, and the other one, a runtime that
	// serves image volumes, is not reported anywhere.
	t.Run("Auto extracts", func(t *testing.T) {
		assert.Equal(t, apiv2alpha1.TablesSourceConfigMap,
			resolveTablesSource(apiv2alpha1.TablesSourceAuto))
	})

	t.Run("treats an unset source as Auto", func(t *testing.T) {
		assert.Equal(t, apiv2alpha1.TablesSourceConfigMap, resolveTablesSource(""))
	})

	t.Run("an explicit source always wins", func(t *testing.T) {
		// An administrator whose cluster and runtime do serve image volumes says
		// so, which is the only way to reach that path now.
		assert.Equal(t, apiv2alpha1.TablesSourceImageVolume,
			resolveTablesSource(apiv2alpha1.TablesSourceImageVolume))
		assert.Equal(t, apiv2alpha1.TablesSourceConfigMap,
			resolveTablesSource(apiv2alpha1.TablesSourceConfigMap))
	})
}

func TestResolveTablesImage(t *testing.T) {
	t.Run("builds the tag from the application version", func(t *testing.T) {
		assert.Equal(t,
			"registry.gitlab.com/gitlab-org/gitlab/gitlab-siphon-tables:v19.2.0-ee",
			resolveTablesImage(newSiphon(), "19.2.0"))
	})

	t.Run("a reference in the specification wins", func(t *testing.T) {
		siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
			s.Spec.Tables.Image = "registry.example.com/tables:pinned"
		})

		assert.Equal(t, "registry.example.com/tables:pinned", resolveTablesImage(siphon, "19.2.0"))
	})
}

func TestTablesMountPath(t *testing.T) {
	t.Run("is the directory the definitions live in inside the image", func(t *testing.T) {
		// A ConfigMap has no directories: its keys land flat in the mount, so the
		// mount has to move down from the image root to where the files are, and
		// it has to match the --tables-dir flag of the init container.
		assert.Equal(t, "/etc/tables/db/siphon/tables/", tablesMountPath)
	})
}

func TestMountTables(t *testing.T) {
	t.Run("swaps the image volume on every workload of the release", func(t *testing.T) {
		// No chart value can do this: the chart emits the image volume
		// unconditionally in split mode and appends the volumes a value supplies.
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		require.NoError(t, mountTables(result.Objects, tablesConfigMapName(siphon.Name)))

		workloads := objects.Filter(result.Objects, objects.ByKind("Deployment"))
		require.Len(t, workloads, 3)

		for _, workload := range workloads {
			volumes, found, err := unstructured.NestedSlice(
				workload.Object, "spec", "template", "spec", "volumes")
			require.NoError(t, err)
			require.True(t, found)

			swapped := false

			for _, item := range volumes {
				volume, ok := item.(map[string]interface{})
				require.True(t, ok)

				if volume["name"] != tablesVolumeName {
					continue
				}

				assert.NotContains(t, volume, "image", workload.GetName())
				assert.Equal(t, map[string]interface{}{"name": tablesConfigMapName(siphon.Name)},
					volume["configMap"], workload.GetName())

				swapped = true
			}

			assert.True(t, swapped, "%s has no %s volume", workload.GetName(), tablesVolumeName)
		}
	})

	t.Run("repoints the init container at the directory the keys land in", func(t *testing.T) {
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		require.NoError(t, mountTables(result.Objects, tablesConfigMapName(siphon.Name)))

		producer := objects.First(result.Objects, objects.And(
			objects.ByKind("Deployment"), objects.ByName(producerDeployment(siphon.Name))))
		require.NotNil(t, producer)

		containers, found, err := unstructured.NestedSlice(
			producer.Object, "spec", "template", "spec", "initContainers")
		require.NoError(t, err)
		require.True(t, found)

		repointed := false

		for _, item := range containers {
			container, ok := item.(map[string]interface{})
			require.True(t, ok)

			mounts, ok := container["volumeMounts"].([]interface{})
			if !ok {
				continue
			}

			for _, entry := range mounts {
				mount, ok := entry.(map[string]interface{})
				require.True(t, ok)

				if mount["name"] != tablesVolumeName {
					continue
				}

				assert.Equal(t, tablesMountPath, mount["mountPath"])

				repointed = true
			}
		}

		assert.True(t, repointed, "no init container mounts %s", tablesVolumeName)
	})

	t.Run("leaves the other objects of the release alone", func(t *testing.T) {
		siphon := newSiphon()
		result := renderTestRelease(t, siphon)

		before := objects.Filter(result.Objects, objects.ByKind("ConfigMap"))
		snapshot := make([]*unstructured.Unstructured, 0, len(before))

		for _, obj := range before {
			snapshot = append(snapshot, obj.DeepCopy())
		}

		require.NoError(t, mountTables(result.Objects, tablesConfigMapName(siphon.Name)))

		assert.Equal(t, snapshot, objects.Filter(result.Objects, objects.ByKind("ConfigMap")))
	})
}

func TestTablesConfigMapName(t *testing.T) {
	t.Run("is scoped to the resource, so it is always ownable", func(t *testing.T) {
		// It lives in the namespace of the resource, which is what lets garbage
		// collection remove it and keeps the finalizer out of it.
		assert.Equal(t, "siphon-siphon-tables", tablesConfigMapName("siphon"))
		assert.NotEqual(t, tablesConfigMapName("one"), tablesConfigMapName("two"))
	})
}
