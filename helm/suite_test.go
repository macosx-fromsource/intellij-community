package helm

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/runtime"
	kubectlscheme "k8s.io/kubectl/pkg/scheme"
	applicationv1beta1 "sigs.k8s.io/application/api/v1beta1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts/populate"
)

func loadTemplate() (Template, error) {
	values := support.Values{}
	_ = values.AddFromYAMLFile("testdata/chart/values.yaml")

	builder, err := NewBuilder(charts.GlobalCatalog())
	if err != nil {
		return nil, err
	}

	return builder.Render(values)
}

func TestHelm(t *testing.T) {
	_ = charts.PopulateGlobalCatalog(
		populate.WithSearchPath("testdata/chart"))

	runtime.Must(applicationv1beta1.AddToScheme(kubectlscheme.Scheme))

	RegisterFailHandler(Fail)
	RunSpecs(t, "Helm Suite")
}
