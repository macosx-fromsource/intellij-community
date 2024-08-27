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
      serviceMonitor:
        enabled: true
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

redis:
  metrics:
    serviceMonitor:
      enabled: true
`
)

var _ = Describe("Monitoring", func() {
	var chartValues support.Values
	var serviceMonitors, podMonitors []client.Object

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

		It("Should contain all Monitoring resources", func() {
			Expect(serviceMonitors).To(HaveLen(len(serviceMonitorComponentMap)))
			Expect(podMonitors).To(HaveLen(len(podMonitorComponentMap)))
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
