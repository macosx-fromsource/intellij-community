package v1beta1

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	api "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

var _ = Describe("GitLab Adapter [v1beta1]", func() {
	It("fails if the Chart version is not in semantic version format", func() {
		_, err := NewAdapter(context.TODO(),
			newGitLabResource("foo", support.Values{}))

		Expect(err).To(MatchError(ContainSubstring("invalid version format")))
	})

	It("fails if the Chart version is not supported", func() {
		_, err := NewAdapter(context.TODO(),
			newGitLabResource("0.0.0", support.Values{}))

		Expect(err).To(MatchError(ContainSubstring("gitlab chart version 0.0.0 not found")))
	})

	It("wraps the original resource", func() {
		g := newGitLabResource(getChartVersion(), support.Values{})
		a, err := NewAdapter(context.TODO(), g)

		Expect(err).NotTo(HaveOccurred())
		Expect(a.Name()).To(Equal(types.NamespacedName{
			Name:      "test",
			Namespace: "default",
		}))
		Expect(a.Origin()).To(BeEquivalentTo(g))
	})

	It("uses the original resource object details to generate hash", func() {
		g := newGitLabResource(getChartVersion(), support.Values{})
		a, err := NewAdapter(context.TODO(), g)

		Expect(err).NotTo(HaveOccurred())

		Expect(a.Hash()).To(BeEmpty())

		/* Pretend object is populated by with client */
		g.ObjectMeta.Generation = 1
		g.ObjectMeta.UID = "abcdef"

		h1 := a.Hash()
		Expect(h1).NotTo(BeEmpty())
		Expect(h1).To(Equal("abcdef-1"))

		/* Pretend object is re-populated by with client */
		g.ObjectMeta.Generation = 2

		h2 := a.Hash()
		Expect(h2).NotTo(BeEmpty())
		Expect(h2).NotTo(Equal(h1))
		Expect(h2).To(Equal("abcdef-2"))

	})

	It("uses default values when user-defined values are empty", func() {
		a, err := NewAdapter(context.TODO(),
			newGitLabResource(getChartVersion(), support.Values{}))

		Expect(err).NotTo(HaveOccurred())
		Expect(a).NotTo(BeNil())

		examples := support.Values{}

		addChartDefaultExamples(examples)
		addOperatorDefaultExamples(examples)
		addOperatorOverrideExamples(examples)

		checkValues(a, examples)
		checkLabels(a)
	})

	It("uses user-defined values only when operator does not overwrite it", func() {
		values := support.Values{}

		/* Operator does not override these */
		_ = values.SetValue("certmanager-issuer.email", "pip@greatexpectations.com")
		_ = values.SetValue("global.hosts.domain", "greatexpectations.com")

		/* Operator overrides these */
		_ = values.SetValue("global.serviceAccount.enabled", false)
		_ = values.SetValue("global.ingress.apiVersion", "networking.k8s.io/v1beta1")
		_ = values.SetValue("gitlab-runner.install", true)
		_ = values.SetValue("certmanager.install", true)
		_ = values.SetValue("gitlab.gitlab-shell.service.type", "NodePort")

		a, err := NewAdapter(context.TODO(),
			newGitLabResource(getChartVersion(), values))

		Expect(err).NotTo(HaveOccurred())
		Expect(a).NotTo(BeNil())

		examples := support.Values{}

		addChartDefaultExamples(examples)
		addOperatorDefaultExamples(examples)
		addUserDefinedExamples(examples, support.Values{
			"certmanager-issuer.email": "pip@greatexpectations.com",
			"global.hosts.domain":      "greatexpectations.com",
		})
		addOperatorOverrideExamples(examples)

		checkValues(a, examples)
		checkLabels(a)

		Expect(a.values.GetValue("global.serviceAccount.enabled")).To(BeTrue())
	})

	It("uses user-defined values over operator default values", func() {
		values := support.Values{}
		_ = values.SetValue("gitlab.webservice.serviceAccount.name", "great-service-account")

		a, err := NewAdapter(context.TODO(),
			newGitLabResource(getChartVersion(), values))

		Expect(err).NotTo(HaveOccurred())
		Expect(a).NotTo(BeNil())

		Expect(a.values.GetValue("gitlab.webservice.serviceAccount.name")).To(Equal("great-service-account"))
	})

	It("wants default components and features when not specified otherwise", func() {
		a, err := NewAdapter(context.TODO(),
			newGitLabResource(getChartVersion(), support.Values{}))

		Expect(err).NotTo(HaveOccurred())
		Expect(a).NotTo(BeNil())

		checkEnabledComponents(a,
			component.Gitaly, component.GitLabExporter, component.GitLabShell,
			component.Migrations, component.MinIO, component.NginxIngress,
			component.PostgreSQL, component.Redis, component.Registry,
			component.Sidekiq, component.Toolbox, component.Webservice)
		checkDisabledComponents(a,
			component.GitLabPages, component.Mailroom,
			component.Praefect, component.Spamcheck)
		checkEnabledFeatures(a, ConfigureCertManager)
		checkDisabledFeatures(a, ReplaceGitalyWithPraefect)
	})

	It("only wants components and features that are specified", func() {
		values := support.Values{}

		/* Replace Gitaly with Praefect */
		_ = values.SetValue("global.gitaly.enabled", false)
		_ = values.SetValue("global.praefect.enabled", true)

		/* Disable CertManager, PostgreSQL, Redis */
		_ = values.SetValue("global.ingress.configureCertmanager", false)
		_ = values.SetValue("postgresql.install", false)
		_ = values.SetValue("redis.install", false)

		/*
		 * Enable GitLab Pages, MinIO, Mailroom, and Spamcheck
		 * Mailroom requires more conditions and will not be enabled.
		 */
		_ = values.SetValue("global.pages.enabled", true)
		_ = values.SetValue("global.minio.enabled", true)
		_ = values.SetValue("gitlab.mailroom.enabled", true)
		_ = values.SetValue("global.spamcheck.enabled", true)

		a, err := NewAdapter(context.TODO(),
			newGitLabResource(getChartVersion(), values))

		Expect(err).NotTo(HaveOccurred())
		Expect(a).NotTo(BeNil())

		checkEnabledComponents(a,
			component.GitLabExporter, component.GitLabPages, component.GitLabShell,
			component.Migrations, component.MinIO, component.NginxIngress,
			component.Praefect, component.Registry, component.Sidekiq,
			component.Spamcheck, component.Toolbox, component.Webservice)
		checkDisabledComponents(a,
			component.Gitaly, component.Mailroom,
			component.PostgreSQL, component.Redis)
		checkEnabledFeatures(a, ReplaceGitalyWithPraefect)
		checkDisabledFeatures(a, ConfigureCertManager)
	})
})

