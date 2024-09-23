certmanager:
  install: false

gitlab-runner:
  install: false

global:
  common:
    labels:
      app.kubernetes.io/name: {{ .ReleaseName }}
      app.kubernetes.io/part-of: gitlab
      app.kubernetes.io/managed-by: gitlab-operator

gitlab:
  gitaly:
    common:
      labels:
        app.kubernetes.io/component: gitaly
        app.kubernetes.io/instance: {{ .ReleaseName }}-gitaly

  gitlab-exporter:
    common:
      labels:
        app.kubernetes.io/component: gitlab-exporter
        app.kubernetes.io/instance: {{ .ReleaseName }}-gitlab-exporter

  gitlab-pages:
    common:
      labels:
        app.kubernetes.io/component: gitlab-pages
        app.kubernetes.io/instance: {{ .ReleaseName }}-gitlab-pages

  gitlab-shell:
    common:
      labels:
        app.kubernetes.io/component: gitlab-shell
        app.kubernetes.io/instance: {{ .ReleaseName }}-gitlab-shell

  kas:
    common:
      labels:
        app.kubernetes.io/component: kas
        app.kubernetes.io/instance: {{ .ReleaseName }}-kas

  mailroom:
    common:
      labels:
        app.kubernetes.io/component: mailroom
        app.kubernetes.io/instance: {{ .ReleaseName }}-mailroom

  migrations:
    common:
      labels:
        app.kubernetes.io/component: migrations
        app.kubernetes.io/instance: {{ .ReleaseName }}-migrations

  sidekiq:
    common:
      labels:
        app.kubernetes.io/component: sidekiq
        app.kubernetes.io/instance: {{ .ReleaseName }}-sidekiq

  spamcheck:
    common:
      labels:
        app.kubernetes.io/component: spamcheck
        app.kubernetes.io/instance: {{ .ReleaseName }}-spamcheck

  toolbox:
    common:
      labels:
        app.kubernetes.io/component: toolbox
        app.kubernetes.io/instance: {{ .ReleaseName }}-toolbox

  webservice:
    common:
      labels:
        app.kubernetes.io/component: webservice
        app.kubernetes.io/instance: {{ .ReleaseName }}-webservice

minio:
  common:
    labels:
      app.kubernetes.io/component: minio
      app.kubernetes.io/instance: {{ .ReleaseName }}-minio

postgresql:
  commonLabels:
    gitlab.io/component: postgresql

redis:
  commonLabels:
    gitlab.io/component: redis

registry:
  common:
    labels:
      app.kubernetes.io/component: registry
      app.kubernetes.io/instance: {{ .ReleaseName }}-registry

