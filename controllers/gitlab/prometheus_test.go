package gitlab

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	prometheusName   = "prometheus"
	alertmanagerName = "alertmanager"
	nodeExporterName = "prometheus-node-exporter"
	pushgatewayName  = "prometheus-pushgateway"
)

var _ = Describe("Prometheus", func() {
	var (
		enablePrometheus, enableAlertmanager, enableNodeExporter, enablePushgateway  bool
		chartValues                                                                  support.Values
		wantPrometheus                                                               bool
		configMaps, services, deployments, daemonSets, statefulSets, ingresses, pvcs []client.Object
	)

	JustBeforeEach(func() {
		_ = chartValues.SetValue("prometheus.install", enablePrometheus)
		_ = chartValues.SetValue("prometheus.alertmanager.enabled", enableAlertmanager)

		template, adapter := prometheusTemplate(chartValues, enableNodeExporter, enablePushgateway)

		wantPrometheus = adapter.WantsComponent(component.Prometheus)
		configMaps = PrometheusConfigMaps(template)
		services = PrometheusServices(template)
		deployments = PrometheusDeployments(template)
		daemonSets = PrometheusDaemonSets(template)
		statefulSets = PrometheusStatefulSets(template)
		ingresses = PrometheusIngresses(template)
		pvcs = PrometheusPersistentVolumeClaims(template)
	})

	When("All Prometheus components are enabled", func() {
		BeforeEach(func() {
			enablePrometheus = true
			enableAlertmanager = true
			enablePushgateway = true
			enableNodeExporter = true

			chartValues = support.Values{}
			_ = chartValues.SetValue("prometheus.server.ingress.enabled", true)
		})

		It("Should contain Prometheus resources", func() {
			Expect(wantPrometheus).To(BeTrue())
			Expect(services).To(
				matchPrometheusElements(Not(BeNil()), prometheusName, alertmanagerName, nodeExporterName, pushgatewayName),
			)
			Expect(configMaps).To(
				matchAllPrometheusElements(Not(BeNil()), prometheusName, alertmanagerName),
			)
			Expect(pvcs).To(
				Or(
					matchAllPrometheusElements(Not(BeNil()), prometheusName),                   // GitLab chart 9.0+
					matchAllPrometheusElements(Not(BeNil()), prometheusName, alertmanagerName), // GitLab chart < 9
				),
			)
			Expect(deployments).To(
				Or(
					matchAllPrometheusElements(Not(BeNil()), prometheusName, pushgatewayName),                   // GitLab chart 9.0+
					matchAllPrometheusElements(Not(BeNil()), prometheusName, alertmanagerName, pushgatewayName), // GitLab chart < 9
				),
			)
			Expect(daemonSets).To(
				matchAllPrometheusElements(Not(BeNil()), nodeExporterName),
			)
			Expect(ingresses).To(
				matchAllPrometheusElements(Not(BeNil()), prometheusName),
			)
			Expect(statefulSets).To(
				Or(
					matchAllPrometheusElements(Not(BeNil()), alertmanagerName), // GitLab chart 9.0+
					BeEmpty(), // GitLab chart < 9
				),
			)
		})
	})

	When("Prometheus server statefulset enabled", func() {
		BeforeEach(func() {
			enablePrometheus = true
			enableAlertmanager = false
			enablePushgateway = false
			enableNodeExporter = false

			chartValues = support.Values{}
			_ = chartValues.SetValue("prometheus.server.statefulSet.enabled", true)
		})

		It("Should contain Prometheus resources", func() {
			Expect(statefulSets).To(matchAllPrometheusElements(Not(BeNil()), prometheusName))
			// check for additional headless service is created
			Expect(services).To(HaveLen(2))
			Expect(wantPrometheus).To(BeTrue())
			Expect(pvcs).To(BeEmpty())
			Expect(deployments).To(BeEmpty())
			Expect(ingresses).To(BeEmpty())
			Expect(daemonSets).To(BeEmpty())
		})
	})

	When("Prometheus chart is disabled", func() {
		BeforeEach(func() {
			enablePrometheus = false
			chartValues = support.Values{}
		})

		It("Should not contain Prometheus resources", func() {
			Expect(wantPrometheus).To(BeFalse())
			Expect(configMaps).To(BeEmpty())
			Expect(services).To(BeEmpty())
			Expect(deployments).To(BeEmpty())
			Expect(pvcs).To(BeEmpty())
			Expect(ingresses).To(BeEmpty())
			Expect(daemonSets).To(BeEmpty())
			Expect(statefulSets).To(BeEmpty())
		})
	})
})

func matchPrometheusElements(match OmegaMatcher, components ...string) OmegaMatcher {
	return MatchElements(prometheusComponent, AllowDuplicates, matchAllElements(match, components...))
}

func matchAllPrometheusElements(match OmegaMatcher, components ...string) OmegaMatcher {
	return MatchAllElements(prometheusComponent, matchAllElements(match, components...))
}

func prometheusTemplate(values support.Values, enableNodeExporter bool, enablePushgateway bool) (helm.Template, gitlab.Adapter) {
	_ = values.SetValue("prometheus.nodeExporter.enabled", enableNodeExporter)
	_ = values.SetValue("prometheus.pushgateway.enabled", enablePushgateway)

	mockGitLab := CreateMockGitLab(releaseName, namespace, values)
	adapter := CreateMockAdapter(mockGitLab)
	template, err := GetTemplate(adapter)

	// Re-render chart with new value format on Prometheus error.
	// This can be the default without fallbacks once Operator only supports Chart 9.0+.
	if err != nil && strings.Contains(err.Error(), "Detected deprecated values for the Prometheus subchart.") {
		delete(values["prometheus"].(map[string]interface{}), "nodeExporter")
		delete(values["prometheus"].(map[string]interface{}), "pushgateway")

		_ = values.SetValue("prometheus.prometheus-node-exporter.enabled", enableNodeExporter)
		_ = values.SetValue("prometheus.prometheus-pushgateway.enabled", enablePushgateway)

		mockGitLab = CreateMockGitLab(releaseName, namespace, values)
		adapter = CreateMockAdapter(mockGitLab)
		template, err = GetTemplate(adapter)
	}

	Expect(err).To(BeNil())
	Expect(template).NotTo(BeNil())

	return template, adapter
}

func prometheusComponent(elements interface{}) string {
	if obj, ok := elements.(client.Object); ok {
		// Label used by Prometheus components pre GitLab chart 9.0
		if v, ok := obj.GetLabels()["component"]; ok {
			return map[string]string{
				"server":        prometheusName,
				"node-exporter": nodeExporterName,
				"pushgateway":   pushgatewayName,
				"alertmanager":  alertmanagerName,
			}[v]
		}

		return obj.GetLabels()["app.kubernetes.io/name"]
	} else {
		return ""
	}
}
