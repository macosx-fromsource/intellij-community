package gitlabcore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("renderRelease", func() {
	When("the resource names a chart version the Operator carries", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})

		release, err := renderRelease(core, chartsDirectory(), mockCapabilities())

		It("renders the objects of the GitLab chart", func() {
			Expect(err).To(BeNil())

			webservice := objects.First(release.Objects, objects.And(
				objects.ByKind("Deployment"), objects.ByComponent("webservice")))

			Expect(webservice).NotTo(BeNil())
			Expect(webservice.GetName()).To(Equal(releaseName + "-webservice-default"))
		})

		It("keeps the hooks out of the objects", func() {
			Expect(err).To(BeNil())
			Expect(objects.First(release.Objects, objects.ByKind("Secret"))).To(BeNil())

			hooks := release.HooksFor(hookEventPreInstall)
			Expect(hooks).NotTo(BeEmpty())
		})

		// The post-install hooks of the chart belong to the NGINX admission
		// webhook patch and the Traefik dashboard. This is what lets the
		// reconciler run the pre-install event alone.
		It("installs no ingress controller, so nothing fires on post-install", func() {
			Expect(err).To(BeNil())

			Expect(release.HooksFor("post-install")).To(BeEmpty())
			Expect(objects.First(release.Objects, objects.ByKind("IngressClass"))).To(BeNil())
			Expect(objects.First(release.Objects, objects.ByKind("GatewayClass"))).To(BeNil())
		})

		It("stamps the release labels, which the finalizer sweeps by", func() {
			Expect(err).To(BeNil())

			webservice := objects.First(release.Objects, objects.ByKind("Deployment"))
			Expect(webservice.GetLabels()).To(HaveKeyWithValue(render.ReleaseNameLabel, releaseName))
			Expect(webservice.GetLabels()).To(HaveKeyWithValue(render.ReleaseNamespaceLabel, testNamespace))
		})

		It("renders the hostname of the resource as the GitLab host", func() {
			Expect(err).To(BeNil())

			// The chart routes through the Gateway API by default, so the host
			// of the instance shows up on an HTTPRoute rather than an Ingress.
			route := objects.First(release.Objects, objects.And(
				objects.ByKind("HTTPRoute"), objects.ByName(releaseName+"-gitlab")))

			Expect(route).NotTo(BeNil())

			hostnames, _, err := unstructured.NestedStringSlice(route.Object, "spec", "hostnames")
			Expect(err).To(BeNil())
			Expect(hostnames).To(ContainElement(testHostname))
		})

		It("derives the hosts of the other components from the domain of the hostname", func() {
			Expect(err).To(BeNil())

			route := objects.First(release.Objects, objects.And(
				objects.ByKind("HTTPRoute"), objects.ByName(releaseName+"-registry")))

			Expect(route).NotTo(BeNil())

			hostnames, _, err := unstructured.NestedStringSlice(route.Object, "spec", "hostnames")
			Expect(err).To(BeNil())
			Expect(hostnames).To(ContainElement("registry.example.com"))
		})
	})

	When("the chart version is empty", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		core.Spec.Chart.Version = ""

		_, err := renderRelease(core, chartsDirectory(), mockCapabilities())

		It("reports that the version is required", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.chart.version is required"))
		})
	})

	When("the chart version is one the Operator does not carry", func() {
		core := CreateMockGitLabCore(releaseName, testNamespace, support.Values{})
		core.Spec.Chart.Version = "0.0.1"

		_, err := renderRelease(core, chartsDirectory(), mockCapabilities())

		It("reports the versions that are available", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("chart gitlab version 0.0.1 not found"))
			Expect(err.Error()).To(ContainSubstring(chartVersion()))
		})
	})
})
