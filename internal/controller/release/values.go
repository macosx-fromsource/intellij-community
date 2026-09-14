package release

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// MergeUserValues merges the free-form chart values of a specification over the
// values derived from its structured fields. The free-form values win, as ADR 26
// decides: a structured field that maps to the same key as a value an
// administrator sets does not override that choice, which keeps the free-form
// values a working escape hatch.
//
// The user values are copied before they are merged. They come from the informer
// cache, and both the merge and the renderer write into the map they are given.
func MergeUserValues(values support.Values, userValues map[string]interface{}) error {
	if userValues == nil {
		return nil
	}

	copied, err := copystructure.Copy(userValues)
	if err != nil {
		return errors.Wrap(err, "failed to copy spec.chart.values")
	}

	typedValues, ok := copied.(map[string]interface{})
	if !ok {
		return errors.New("spec.chart.values is not an object")
	}

	if err := values.Merge(typedValues); err != nil {
		return errors.Wrap(err, "failed to merge spec.chart.values")
	}

	return nil
}
