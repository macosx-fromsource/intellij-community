package v1beta1

import (
	"strings"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var (
	BackupCronJob                = newCheckEnabled("gitlab.toolbox.backups.cron.enabled")
	BackupCronJobPersistence     = newCheckEnabled("gitlab.toolbox.backups.cron.persistence.enabled")
	ConfigureCertManager         = newCheckEnabledWithDefault(true, "global.ingress.configureCertmanager")
	ConfigureGatewayCertManager  = newCheckEnabled("global.gatewayApi.configureCertmanager")
	ReplaceGitalyWithPraefect    = newCheckEnabled("global.praefect.enabled", "global.praefect.replaceInternalGitaly")
	RestoreDeploymentPersistence = newCheckEnabled("gitlab.toolbox.persistence.enabled")
)

/* GitLabFeatures */

func (w *Adapter) WantsFeature(check gitlab.FeatureCheck) bool {
	return check(w.Values())
}

func (w *Adapter) WantsComponent(component gitlab.Component) bool {
	if check, ok := mapComponentEnabled[component]; ok {
		return w.WantsFeature(check)
	}

	return false
}

/* Helpers */

func newCheckEnabledWithDefault(defaultValue bool, keys ...string) gitlab.FeatureCheck {
	return func(values support.Values) bool {
		for _, k := range keys {
			if !values.GetBool(k, defaultValue) {
				return false
			}
		}

		return true
	}
}

func newCheckEnabled(keys ...string) gitlab.FeatureCheck {
	return newCheckEnabledWithDefault(false, keys...)
}

var mapComponentEnabled = map[gitlab.Component]gitlab.FeatureCheck{
	component.GeoLogcursor: func(values support.Values) bool {
		if !newCheckEnabled("global.geo.enabled")(values) {
			return false
		}

		geoRole := values.GetString("global.geo.role")
		if strings.ToLower(geoRole) != "secondary" {
			return false
		}

		return newCheckEnabledWithDefault(true, "gitlab.geo-logcursor.enabled")(values)
	},
	component.Gitaly:         newCheckEnabled("global.gitaly.enabled"),
	component.GitLabExporter: newCheckEnabled("gitlab.gitlab-exporter.enabled"),
	component.GitLabPages:    newCheckEnabled("global.pages.enabled"),
	component.GitLabShell:    newCheckEnabled("gitlab.gitlab-shell.enabled"),
	component.GitLabKAS:      newCheckEnabled("global.kas.enabled"),
	component.Mailroom:       newCheckEnabled("gitlab.mailroom.enabled", "global.appConfig.incomingEmail.enabled"),
	component.Migrations:     newCheckEnabled("gitlab.migrations.enabled"),
	component.MinIO:          newCheckEnabled("global.minio.enabled"),
	component.NginxIngress:   newCheckEnabled("nginx-ingress.enabled"),
	component.NginxGeo:       newCheckEnabled("nginx-ingress-geo.enabled"),
	component.PostgreSQL:     newCheckEnabled("postgresql.install"),
	component.Praefect:       newCheckEnabled("global.praefect.enabled"),
	component.Prometheus:     newCheckEnabled("prometheus.install"),
	component.Redis:          newCheckEnabled("redis.install"),
	component.Registry:       newCheckEnabled("registry.enabled"),
	component.Sidekiq:        newCheckEnabled("gitlab.sidekiq.enabled"),
	component.Spamcheck:      newCheckEnabled("global.spamcheck.enabled"),
	component.Toolbox:        newCheckEnabled("gitlab.toolbox.enabled"),
	component.Webservice:     newCheckEnabled("gitlab.webservice.enabled"),
	component.Zoekt:          newCheckEnabled("gitlab-zoekt.install"),
}
