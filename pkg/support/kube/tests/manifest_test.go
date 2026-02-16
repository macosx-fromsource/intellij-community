package kubetests

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube/apply"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube/manifest"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/objects"
)

var _ = Describe("DiscoverManagedObjects", func() {
	var (
		discoveryClient discovery.DiscoveryInterface
	)

	BeforeEach(func() {
		/* This can be an expensive operation. Only run it once. */
		if discoveryClient != nil {
			return
		}

		var err error

		cfg := Manager.GetConfig()
		discoveryClient, err = discovery.NewDiscoveryClientForConfig(cfg)
		Expect(err).To(Succeed(), "failed to create discovery client for manager config")
	})

	Context("Options", func() {
		It("fails when required options are not provided", func() {
			dummy := &unstructured.Unstructured{}

			Expect(kube.DiscoverManagedObjects(dummy)).Error().To(HaveOccurred())
			Expect(kube.DiscoverManagedObjects(dummy,
				manifest.WithClient(Manager.GetClient()))).Error().To(HaveOccurred())
			Expect(kube.DiscoverManagedObjects(dummy,
				manifest.AutoDiscovery(),
				manifest.WithClient(Manager.GetClient()))).Error().To(HaveOccurred())
		})
	})

	Context("With AutoDiscovery", func() {
		It("finds objects that reference the owner", func() {
			namespace, objects, owner := prepareManifestTestFixture("with-auto-discovery", 1)

			deployManifestTestFixture(objects, namespace)

			/* Duplicate the same fixture in another namespace. Discovery only
			locates the objects within the same namespace as the owner. */
			deployManifestTestFixture(objects, namespace+"-duplicate")

			children, err := kube.DiscoverManagedObjects(owner,
				manifest.AutoDiscovery(),
				manifest.WithManager(Manager),
				manifest.WithDiscoveryClient(discoveryClient),
			)

			ownedJobRef := fmt.Sprintf("batch/v1/Job:%s/%s-job", namespace, owner.GetName())
			ownedConfigMapRef := fmt.Sprintf("v1/ConfigMap:%s/%s-config", namespace, owner.GetName())

			Expect(err).NotTo(HaveOccurred())
			Expect(children).To(ConsistOf(
				WithTransform(objToStr, Equal(ownedJobRef)),
				WithTransform(objToStr, Equal(ownedConfigMapRef)),
			))
		})
	})

	Context("Without AutoDiscovery", func() {
		It("finds objects that reference the owner", func() {
			namespace, objects, owner := prepareManifestTestFixture("without-auto-discovery", 1)

			deployManifestTestFixture(objects, namespace)

			/* Duplicate the same fixture in another namespace. Discovery only
			   locates the objects within the same namespace as the owner. */
			deployManifestTestFixture(objects, namespace+"-duplicate")

			children, err := kube.DiscoverManagedObjects(owner,
				manifest.WithGroupVersionResourceArgs("deployment.v1.apps", "job.v1.batch"),
				manifest.WithManager(Manager),
			)

			ownedJobRef := fmt.Sprintf("batch/v1/Job:%s/%s-job", namespace, owner.GetName())

			Expect(err).NotTo(HaveOccurred())
			Expect(children).To(ConsistOf(
				WithTransform(objToStr, Equal(ownedJobRef)),
			))
		})
	})

	Context("Cluster-scoped Resources", func() {
		It("finds namespace-scoped objects that reference the owner", func() {
			namespace, objects, owner := prepareManifestTestFixture("cluster-scoped", 1,
				"crontab", "configmap", "ingressclass", "job")

			deployManifestTestFixture(objects, namespace)

			children, err := kube.DiscoverManagedObjects(owner,
				manifest.WithGroupVersionResourceArgs("ingressclass.v1.networking.k8s.io", "job.v1.batch"),
				manifest.WithManager(Manager),
			)

			ownedJobRef := fmt.Sprintf("batch/v1/Job:%s/%s-job", namespace, owner.GetName())

			Expect(err).NotTo(HaveOccurred())
			Expect(children).To(ConsistOf(
				WithTransform(objToStr, Equal(ownedJobRef)),
			))
		})
	})

	Context("With Filters", func() {
		It("finds objects that reference the owner", func() {
			namespace, objects, owner := prepareManifestTestFixture("with-filters", 1)

			deployManifestTestFixture(objects, namespace)

			children, err := kube.DiscoverManagedObjects(owner,
				manifest.WithGroupVersionResourceArgs("deployment.v1.apps", "job.v1.batch"),
				manifest.WithManager(Manager),
				manifest.WithFilters(manifest.IsController),
			)

			ownedJobRef := fmt.Sprintf("batch/v1/Job:%s/%s-job", namespace, owner.GetName())

			Expect(err).NotTo(HaveOccurred())
			Expect(children).To(HaveLen(1))
			Expect(children.First()).To(
				WithTransform(objToStr, Equal(ownedJobRef)),
			)
		})
	})
})

func objToStr(o client.Object) string {
	apiVersion, kind := o.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	return fmt.Sprintf("%s/%s:%s/%s", apiVersion, kind, o.GetNamespace(), o.GetName())
}

//nolint:unparam
func prepareManifestTestFixture(prefix string, index int, resources ...string) (namespace string, objects objects.Collection, owner client.Object) {
	if len(resources) == 0 {
		resources = []string{"crontab", "configmap", "deployment", "job"}
	}

	namespace = fmt.Sprintf("%s-test-%d", prefix, index)
	objects = make([]client.Object, len(resources))

	for i, name := range resources {
		path := fmt.Sprintf("manifest/%s-%d", name, index)
		objects[i] = ReadObject(path, UnstructuredYAMLCodec)
	}

	owner = &unstructured.Unstructured{}
	owner.GetObjectKind().SetGroupVersionKind(objects[0].GetObjectKind().GroupVersionKind())
	owner.SetName(objects[0].GetName())
	owner.SetNamespace(namespace)

	return
}

func deployManifestTestFixture(objects objects.Collection, namespace string) {
	CreateNamespace(namespace)

	for _, obj := range objects {
		o := obj.DeepCopyObject().(client.Object)
		o.SetNamespace(namespace)
		Expect(
			kube.ApplyObject(o, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectCreated))
	}
}
