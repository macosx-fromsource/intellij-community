package charts

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func newTestChart(name, version, appVersion string) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       name,
			Version:    version,
			AppVersion: appVersion,
		},
	}
}

func TestGitlabOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitLab Operator Framework: Charts Support")
}
