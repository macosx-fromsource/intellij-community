package capabilities

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestDiscover(t *testing.T) {
	t.Run("reads the version and the served api versions", func(t *testing.T) {
		discoveryClient := fakeDiscovery(t)
		discoveryClient.FakedServerVersion = &version.Info{
			GitVersion: "v1.34.0", Major: "1", Minor: "34",
		}
		discoveryClient.Resources = []*metav1.APIResourceList{{
			GroupVersion: "policy/v1",
			APIResources: []metav1.APIResource{{Kind: "PodDisruptionBudget"}},
		}}

		capabilities, err := Discover(discoveryClient)

		require.NoError(t, err)
		assert.Equal(t, "v1.34.0", capabilities.KubeVersion.Version)
		assert.Equal(t, "1", capabilities.KubeVersion.Major)
		assert.Equal(t, "34", capabilities.KubeVersion.Minor)

		// Charts test both forms, so both have to be present.
		assert.Contains(t, capabilities.APIVersions, "policy/v1")
		assert.Contains(t, capabilities.APIVersions, "policy/v1/PodDisruptionBudget")
	})

	t.Run("keeps the versions that resolved when a group fails", func(t *testing.T) {
		// An aggregated API server that is registered but down makes
		// discovery fail for its group alone. The groups that did resolve are
		// still a usable reading, so that error is tolerated and every other
		// one is not.
		discoveryClient := fakeDiscovery(t)
		discoveryClient.FakedServerVersion = &version.Info{GitVersion: "v1.34.0", Major: "1", Minor: "34"}
		discoveryClient.Resources = []*metav1.APIResourceList{{
			GroupVersion: "policy/v1",
			APIResources: []metav1.APIResource{{Kind: "PodDisruptionBudget"}},
		}}

		// The fake runs this reaction for the resource listing only, so the
		// server version still reads cleanly.
		discoveryClient.PrependReactor("get", "resource", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "metrics.k8s.io", Version: "v1beta1"}: errors.New("service unavailable"),
				},
			}
		})

		capabilities, err := Discover(discoveryClient)

		require.NoError(t, err)
		assert.Contains(t, capabilities.APIVersions, "policy/v1")
	})

	t.Run("lists every kind of a group version once", func(t *testing.T) {
		// A kind can be served by more than one resource at the same group
		// version, and the group version itself arrives from both the group
		// listing and the resource listing. The set carries each entry once.
		discoveryClient := fakeDiscovery(t)
		discoveryClient.FakedServerVersion = &version.Info{GitVersion: "v1.34.0", Major: "1", Minor: "34"}
		discoveryClient.Resources = []*metav1.APIResourceList{{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment"},
				{Name: "deployments/status", Kind: "Deployment"},
				{Name: "statefulsets", Kind: "StatefulSet"},
			},
		}}

		capabilities, err := Discover(discoveryClient)

		require.NoError(t, err)
		assert.Equal(t, []string{"apps/v1", "apps/v1/Deployment", "apps/v1/StatefulSet"}, capabilities.APIVersions)
	})

	t.Run("refuses a discovery that comes back empty", func(t *testing.T) {
		// An API server always serves something, so an empty answer means
		// the client is not talking to one. An empty set would make every
		// capability probe in the chart report false.
		discoveryClient := fakeDiscovery(t)
		discoveryClient.FakedServerVersion = &version.Info{GitVersion: "v1.34.0", Major: "1", Minor: "34"}

		_, err := Discover(discoveryClient)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no api versions")
	})

	t.Run("reports a version that cannot be read", func(t *testing.T) {
		discoveryClient := fakeDiscovery(t)
		discoveryClient.PrependReactor("*", "*", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver unreachable")
		})

		_, err := Discover(discoveryClient)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "server version")
	})
}

// fakeDiscovery returns the discovery client of a fake clientset.
func fakeDiscovery(t *testing.T) *fakediscovery.FakeDiscovery {
	t.Helper()

	discoveryClient, ok := fakeclientset.NewSimpleClientset().Discovery().(*fakediscovery.FakeDiscovery)
	require.True(t, ok)

	return discoveryClient
}
