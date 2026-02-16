package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	globalGeoEnabled       = "global.geo.enabled"
	globalGeoRole          = "global.geo.role"
	gitlabLogCursorEnabled = "gitlab.geo-logcursor.enabled"
)

var _ = Describe("Geo Logcursor", func() {
	var (
		chartValues           support.Values
		configMap, deployment client.Object
		logCursorEnabled      bool
	)

	JustBeforeEach(func() {
		// Geo needs to a use a external PostgreSQL. Set the values to satisfy the
		// config check.
		_ = chartValues.SetValue("global.psql.password.secret", "psql-secret")
		_ = chartValues.SetValue("global.geo.psql.password.secret", "geo-psql-secret")
		_ = chartValues.SetValue("global.psql.host", "psql.example.com")
		_ = chartValues.SetValue("global.geo.psql.host", "geo-psql.example.com")

		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		Expect(err).To(BeNil())
		Expect(template).NotTo(BeNil())

		configMap = GeoLogcursorConfigMap(template)
		deployment = GeoLogcursorDeployment(template)
		logCursorEnabled = adapter.WantsComponent(component.GeoLogcursor)
	})

	When("Geo secondary role is configured", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue(globalGeoEnabled, true)
			_ = chartValues.SetValue(globalGeoRole, "Secondary")
		})

		It("Should enable the logcursor", func() {
			Expect(configMap).ToNot(BeNil())
			Expect(deployment).ToNot(BeNil())
			Expect(logCursorEnabled).To(BeTrue())
		})
	})

	AssertLogCursorDisabled := func() {
		It("Should disable the logcursor", func() {
			Expect(configMap).To(BeNil())
			Expect(deployment).To(BeNil())
			Expect(logCursorEnabled).To(BeFalse())
		})
	}

	When("Geo primary role is configured", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue(globalGeoEnabled, true)
			_ = chartValues.SetValue(globalGeoRole, "primary")
		})

		AssertLogCursorDisabled()
	})

	When("Logcursor chart is disabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			_ = chartValues.SetValue(globalGeoEnabled, true)
			_ = chartValues.SetValue(globalGeoRole, "secondary")
			_ = chartValues.SetValue(gitlabLogCursorEnabled, false)
		})

		AssertLogCursorDisabled()
	})
})
