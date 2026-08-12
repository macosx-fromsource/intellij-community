package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
)

func TestCapabilities(t *testing.T) {
	// The Helm defaults are the client-go scheme, which registers group
	// versions no current cluster serves. One leaking into a supplied set
	// makes the chart render an object that cannot be applied.
	const removedFromKubernetes = "extensions/v1beta1"

	require.Contains(t, common.DefaultVersionSet, removedFromKubernetes,
		"the Helm defaults are expected to carry group versions a cluster does not serve")

	t.Run("a supplied set replaces the helm defaults", func(t *testing.T) {
		caps := capabilities(Request{APIVersions: []string{"policy/v1", "policy/v1/PodDisruptionBudget"}})

		assert.Equal(t, common.VersionSet{"policy/v1", "policy/v1/PodDisruptionBudget"}, caps.APIVersions)
		assert.False(t, caps.APIVersions.Has(removedFromKubernetes),
			"a supplied set must be able to say a group version is absent")
	})

	t.Run("the helm defaults stand when nothing is supplied", func(t *testing.T) {
		caps := capabilities(Request{})

		assert.Equal(t, common.DefaultVersionSet, caps.APIVersions)
	})

	t.Run("the request is the only source of the api versions", func(t *testing.T) {
		// The renderer reads no process state, so composing the
		// operator-wide override with a discovered set is the caller's job.
		t.Setenv("GITLAB_OPERATOR_KUBERNETES_API_VERSIONS", "example.com/v1")
		t.Setenv("GITLAB_OPERATOR_KUBERNETES_VERSION", "v1.11.0")

		caps := capabilities(Request{})

		assert.Equal(t, common.DefaultVersionSet, caps.APIVersions)
		assert.Equal(t, common.DefaultCapabilities.KubeVersion, caps.KubeVersion)
	})

	t.Run("the kube version comes from the request or the helm default", func(t *testing.T) {
		requested := &common.KubeVersion{Version: "v1.34.0", Major: "1", Minor: "34"}

		assert.Equal(t, *requested, capabilities(Request{KubeVersion: requested}).KubeVersion)
		assert.Equal(t, common.DefaultCapabilities.KubeVersion, capabilities(Request{}).KubeVersion)
	})

	t.Run("the helm default set is left unmodified", func(t *testing.T) {
		// caps.APIVersions aliases the package global when nothing is
		// supplied, so an append would corrupt every later render.
		before := len(common.DefaultVersionSet)

		_ = capabilities(Request{})
		_ = capabilities(Request{APIVersions: []string{"policy/v1"}})

		assert.Len(t, common.DefaultVersionSet, before)
	})
}
