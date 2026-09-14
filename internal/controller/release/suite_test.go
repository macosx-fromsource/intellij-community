package release

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// The release under test. The owner is a ConfigMap rather than a custom resource
// so the package is tested without depending on any api/ type, which is the
// point of it taking a client.Object.
const (
	releaseName   = "gitlab"
	testNamespace = "gitlab-system"
)

// observedGenerationSentinel stands in for a controller that has observed the
// current spec. The fake client does not maintain status.observedGeneration, and
// a live workload that has not been observed yet is deliberately not ready.
const observedGenerationSentinel int64 = 1

// testOwner is the resource the applied objects belong to. Any client.Object
// serves; the applier reads only its name, namespace and kind.
func testOwner(namespace string) client.Object {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: namespace},
	}
}

// testClient builds a fake client that resolves the scope of the kinds the tests
// apply. A kind the mapper does not know fails while its scope is resolved,
// which several tests rely on to prove an object was skipped.
func testClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	mapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), apimeta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Service"), apimeta.RESTScopeNamespace)
	mapper.Add(appsv1.SchemeGroupVersion.WithKind(DeploymentKind), apimeta.RESTScopeNamespace)
	mapper.Add(batchv1.SchemeGroupVersion.WithKind("Job"), apimeta.RESTScopeNamespace)
	mapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"), apimeta.RESTScopeRoot)
	mapper.Add(admissionv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration"), apimeta.RESTScopeRoot)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), apimeta.RESTScopeNamespace)

	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithRESTMapper(mapper).
		WithObjects(objects...).
		WithReturnManagedFields().
		Build()
}

// testApplier builds an Applier over an empty fake client.
func testApplier(t *testing.T) *Applier {
	t.Helper()

	return &Applier{
		Client:     testClient(t),
		Scheme:     scheme.Scheme,
		FieldOwner: client.FieldOwner("release-test"),
	}
}

// testLogger returns a logger that routes into the test output.
func testLogger(t *testing.T) logr.Logger {
	t.Helper()

	return testr.New(t)
}

// renderedObject builds a minimal rendered object, carrying the release labels
// the renderer stamps.
func renderedObject(apiVersion, kind, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}

	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetLabels(map[string]string{
		render.ReleaseNameLabel:      releaseName,
		render.ReleaseNamespaceLabel: testNamespace,
	})

	return obj
}

// renderedWorkload builds a live Deployment with the status counts of a finished
// rollout.
func renderedWorkload(t *testing.T, name string, replicas, ready int64) *unstructured.Unstructured {
	t.Helper()

	workload := renderedObject("apps/v1", DeploymentKind, name)
	workload.SetNamespace(testNamespace)

	require.NoError(t, unstructured.SetNestedField(workload.Object, replicas, "spec", "replicas"))
	require.NoError(t, unstructured.SetNestedField(
		workload.Object, observedGenerationSentinel, "status", "observedGeneration"))

	for _, field := range []string{"readyReplicas", "updatedReplicas", "availableReplicas", "replicas"} {
		require.NoError(t, unstructured.SetNestedField(workload.Object, ready, "status", field))
	}

	return workload
}
