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

	When("the Community Edition is selected", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Edition = apiv2alpha1.EditionCE

		values, err := EffectiveValues(core)

		It("switches the chart to it", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(editionKey)).To(Equal("ce"))
		})
	})

	When("no edition is set", func() {
		core := &apiv2alpha1.GitLabCore{}

		values, err := EffectiveValues(core)

		It("leaves the chart default in place", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(editionKey)).To(BeEmpty())
		})
	})

	When("PostgreSQL is specified", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.PostgreSQL = &apiv2alpha1.PostgreSQLSpec{
			Host:              "dev-cluster-rw",
			PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "dev-cluster-app", Key: "password"},
		}

		values, err := EffectiveValues(core)

		It("points the chart at the server and the password Secret", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(psqlHostKey)).To(Equal("dev-cluster-rw"))
			Expect(values.GetString(psqlPasswordSecretKey)).To(Equal("dev-cluster-app"))
			Expect(values.GetString(psqlPasswordKeyKey)).To(Equal("password"))
		})
	})

	When("Redis is specified", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Redis = &apiv2alpha1.RedisSpec{
			Host:              "dev-valkey",
			PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "dev-valkey-auth", Key: "default"},
		}

		values, err := EffectiveValues(core)

		It("points the chart at the server and the auth Secret", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(redisHostKey)).To(Equal("dev-valkey"))
			Expect(values.GetString(redisAuthSecretKey)).To(Equal("dev-valkey-auth"))
			Expect(values.GetString(redisAuthKeyKey)).To(Equal("default"))
		})
	})

	When("object storage is specified", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.ObjectStorage = &apiv2alpha1.ObjectStorageSpec{
			ConnectionSecretRef: apiv2alpha1.SecretKeySelector{
				Name: "gitlab-object-storage", Key: "config",
			},
		}

		values, err := EffectiveValues(core)

		It("turns the consolidated object storage on and points it at the Secret", func() {
			Expect(err).To(BeNil())
			Expect(values.GetBool(objectStoreEnabledKey)).To(BeTrue())
			Expect(values.GetString(objectStoreConnectionSecretKey)).To(Equal("gitlab-object-storage"))
			Expect(values.GetString(objectStoreConnectionKeyKey)).To(Equal("config"))
		})
	})

	When("neither data store is specified", func() {
		core := &apiv2alpha1.GitLabCore{}

		values, err := EffectiveValues(core)

		It("leaves the connections to the free-form values", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(psqlHostKey)).To(BeEmpty())
			Expect(values.GetString(redisHostKey)).To(BeEmpty())
			Expect(values.GetBool(objectStoreEnabledKey)).To(BeFalse())
		})
	})

	When("OpenBao is specified", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.OpenBao = &apiv2alpha1.OpenBaoSpec{
			PostgreSQL: apiv2alpha1.OpenBaoPostgreSQLSpec{
				Host:              "dev-openbao-postgresql",
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "openbao-db-password", Key: "password"},
			},
			ServiceAccount: apiv2alpha1.OpenBaoServiceAccountSpec{Name: "openbao"},
		}

		values, err := EffectiveValues(core)

		It("turns on the GitLab-side integration and the bundled subchart", func() {
			Expect(err).To(BeNil())
			Expect(values.GetBool(openbaoEnabledKey)).To(BeTrue())
			Expect(values.GetBool(openbaoInstallKey)).To(BeTrue())
		})

		It("points the chart at the database and the password Secret", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(openbaoPsqlHostKey)).To(Equal("dev-openbao-postgresql"))
			Expect(values.GetString(openbaoPsqlPasswordSecretKey)).To(Equal("openbao-db-password"))
			Expect(values.GetString(openbaoPsqlPasswordKeyKey)).To(Equal("password"))
		})

		It("leaves the port, database, and username to the chart's own defaults", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(openbaoPsqlPortKey)).To(BeEmpty())
			Expect(values.GetString(openbaoPsqlDatabaseKey)).To(BeEmpty())
			Expect(values.GetString(openbaoPsqlUsernameKey)).To(BeEmpty())
		})

		It("names the pre-existing ServiceAccount and turns off the chart's own RBAC", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(openbaoServiceAccountNameKey)).To(Equal("openbao"))
			Expect(values.GetBool(openbaoServiceAccountCreateKey, true)).To(BeFalse())
			Expect(values.GetBool(openbaoRoleCreateKey, true)).To(BeFalse())
		})
	})

	When("OpenBao specifies a port, database, and username", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.OpenBao = &apiv2alpha1.OpenBaoSpec{
			PostgreSQL: apiv2alpha1.OpenBaoPostgreSQLSpec{
				Host:              "dev-openbao-postgresql",
				Port:              5433,
				Database:          "openbao_db",
				Username:          "openbao_user",
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "openbao-db-password", Key: "password"},
			},
			ServiceAccount: apiv2alpha1.OpenBaoServiceAccountSpec{Name: "openbao"},
		}

		values, err := EffectiveValues(core)

		It("maps every field", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(openbaoPsqlPortKey)).To(Equal("5433"))
			Expect(values.GetString(openbaoPsqlDatabaseKey)).To(Equal("openbao_db"))
			Expect(values.GetString(openbaoPsqlUsernameKey)).To(Equal("openbao_user"))
		})
	})

	When("the free-form values ask the chart to manage OpenBao's own RBAC", func() {
		userValues := support.Values{}
		_ = userValues.SetValue(openbaoServiceAccountCreateKey, true)
		_ = userValues.SetValue(openbaoRoleCreateKey, true)

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.OpenBao = &apiv2alpha1.OpenBaoSpec{
			PostgreSQL: apiv2alpha1.OpenBaoPostgreSQLSpec{
				Host:              "dev-openbao-postgresql",
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "openbao-db-password", Key: "password"},
			},
			ServiceAccount: apiv2alpha1.OpenBaoServiceAccountSpec{Name: "openbao"},
		}
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("keeps the administrator's choice, because the defaults are not overrides", func() {
			Expect(err).To(BeNil())
			Expect(values.GetBool(openbaoServiceAccountCreateKey)).To(BeTrue())
			Expect(values.GetBool(openbaoRoleCreateKey)).To(BeTrue())
		})
	})

	When("OpenBao is not specified", func() {
		core := &apiv2alpha1.GitLabCore{}

		values, err := EffectiveValues(core)

		It("leaves the Secret Manager off", func() {
			Expect(err).To(BeNil())
			Expect(values.HasKey(openbaoEnabledKey)).To(BeFalse())
			Expect(values.HasKey(openbaoInstallKey)).To(BeFalse())
		})
	})

	When("the free-form values name another database than the structured field", func() {
		userValues := support.Values{}
		_ = userValues.SetValue(psqlHostKey, "administrator-cluster-rw")

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.PostgreSQL = &apiv2alpha1.PostgreSQLSpec{
			Host:              "dev-cluster-rw",
			PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "dev-cluster-app", Key: "password"},
		}
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("keeps the host of the administrator and the derived password Secret", func() {
			Expect(err).To(BeNil())
			Expect(values.GetString(psqlHostKey)).To(Equal("administrator-cluster-rw"))
			Expect(values.GetString(psqlPasswordSecretKey)).To(Equal("dev-cluster-app"))
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

	When("nothing asks for a container registry", func() {
		core := &apiv2alpha1.GitLabCore{}

		values, err := EffectiveValues(core)

		It("leaves it off, because its storage is not configured either", func() {
			Expect(err).To(BeNil())
			Expect(values.HasKey(registryEnabledKey)).To(BeTrue())
			Expect(values.GetBool(registryEnabledKey)).To(BeFalse())
		})
	})

	When("the free-form values ask for a container registry", func() {
		userValues := support.Values{}
		_ = userValues.SetValue(registryEnabledKey, true)

		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: userValues}

		values, err := EffectiveValues(core)

		It("gives them one, because the Operator only defaults it off", func() {
			Expect(err).To(BeNil())
			Expect(values.GetBool(registryEnabledKey)).To(BeTrue())
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
