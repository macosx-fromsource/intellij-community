package gitlabcore

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/capabilities"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts/populate"
)

const (
	releaseName        = "test"
	testNamespace      = "default"
	testHostname       = "gitlab.example.com"
	testPostgreSQLHost = "psql.example.com"
	testRedisHost      = "redis.example.com"

	testObjectStorageSecret = "object-storage-secret"
)

func TestGitLabCore(t *testing.T) {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	settings.Load()

	// The catalog only backs the error message that lists the chart versions the
	// Operator carries; the renderer reads the archive from disk.
	_ = charts.PopulateGlobalCatalog(
		populate.WithSearchPath(settings.HelmChartsDirectory))

	runtime.Must(apiv2alpha1.AddToScheme(scheme.Scheme))

	RegisterFailHandler(Fail)
	RunSpecs(t, "GitLabCore Suite")
}

// CreateMockGitLabCore builds a GitLabCore that renders the whole chart, with
// the custom values merged over the minimal ones.
func CreateMockGitLabCore(name, namespace string, customValues support.Values) *apiv2alpha1.GitLabCore {
	values := support.Values{}
	_ = values.AddFromYAML(minimalValues())
	_ = values.Merge(customValues)

	return &apiv2alpha1.GitLabCore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.gitlab.com/v2alpha1",
			Kind:       "GitLabCore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: apiv2alpha1.GitLabCoreSpec{
			Hostname: testHostname,
			Edition:  apiv2alpha1.EditionEE,
			PostgreSQL: &apiv2alpha1.PostgreSQLSpec{
				Host: testPostgreSQLHost,
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{
					Name: "psql-password", Key: "password",
				},
			},
			Redis: &apiv2alpha1.RedisSpec{
				Host: testRedisHost,
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{
					Name: "redis-password", Key: "password",
				},
			},
			ObjectStorage: &apiv2alpha1.ObjectStorageSpec{
				ConnectionSecretRef: apiv2alpha1.SecretKeySelector{
					Name: testObjectStorageSecret, Key: "connection",
				},
			},
			Chart: apiv2alpha1.ChartSpec{
				Version: chartVersion(),
				Values: apiv2alpha1.ChartValues{
					Object: values,
				},
			},
		},
	}
}

// chartsDirectory is where the chart archives are staged, which `task
// unit-tests` points HELM_CHARTS at.
func chartsDirectory() string {
	return settings.HelmChartsDirectory
}

// chartVersion is the chart version under test, which `task unit-tests` sets.
func chartVersion() string {
	return os.Getenv("CHART_VERSION")
}

// mockCapabilities stands in for a current cluster with the Prometheus
// operator, the way the internal/render tests do. The renderer replaces the
// Helm defaults with this set, so an entry left out reads as absent to the
// chart.
func mockCapabilities() *capabilities.Capabilities {
	return &capabilities.Capabilities{
		APIVersions: []string{
			"v1",
			"apps/v1",
			"batch/v1",
			"batch/v1/CronJob",
			"autoscaling/v2",
			"autoscaling/v2/HorizontalPodAutoscaler",
			"networking.k8s.io/v1",
			"networking.k8s.io/v1/Ingress",
			"networking.k8s.io/v1/IngressClass",
			"policy/v1",
			"policy/v1/PodDisruptionBudget",
			"monitoring.coreos.com/v1",
		},
	}
}

// mockRESTMapper resolves the scope of the kinds the tests apply. The fake
// client defaults to an empty mapper, which cannot resolve any kind.
func mockRESTMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})

	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Service"), meta.RESTScopeNamespace)
	mapper.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
	mapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"), meta.RESTScopeRoot)
	mapper.Add(admissionv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration"), meta.RESTScopeRoot)

	return mapper
}

// minimalValues are the smallest values that render the chart, mirroring the
// minimal values of the controllers/gitlab suite. The registry storage is
// absent because the reconciler defaults the registry off. The hosts are absent on
// purpose, and so are the PostgreSQL, Redis, and object storage connections:
// spec.hostname, spec.postgresql, spec.redis, and spec.objectStorage supply
// them.
func minimalValues() string {
	return `
certmanager-issuer:
  email: admin@example.com
gitlab:
  toolbox:
    backups:
      cron:
        enabled: true
      objectStorage:
        config:
          secret: backup-storage-secret
          key: config
global:
  pages:
    objectStore:
      connection:
        secret: object-storage-secret
        key: connection
`
}
