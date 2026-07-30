package component

import (
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

const (
	GitLab gitlab.Component = "gitlab"

	GeoLogcursor   gitlab.Component = "geo-logcursor"
	Gitaly         gitlab.Component = "gitaly"
	GitLabExporter gitlab.Component = "gitlab-exporter"
	GitLabPages    gitlab.Component = "gitlab-pages"
	GitLabShell    gitlab.Component = "gitlab-shell"
	GitLabKAS      gitlab.Component = "kas"
	Mailroom       gitlab.Component = "mailroom"
	Migrations     gitlab.Component = "migrations"
	NginxIngress   gitlab.Component = "nginx-ingress"
	NginxGeo       gitlab.Component = "nginx-ingress-geo"
	Praefect       gitlab.Component = "praefect"
	Prometheus     gitlab.Component = "prometheus"
	Registry       gitlab.Component = "registry"
	SharedSecrets  gitlab.Component = "shared-secrets"
	Sidekiq        gitlab.Component = "sidekiq"
	Toolbox        gitlab.Component = "toolbox"
	Webservice     gitlab.Component = "webservice"
	Zoekt          gitlab.Component = "zoekt"
)

var (
	Core = gitlab.Components{
		Gitaly,
	}

	Stateful = gitlab.Components{
		Gitaly,
	}

	All = gitlab.Components{
		Gitaly,
		GitLabExporter,
		GitLabPages,
		GitLabShell,
		GitLabKAS,
		Mailroom,
		Migrations,
		NginxIngress,
		Registry,
		SharedSecrets,
		Sidekiq,
		Webservice,
	}
)