/* Helpers */

func newGitLabResource(version string, values support.Values) *api.GitLab {
	return &api.GitLab{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: api.GitLabSpec{
			Chart: api.GitLabChartSpec{
				Version: version,
				Values: api.ChartValues{
					Object: values,
				},
			},
		},
	}
}

func getChartVersion() string {
	version, found := os.LookupEnv("CHART_VERSION")
	if !found {
		version = charts.GlobalCatalog().Versions("gitlab")[0]
	}

	return version
}

func addChartDefaultExamples(examples support.Values) {
	examples["gitlab.gitlab-exporter.enabled"] = true
	examples["gitlab.gitlab-shell.enabled"] = true
	examples["gitlab.mailroom.enabled"] = true
	examples["gitlab.migrations.enabled"] = true
	examples["gitlab.sidekiq.enabled"] = true
	examples["gitlab.toolbox.enabled"] = true
	examples["gitlab.webservice.enabled"] = true
	examples["global.gitaly.enabled"] = true
	examples["global.hosts.domain"] = "example.com"
	examples["global.ingress.configureCertmanager"] = true
	examples["global.ingress.provider"] = "nginx"
	examples["global.pages.enabled"] = false
	examples["global.spamcheck.enabled"] = false
	examples["nginx-ingress.enabled"] = true
	examples["postgresql.install"] = true
	examples["redis.install"] = true
	examples["registry.enabled"] = true
	examples["gitlab.gitaly.securityContext.runAsUser"] = 1000.0
	examples["gitlab.gitlab-exporter.securityContext.fsGroup"] = 1000.0
	examples["gitlab.sidekiq.securityContext.runAsUser"] = 1000.0
}

