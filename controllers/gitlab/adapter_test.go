package gitlab

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("CustomResourceAdapter", func() {
	if namespace == "" {
		namespace = "default" //nolint:golint,goconst
	}

	mockGitLab := CreateMockGitLab(releaseName, namespace, support.Values{})

	It("retrieve the attributes from GitLab CR", func() {
		adapter := CreateMockAdapter(mockGitLab)

		Expect(adapter.Reference()).To(Equal(fmt.Sprintf("test.%s", namespace)))
		Expect(adapter.Namespace()).To(Equal(namespace))
		Expect(adapter.ReleaseName()).To(Equal(releaseName))
		Expect(adapter.ChartVersion()).To(Equal(helm.GetChartVersion()))
	})

	It("should change the hash when values change", func() {
		adapter := CreateMockAdapter(mockGitLab)

		gitlabCopy := mockGitLab.DeepCopy()

		gitlabCopy.Spec.Chart.Values.Object = map[string]interface{}{
			"foo": "FOO",
			"bar": map[string]interface{}{
				"baz": "BAZ",
			},
		}

		beforeHash := adapter.Hash()

		adapter = CreateMockAdapter(gitlabCopy)

		afterHash := adapter.Hash()

		Expect(beforeHash).NotTo(Equal(afterHash))
	})

	It("should reject unsupported chart versions", func() {
		currentChartVersion := helm.GetChartVersion()
		os.Setenv("CHART_VERSION", "0.0.0")
		mockGitLab := CreateMockGitLab(releaseName, namespace, support.Values{})
		adapter := CreateMockAdapter(mockGitLab)
		os.Setenv("CHART_VERSION", currentChartVersion)

		supported, err := helm.ChartVersionSupported(adapter.ChartVersion())

		Expect(supported).To(BeFalse())
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("chart version 0.0.0 not supported"))
	})

	It("should accept supported chart versions", func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, support.Values{})
		adapter := CreateMockAdapter(mockGitLab)
		supported, err := helm.ChartVersionSupported(adapter.ChartVersion())

		Expect(supported).To(BeTrue())
		Expect(err).To(BeNil())
	})

	It("should render expected RBAC", func() {
		mockGitLab := CreateMockGitLab(releaseName, namespace, support.Values{})
		adapter := CreateMockAdapter(mockGitLab)
		values := adapter.Values()

		expected := map[string]string{
			"global.serviceAccount.name":            settings.AppNonRootServiceAccount,
			"gitlab.webservice.serviceAccount.name": settings.AppAnyUIDServiceAccount,
			"shared-secrets.serviceAccount.name":    settings.ManagerServiceAccount,
			"redis.serviceAccount.name":             settings.AppNonRootServiceAccount,
			"postgresql.serviceAccount.name":        settings.AppNonRootServiceAccount,
			"nginx-ingress.serviceAccount.name":     settings.NGINXServiceAccount,
		}

		for key, value := range expected {
			Expect(values.GetString(key)).To(Equal(value))
		}
	})
})
