package kubetests

import (
	"context"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/kubectl/pkg/scheme"
	ctrlrt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestGitlabOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitLab Operator Framework: Kubernetes Client Support")
}

var _ = BeforeSuite(func() {
	UnstructuredYAMLCodec = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	By("Starting test environment")

	ctx, cancel = context.WithCancel(context.TODO())
	env = &envtest.Environment{}

	schemeBuilder := runtime.SchemeBuilder{
		clientgoscheme.AddToScheme,
		monitoringv1.AddToScheme,
		certmanagerv1.AddToScheme,
	}

	cfg, err := env.Start()
	Expect(err).ToNot(HaveOccurred())

	for _, addToScheme := range schemeBuilder {
		Expect(
			addToScheme(scheme.Scheme),
		).NotTo(HaveOccurred())
	}

	Manager, err = ctrlrt.NewManager(cfg, ctrlrt.Options{
		Logger: zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		),
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		Expect(
			Manager.Start(ctx),
		).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("Stopping test environment")
	cancel()
	Expect(
		env.Stop(),
	).ToNot(HaveOccurred())
})

/* Test Helpers */

var (
	ctx    context.Context
	env    *envtest.Environment
	cancel context.CancelFunc
)

var (
	Manager               manager.Manager
	UnstructuredYAMLCodec runtime.Serializer
)

func readObject(name string, decoder ...runtime.Codec) client.Object {
	content, err := os.ReadFile(fmt.Sprintf("testdata/%s.yaml", name))
	Expect(err).NotTo(HaveOccurred())

	decoderToUse := scheme.Codecs.UniversalDeserializer()
	if len(decoder) > 0 {
		decoderToUse = decoder[0]
	}

	obj, gvk, err := decoderToUse.Decode(content, nil, nil)
	Expect(err).NotTo(HaveOccurred())

	result, ok := obj.(client.Object)
	if !ok {
		ref := fmt.Sprintf("%T", obj)
		if gvk != nil {
			ref = gvk.String()
		}

		Fail(fmt.Sprintf("can not convert %s to client object", ref))
	}

	return result
}

func createObject(obj client.Object) func() error {
	return func() error {
		return Manager.GetClient().Create(context.TODO(), obj)
	}
}

func getObject(obj client.Object, minGeneration ...int64) func() error {
	return func() error {
		if err := Manager.GetClient().Get(context.TODO(), client.ObjectKeyFromObject(obj), obj); err != nil {
			return err
		}

		if len(minGeneration) > 0 && obj.GetGeneration() < minGeneration[0] {
			return fmt.Errorf("expected object generation to be greater than %d, found %d",
				minGeneration[0], obj.GetGeneration())
		}

		return nil
	}
}

func deleteObject(obj client.Object) func() error {
	return func() error {
		return Manager.GetClient().Delete(context.TODO(), obj)
	}
}
