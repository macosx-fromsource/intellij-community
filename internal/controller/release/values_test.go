package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

func TestMergeUserValues(t *testing.T) {
	t.Run("lets the free-form values win on conflict", func(t *testing.T) {
		// ADR 26: a structured field that maps to the same key as a value an
		// administrator sets does not override that choice, which is what keeps
		// the free-form values a working escape hatch.
		values := support.Values{}
		require.NoError(t, values.SetValue("global.hosts.domain", "derived.example.com"))

		require.NoError(t, MergeUserValues(values, map[string]interface{}{
			"global": map[string]interface{}{
				"hosts": map[string]interface{}{"domain": "chosen.example.com"},
			},
		}))

		assert.Equal(t, "chosen.example.com", stringValue(t, values, "global.hosts.domain"))
	})

	t.Run("keeps the derived siblings of an overridden key", func(t *testing.T) {
		values := support.Values{}
		require.NoError(t, values.SetValue("global.edition", "ee"))
		require.NoError(t, values.SetValue("global.hosts.domain", "derived.example.com"))

		require.NoError(t, MergeUserValues(values, map[string]interface{}{
			"global": map[string]interface{}{
				"hosts": map[string]interface{}{"domain": "chosen.example.com"},
			},
		}))

		assert.Equal(t, "ee", stringValue(t, values, "global.edition"))
	})

	t.Run("does not write into the values it was given", func(t *testing.T) {
		// The user values come from the informer cache, and both the merge and
		// the renderer write into the map they are handed.
		userValues := map[string]interface{}{
			"global": map[string]interface{}{"edition": "ce"},
		}

		values := support.Values{}
		require.NoError(t, values.SetValue("global.hosts.domain", "derived.example.com"))
		require.NoError(t, MergeUserValues(values, userValues))

		// Mutating the merged result must not reach back into the cached map.
		require.NoError(t, values.SetValue("global.edition", "ee"))

		nested, ok := userValues["global"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ce", nested["edition"])
	})

	t.Run("accepts an absent set of free-form values", func(t *testing.T) {
		values := support.Values{}
		require.NoError(t, values.SetValue("global.edition", "ee"))

		require.NoError(t, MergeUserValues(values, nil))

		assert.Equal(t, "ee", stringValue(t, values, "global.edition"))
	})
}

// stringValue reads a dotted key off a values document.
func stringValue(t *testing.T, values support.Values, key string) string {
	t.Helper()

	value, err := values.GetValue(key)
	require.NoError(t, err)

	result, ok := value.(string)
	require.True(t, ok, "%s is %T, not a string", key, value)

	return result
}
