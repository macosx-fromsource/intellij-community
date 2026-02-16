package helm

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Query", func() {
	template, err := loadTemplate()

	labels := map[string]string{
		"app.kubernetes.io/managed-by": "Helm",
		"app.kubernetes.io/name":       "test",
	}

	It("must cache the query", func() {
		Expect(err).To(BeNil())

		cache := cacheBackdoor(template.Query())
		saveCacheSize := len(*cache)

		deployments := template.Query().ObjectsByKindAndLabels("Deployment", labels)

		Expect(len(*cache)).To(Equal(saveCacheSize + 1))

		cachedDeployments := template.Query().ObjectsByKindAndLabels("Deployment", labels)

		Expect(len(*cache)).To(Equal(saveCacheSize + 1))

		Expect(deployments).To(Equal(cachedDeployments))
	})

	It("must return empty list for a non-existent kind specifiers", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKind("Ingress.v1beta1.networking.k8s.io")).To(BeEmpty())
	})

	It("must return the same objects with different equivalent kind specifiers", func() {
		Expect(err).To(BeNil())

		list1 := template.Query().ObjectsByKind("Ingress")
		list2 := template.Query().ObjectsByKind("Ingress.networking.k8s.io")

		Expect(list1).To(Equal(list2))
	})

	It("must return the same object that matches the kind specifier and has the specified name", func() {
		Expect(err).To(BeNil())

		obj1 := template.Query().ObjectByKindAndName("Deployment", "ephemeral-test")
		obj2 := template.Query().ObjectByKindAndName("Deployment.apps", "ephemeral-test")
		obj3 := template.Query().ObjectByKindAndName("Deployment.v1.apps", "ephemeral-test")

		Expect(obj1).To(Equal(obj2))
		Expect(obj2).To(Equal(obj3))
	})

	It("must return the same object that matches the kind specifier and has the specified labels", func() {
		Expect(err).To(BeNil())

		list1 := template.Query().ObjectsByKindAndLabels("ConfigMap", labels)
		list2 := template.Query().ObjectsByKindAndLabels("ConfigMap.v1", labels)

		Expect(list1).To(Equal(list2))
	})

	It("must return ConfigMaps that match the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("ConfigMap", labels)).
			To(HaveLen(1))
	})

	It("must return empty list when there is no ConfigMap that matchs the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("ConfigMap", map[string]string{
			"foo": "bar",
		})).
			To(BeEmpty())
	})

	It("must return nil when there is no ConfigMap that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("ConfigMap", "does-not-exist")).
			To(BeNil())
	})

	It("must return the ConfigMap that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("ConfigMap", "ephemeral-test")).
			NotTo(BeNil())
	})

	It("must return Secrets that match the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("Secret", labels)).
			To(HaveLen(1))
	})

	It("must return empty list when there is no Secret that matchs the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("Secret", map[string]string{
			"foo": "bar",
		})).
			To(BeEmpty())
	})

	It("must return nil when there is no Secret that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("Secret", "does-not-exist")).
			To(BeNil())
	})

	It("must return the Secret that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("Secret", "ephemeral-test")).
			NotTo(BeNil())
	})

	It("must return Deployments that match the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("Deployment", labels)).
			To(HaveLen(1))
	})

	It("must return empty list when there is no Deployment that matchs the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("Deployment", map[string]string{
			"foo": "bar",
		})).
			To(BeEmpty())
	})

	It("must return nil when there is no Deployment that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("Deployment", "does-not-exist")).
			To(BeNil())
	})

	It("must return the Deployment that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("Deployment", "ephemeral-test")).
			NotTo(BeNil())
	})

	It("must return Services that match the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("Service", labels)).
			To(HaveLen(1))
	})

	It("must return empty list when there is no Service that matchs the labels", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectsByKindAndLabels("Service", map[string]string{
			"foo": "bar",
		})).
			To(BeEmpty())
	})

	It("must return nil when there is no Service that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("Service", "does-not-exist")).
			To(BeNil())
	})

	It("must return the Service that matches the name", func() {
		Expect(err).To(BeNil())

		Expect(template.Query().ObjectByKindAndName("Service", "ephemeral-test")).
			NotTo(BeNil())
	})
})
