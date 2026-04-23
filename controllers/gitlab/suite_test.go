package gitlab

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gitlabv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/adapter"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts/populate"
)

const (
	testNamespace  = "default"
	envAPIVersions = "GITLAB_OPERATOR_KUBERNETES_API_VERSIONS"
)

var (
	releaseName = "test"
	namespace   = os.Getenv("HELM_NAMESPACE")
)

func CreateMockGitLab(releaseName, namespace string, customValues support.Values) *gitlabv1beta1.GitLab {
	values := support.Values{}
	_ = values.AddFromYAML(minimalValues())
	_ = values.Merge(customValues)

	return &gitlabv1beta1.GitLab{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.gitlab.com/v1beta1",
			Kind:       "GitLab",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: namespace,
		},
		Spec: gitlabv1beta1.GitLabSpec{
			Chart: gitlabv1beta1.GitLabChartSpec{
				Version: helm.GetChartVersion(),
				Values: gitlabv1beta1.ChartValues{
					Object: values,
				},
			},
		},
	}
}

func CreateMockAdapter(mockGitLab *gitlabv1beta1.GitLab) gitlab.Adapter {
	adapter, _ := adapter.NewV1Beta1(context.TODO(), mockGitLab)

	return adapter
}

func TestGitLab(t *testing.T) {
	// The tests do not have access to a live cluster, so we manually add the
	// Monitoring API version via an environment variable defined in the
	// 'settings' package.
	resetEnv := setAPIVersionsEnv("monitoring.coreos.com/v1")
	defer resetEnv()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	settings.Load()

	_ = charts.PopulateGlobalCatalog(
		populate.WithSearchPath(settings.HelmChartsDirectory))

	runtime.Must(gitlabv1beta1.AddToScheme(scheme.Scheme))

	RegisterFailHandler(Fail)
	RunSpecs(t, "GitLab Suite")
}

func setAPIVersionsEnv(value string) func() {
	beforeVal := os.Getenv(envAPIVersions)

	setEnvOrPanic(envAPIVersions, value)

	return func() {
		setEnvOrPanic(envAPIVersions, beforeVal)
	}
}

func setEnvOrPanic(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		panic(err)
	}
}

func minimalValues() string {
	return `
gitlab:
  toolbox:
    backups:
      cron:
        enabled: true
      objectStorage:
        config:
          secret: backup-storage-secret
          key: config
registry:
  storage:
    secret: registry-storage-secret
    key: config
global:
  redis:
    host: redis.example.com
  psql:
    host: psql.example.com
    password:
      secret: psql-password
      key: password
  appConfig:
    object_store:
      enabled: true
      connection:
        secret: object-storage-secret
        key: connection
`
}
