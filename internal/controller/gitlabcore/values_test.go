package gitlabcore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("EffectiveValues", func() {
	When("the hostname carries a subdomain", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Hostname = "gitlab.example.com"

		values, err := EffectiveValues(core)

		It("names the GitLab host and derives the domain the other hosts hang off", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(hostsGitLabNameKey)).To(Equal("gitlab.example.com"))
			Expect(values.GetString(hostsDomainKey)).To(Equal("example.com"))
		})
	})

	When("the hostname is an apex domain", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Hostname = "example.com"

		values, err := EffectiveValues(core)

		It("uses it as the domain, because its first label is not a subdomain", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(hostsGitLabNameKey)).To(Equal("example.com"))
			Expect(values.GetString(hostsDomainKey)).To(Equal("example.com"))
		})
	})

	When("no hostname is set", func() {
		core := &apiv2alpha1.GitLabCore{}

		values, err := EffectiveValues(core)

		It("leaves the chart hosts alone", func() {
			Expect(err).To(BeNil())
			Expect(values.HasKey(hostsGitLabNameKey)).To(BeFalse())
			Expect(values.HasKey(hostsDomainKey)).To(BeFalse())
		})
	})

	When("a license is referenced", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.License = &apiv2alpha1.LicenseSpec{
			SecretRef: apiv2alpha1.SecretKeySelector{Name: "gitlab-license", Key: "license"},
		}

		values, err := EffectiveValues(core)

		It("points the chart at the Secret", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(licenseSecretKey)).To(Equal("gitlab-license"))
			Expect(values.GetString(licenseKeyKey)).To(Equal("license"))
		})
	})

	When("the free-form values set a key the structured fields also map to", func() {
		userValues := support.Values{}
		_ = userValues.SetValue(hostsDomainKey, "administrator.example.com")

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Hostname = "gitlab.example.com"
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("keeps the value of the administrator", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(hostsDomainKey)).To(Equal("administrator.example.com"))
		})

		It("still maps the structured fields the free-form values leave out", func() {
			Expect(values.GetString(hostsGitLabNameKey)).To(Equal("gitlab.example.com"))
		})
	})

	When("nothing is specified", func() {
		core := &apiv2alpha1.GitLabCore{}

		values, err := EffectiveValues(core)

		It("mirrors the shared secrets defaults of the v1beta1 controller", func() {
			Expect(err).To(BeNil())
			Expect(values.HasKey(sharedSecretsCreateRBACKey)).To(BeTrue())
			Expect(values.GetBool(sharedSecretsCreateRBACKey)).To(BeFalse())
			Expect(values.HasKey(sharedSecretsCreateSAKey)).To(BeTrue())
			Expect(values.GetBool(sharedSecretsCreateSAKey)).To(BeFalse())
			Expect(values.GetString(sharedSecretsSANameKey)).To(Equal(settings.ManagerServiceAccount))
			Expect(values.GetString(sharedSecretsRunAsUserKey)).To(BeEmpty())
			Expect(values.GetString(sharedSecretsFSGroupKey)).To(BeEmpty())
		})
	})

	When("the free-form values replace the shared secrets service account", func() {
		userValues := support.Values{}
		_ = userValues.SetValue(sharedSecretsCreateSAKey, true)
		_ = userValues.SetValue(sharedSecretsSANameKey, "own-account")

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("keeps them, because the mirrored values are defaults", func() {
			Expect(err).To(BeNil())
			Expect(values.GetBool(sharedSecretsCreateSAKey)).To(BeTrue())
			Expect(values.GetString(sharedSecretsSANameKey)).To(Equal("own-account"))
		})
	})

	When("the free-form values ask for cert-manager and the GitLab Runner", func() {
		userValues := support.Values{}
		_ = userValues.SetValue(installCertmanagerKey, true)
		_ = userValues.SetValue(installRunnerKey, true)

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("overrides them, because the Operator owns what they configure", func() {
			Expect(err).To(BeNil())
			Expect(values.GetBool(installCertmanagerKey)).To(BeFalse())
			Expect(values.GetBool(installRunnerKey)).To(BeFalse())
		})
	})

	When("the free-form values ask for a bundled ingress controller", func() {
		userValues := support.Values{}
		_ = userValues.SetValue(installNGINXKey, true)
		_ = userValues.SetValue(installNGINXGeoKey, true)
		_ = userValues.SetValue(installHAProxyKey, true)
		_ = userValues.SetValue(installTraefikKey, true)
		_ = userValues.SetValue(installEnvoyKey, true)

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("overrides them, because the Operator installs no ingress controller", func() {
			Expect(err).To(BeNil())
			Expect(values.GetBool(installNGINXKey)).To(BeFalse())
			Expect(values.GetBool(installNGINXGeoKey)).To(BeFalse())
			Expect(values.GetBool(installHAProxyKey)).To(BeFalse())
			Expect(values.GetBool(installTraefikKey)).To(BeFalse())
			Expect(values.GetBool(installEnvoyKey)).To(BeFalse())
		})
	})

	When("the free-form values are merged", func() {
		userValues := support.Values{}
		_ = userValues.SetValue("global.psql.host", "psql.example.com")

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Hostname = "gitlab.example.com"
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("does not write into the specification, which comes from the informer cache", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString("global.psql.host")).To(Equal("psql.example.com"))
			Expect(userValues.HasKey(hostsGitLabNameKey)).To(BeFalse())

			// The merged values are a copy, so changing them leaves the
			// resource untouched.
			Expect(values.SetValue("global.psql.host", "other.example.com")).To(Succeed())
			Expect(userValues.GetString("global.psql.host")).To(Equal("psql.example.com"))
		})
	})
})
