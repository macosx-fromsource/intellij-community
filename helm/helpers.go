package helm

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

const (
	GitLabChartName = "gitlab"
)

// AvailableChartVersions lists the version of available GitLab Charts.
func AvailableChartVersions() []string {
	return charts.GlobalCatalog().Versions(GitLabChartName)
}

func ChartVersionSupported(version string) (bool, error) {
	result := charts.GlobalCatalog().Query(charts.WithName(GitLabChartName), charts.WithVersion(version)).Empty()

	if result {
		return false, fmt.Errorf("chart version %s not supported; please use one of the following: %s",
			version, strings.Join(charts.GlobalCatalog().Versions(GitLabChartName), ", "))
	}

	return true, nil
}

func GetChartVersion() string {
	version, found := os.LookupEnv("CHART_VERSION")
	if !found {
		version = AvailableChartVersions()[0]
		_ = os.Setenv("CHART_VERSION", version)
	}

	return version
}

// TrueSelector is an ObjectSelector that selects all objects.
var TrueSelector = func(_ runtime.Object) bool {
	return true
}

// FalseSelector is an ObjectSelector that selects no object.
var FalseSelector = func(_ runtime.Object) bool {
	return false
}

type typeMistmatchError struct {
	expected interface{}
	observed interface{}
}

func (e *typeMistmatchError) Error() string {
	return fmt.Sprintf("expected %T, got %T", e.expected, e.observed)
}

func NewTypeMistmatchError(expected, observed interface{}) error {
	return &typeMistmatchError{
		expected: expected,
		observed: observed,
	}
}

// IsTypeMistmatchError returns true if the error is raised because of type mistmatch.
func IsTypeMistmatchError(err error) bool {
	_, ok := err.(*typeMistmatchError)
	return ok
}
