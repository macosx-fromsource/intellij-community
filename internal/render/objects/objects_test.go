package objects

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSelection(t *testing.T) {
	webservice := testObject("Deployment", "gitlab-webservice-default", map[string]string{
		appLabel: "webservice",
	})
	sidekiq := testObject("Deployment", "gitlab-sidekiq-all-in-1-v2", map[string]string{
		gitlabComponentLabel: "sidekiq",
	})
	migrations := testObject("Job", "gitlab-migrations", map[string]string{
		appLabel: "migrations",
	})
	service := testObject("Service", "gitlab-webservice-default", map[string]string{
		appLabel: "webservice",
	})

	objs := []*unstructured.Unstructured{webservice, sidekiq, migrations, service}

	t.Run("Filter keeps matching objects in order", func(t *testing.T) {
		matched := Filter(objs, ByKind("Deployment"))

		require.Len(t, matched, 2)
		assert.Same(t, webservice, matched[0])
		assert.Same(t, sidekiq, matched[1])
	})

	t.Run("Partition splits without losing objects", func(t *testing.T) {
		matched, rest := Partition(objs, ByComponent("webservice"))

		require.Len(t, matched, 2)
		require.Len(t, rest, 2)
		assert.Same(t, webservice, matched[0])
		assert.Same(t, service, matched[1])
		assert.Same(t, sidekiq, rest[0])
		assert.Same(t, migrations, rest[1])
	})

	t.Run("First returns the first match or nil", func(t *testing.T) {
		assert.Same(t, migrations, First(objs, ByKind("Job")))
		assert.Nil(t, First(objs, ByKind("StatefulSet")))
	})

	t.Run("ByComponent matches either component label", func(t *testing.T) {
		assert.True(t, ByComponent("webservice")(webservice))
		assert.True(t, ByComponent("sidekiq")(sidekiq))
		assert.False(t, ByComponent("webservice")(sidekiq))
	})

	t.Run("ByName matches the object name", func(t *testing.T) {
		assert.True(t, ByName("gitlab-migrations")(migrations))
		assert.False(t, ByName("gitlab-migrations")(webservice))
	})

	t.Run("MatchLabels requires every label", func(t *testing.T) {
		assert.True(t, MatchLabels(map[string]string{appLabel: "webservice"})(webservice))
		assert.False(t, MatchLabels(map[string]string{
			appLabel: "webservice",
			"tier":   "frontend",
		})(webservice))
	})

	t.Run("And and Or combine predicates", func(t *testing.T) {
		webserviceDeployment := And(ByKind("Deployment"), ByComponent("webservice"))

		assert.True(t, webserviceDeployment(webservice))
		assert.False(t, webserviceDeployment(service))

		workloads := Or(ByComponent("webservice"), ByComponent("sidekiq"))

		assert.True(t, workloads(sidekiq))
		assert.False(t, workloads(migrations))
	})
}

// testObject builds a minimal unstructured object for selection tests.
func testObject(kind, name string, labels map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       kind,
	}}

	obj.SetName(name)
	obj.SetLabels(labels)

	return obj
}
