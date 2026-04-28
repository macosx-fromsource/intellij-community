package gitlab

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	globalMinioEnabled = "global.minio.enabled"
)

var _ = Describe("Enabling or disabling internal MinIO", func() {
	BeforeEach(func() {
		if !IsChartVersionOlderThan(helm.GetChartVersion(), ChartVersion100) {
			Skip("Chart does not support MinIO anymore")
		}
	})

	chartValuesDefault := support.Values{}

	chartValuesEnabled := support.Values{}
	_ = chartValuesEnabled.SetValue(globalMinioEnabled, true)

	chartValuesDisabled := support.Values{}
	chartValuesDisabledString := `
global:
  minio:
    enabled: false
  appConfig:
    object_store:
      enabled: true
      connection:
        secret: gitlab-external-storage
        key: connection
gitlab:
  toolbox:
    backups:
      objectStorage:
        config:
          secret: s3cmd-config
          key: config
`

	_ = chartValuesDisabled.AddFromYAML(chartValuesDisabledString)

	tests := map[string]struct {
		chartValues support.Values
		expected    bool
	}{
		"enabled (default)": {chartValues: chartValuesDefault, expected: true},
		"enabled":           {chartValues: chartValuesEnabled, expected: true},
		"disabled":          {chartValues: chartValuesDisabled, expected: false},
	}

	for name, test := range tests {
		// Must assign a copy of the loop variable to a local variable:
		// https://onsi.github.io/ginkgo/#dynamically-generating-specs
		name := name
		test := test

		When(name, func() {
			mockGitLab := CreateMockGitLab(releaseName, namespace, test.chartValues)
			adapter := CreateMockAdapter(mockGitLab)
			template, err := GetTemplate(adapter)
			enabled := adapter.WantsComponent(component.MinIO)

			It("Should render the template", func() {
				Expect(err).To(BeNil())
				Expect(template).NotTo(BeNil())
			})

			It(fmt.Sprintf("Should have %s internal MinIO", name), func() {
				Expect(enabled).To(Equal(test.expected))
			})
		})
	}
})
