package gitlab

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

const (
	valuesMonitoringEnabled = `
global:
  pages:
    enabled: true
  praefect:
    enabled: true
  kas:
    enabled: true

registry:
  metrics:
    enabled: true
    serviceMonitor:
      enabled: true

gitlab:
  gitaly:
    metrics:
      serviceMonitor:
        enabled: true
  gitlab-exporter:
    metrics:
      serviceMonitor:
        enabled: true
  gitlab-pages:
    metrics:
      serviceMonitor:
        enabled: true
  gitlab-shell:
    sshDaemon: gitlab-sshd
    metrics:
      enabled: true
      serviceMonitor:
        enabled: true
  kas:
    metrics:
      enabled: true
      podMonitor:
        enabled: true
      serviceMonitor:
        enabled: false
  praefect:
    metrics:
      serviceMonitor:
        enabled: true
  sidekiq:
    metrics:
      podMonitor:
        enabled: true
  webservice:
    metrics:
      serviceMonitor:
        enabled: true
    workhorse:
      metrics:
        enabled: true
        serviceMonitor:
          enabled: true

nginx-ingress:
  controller:
    metrics:
      enabled: true
      serviceMonitor:
        enabled: true

nginx-ingress-geo:
  enabled: true
  controller:
    metrics:
      enabled: true
      serviceMonitor:
        enabled: true
`
)

var _ = Describe("Monitoring", func() {
	var (
		chartValues                  support.Values
		serviceMonitors, podMonitors []client.Object
	)

	JustBeforeEach(func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, chartValues)
		adapter := CreateMockAdapter(mockGitLab)
		template, err := GetTemplate(adapter)

		Expect(err).To(BeNil())
		Expect(template).NotTo(BeNil())

		serviceMonitors = WantedServiceMonitors(adapter, template)
		podMonitors = WantedPodMonitors(adapter, template)
	})

	When("All Monitoring components are enabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
			err := chartValues.AddFromYAML(valuesMonitoringEnabled)
			Expect(err).To(BeNil())
		})

		When("KAS PodMonitor is enabled, ServiceMonitor is disabled", func() {
			BeforeEach(func() {
				Expect(chartValues.SetValue("gitlab.kas.metrics.serviceMonitor.enabled", false)).To(BeNil())
				Expect(chartValues.SetValue("gitlab.kas.metrics.podMonitor.enabled", true)).To(BeNil())
			})

			It("Should contain all Monitoring resources", func() {
				Expect(serviceMonitors).To(HaveLen(len(serviceMonitorComponentMap) - 1))
				Expect(podMonitors).To(HaveLen(len(podMonitorComponentMap)))
			})
		})

		When("KAS PodMonitor is disabled, ServiceMonitor is enabled", func() {
			BeforeEach(func() {
				Expect(chartValues.SetValue("gitlab.kas.metrics.serviceMonitor.enabled", true)).To(BeNil())
				Expect(chartValues.SetValue("gitlab.kas.metrics.podMonitor.enabled", false)).To(BeNil())
			})

			It("Should contain all Monitoring resources", func() {
				Expect(serviceMonitors).To(HaveLen(len(serviceMonitorComponentMap)))
				Expect(podMonitors).To(HaveLen(len(podMonitorComponentMap) - 1))
			})
		})
	})

	When("Monitoring is disabled", func() {
		BeforeEach(func() {
			chartValues = support.Values{}
		})

		It("Should not contain Monitoring resources", func() {
			Expect(serviceMonitors).To(BeEmpty())
			Expect(podMonitors).To(BeEmpty())
		})
	})
})