func addOperatorDefaultExamples(examples support.Values) {
	examples["certmanager-issuer.email"] = "admin@example.com"
	examples["gitlab.webservice.serviceAccount.name"] = settings.AppAnyUIDServiceAccount
}

func addOperatorOverrideExamples(examples support.Values) {
	examples["global.serviceAccount.name"] = settings.AppNonRootServiceAccount
	examples["global.ingress.apiVersion"] = "networking.k8s.io/v1" // ""
	examples["global.serviceAccount.enabled"] = true               // false
	examples["gitlab.gitlab-shell.service.type"] = ""              // "ClusterIP"
	examples["certmanager.install"] = false                        // true
	examples["gitlab-runner.install"] = false                      // true
	examples["shared-secrets.securityContext.runAsUser"] = ""      // 1000
	examples["shared-secrets.securityContext.fsGroup"] = ""        // 1000
	examples["shared-secrets.serviceAccount.name"] = settings.ManagerServiceAccount
}

func addUserDefinedExamples(examples, values support.Values) {
	for key, val := range values {
		examples[key] = val
	}
}

func checkValues(a *Adapter, examples support.Values) {
	for key, val := range examples {
		Expect(a.Values().GetValue(key)).To(Equal(val),
			fmt.Sprintf("Value of `%s` is not correct", key))
	}
}

func checkLabels(a *Adapter) {
	Expect(
		a.Values().GetValue("global.common.labels"),
	).To(
		SatisfyAll(
			HaveKeyWithValue("app.kubernetes.io/name", "test"),
			HaveKeyWithValue("app.kubernetes.io/part-of", "gitlab"),
			HaveKeyWithValue("app.kubernetes.io/managed-by", "gitlab-operator"),
		))

	for _, c := range component.All {
		var labelsKey string

		var checkMoreLabels bool

		switch c {
		case component.NginxIngress:
			labelsKey = "nginx-ingress.labels"
			checkMoreLabels = true
		case component.PostgreSQL:
			labelsKey = "postgresql.commonLabels"
			checkMoreLabels = false
		case component.Redis:
			labelsKey = "redis.commonLabels"
			checkMoreLabels = false
		case component.Registry:
			labelsKey = "registry.common.labels"
			checkMoreLabels = false
		case component.MinIO:
			labelsKey = "minio.common.labels"
			checkMoreLabels = false
		case component.SharedSecrets:
			continue
		default:
			labelsKey = fmt.Sprintf("gitlab.%s.common.labels", c.Name())
			checkMoreLabels = false
		}

		Expect(
			a.Values().GetValue(labelsKey),
		).To(
			SatisfyAny(
				HaveKeyWithValue("gitlab.io/component", c.Name()),
				HaveKeyWithValue("app.kubernetes.io/component", c.Name()),
			),
			fmt.Sprintf("Required labels for component `%s` do not match", c.Name()))

		if checkMoreLabels {
			Expect(
				a.Values().GetValue(labelsKey),
			).To(
				SatisfyAll(
					HaveKeyWithValue("app.kubernetes.io/name", "test"),
					HaveKeyWithValue("app.kubernetes.io/part-of", "gitlab"),
					HaveKeyWithValue("app.kubernetes.io/managed-by", "gitlab-operator"),
				),
				fmt.Sprintf("Additional labels for component `%s` do not match", c.Name()))
		}
	}
}

func checkEnabledComponents(a *Adapter, components ...gitlab.Component) {
	for _, c := range components {
		Expect(a.WantsComponent(c)).To(BeTrue(),
			fmt.Sprintf("Component `%s` must be enabled", c.Name()))
	}
}

func checkDisabledComponents(a *Adapter, components ...gitlab.Component) {
	for _, c := range components {
		Expect(a.WantsComponent(c)).To(BeFalse(),
			fmt.Sprintf("Component `%s` must be disabled", c.Name()))
	}
}

func checkEnabledFeatures(a *Adapter, features ...gitlab.FeatureCheck) {
	for _, f := range features {
		Expect(a.WantsFeature(f)).To(BeTrue(),
			fmt.Sprintf("Feature `%p` must be enabled", f))
	}
}

func checkDisabledFeatures(a *Adapter, features ...gitlab.FeatureCheck) {
	for _, f := range features {
		Expect(a.WantsFeature(f)).To(BeFalse(),
			fmt.Sprintf("Feature `%p` must be disabled", f))
	}
}
