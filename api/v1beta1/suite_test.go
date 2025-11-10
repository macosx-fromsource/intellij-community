package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts/populate"
)

func TestV1Beta1(t *testing.T) {
	RegisterFailHandler(Fail)

	settings.Load()
	_ = charts.PopulateGlobalCatalog(
		populate.WithSearchPath(settings.HelmChartsDirectory))

	RunSpecs(t, "GitLab v1beta1 Suite")
}
