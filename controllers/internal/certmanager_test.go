package internal

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// fakeAdapter implements only the parts of gitlab.Adapter the issuer reads.
type fakeAdapter struct {
	gitlab.Adapter
	name   types.NamespacedName
	values support.Values
}

func (f fakeAdapter) Name() types.NamespacedName                  { return f.name }
func (f fakeAdapter) ReleaseName() string                         { return f.name.Name }
func (f fakeAdapter) Values() support.Values                      { return f.values }
func (f fakeAdapter) WantsFeature(check gitlab.FeatureCheck) bool { return check(f.values) }

var _ = Describe("Gateway certificate issuer", func() {
	var values support.Values

	parentRef := func() gatewayv1.ParentReference {
		adapter := fakeAdapter{
			name:   types.NamespacedName{Name: "test", Namespace: "gitlab-system"},
			values: values,
		}

		config := GetGatewayIssuerConfig(adapter)
		Expect(config.ACME).NotTo(BeNil())
		Expect(config.ACME.Solvers).To(HaveLen(1))

		refs := config.ACME.Solvers[0].HTTP01.GatewayHTTPRoute.ParentRefs
		Expect(refs).To(HaveLen(1))

		return refs[0]
	}

	BeforeEach(func() {
		values = support.Values{}
		_ = values.SetValue("global.gatewayApi.configureCertmanager", true)
	})

	When("global.gatewayApi.gatewayRef is not set", func() {
		It("targets the chart-managed Gateway in the release namespace", func() {
			ref := parentRef()
			Expect(ref.Name).To(Equal(gatewayv1.ObjectName("test-gw")))
			Expect(ref.Namespace).To(HaveValue(Equal(gatewayv1.Namespace("gitlab-system"))))
		})
	})

	When("global.gatewayApi.gatewayRef is set", func() {
		BeforeEach(func() {
			_ = values.SetValue("global.gatewayApi.gatewayRef.name", "shared-gw")
			_ = values.SetValue("global.gatewayApi.gatewayRef.namespace", "gateways")
		})

		It("targets the referenced Gateway", func() {
			ref := parentRef()
			Expect(ref.Name).To(Equal(gatewayv1.ObjectName("shared-gw")))
			Expect(ref.Namespace).To(HaveValue(Equal(gatewayv1.Namespace("gateways"))))
		})
	})

	When("only global.gatewayApi.gatewayRef.name is set", func() {
		BeforeEach(func() {
			_ = values.SetValue("global.gatewayApi.gatewayRef.name", "shared-gw")
		})

		It("targets the referenced Gateway in the release namespace", func() {
			ref := parentRef()
			Expect(ref.Name).To(Equal(gatewayv1.ObjectName("shared-gw")))
			Expect(ref.Namespace).To(HaveValue(Equal(gatewayv1.Namespace("gitlab-system"))))
		})
	})
})
