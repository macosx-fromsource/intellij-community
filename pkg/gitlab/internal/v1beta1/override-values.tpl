certmanager:
  install: false

gitlab-runner:
  install: false

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

global:
  common:
    labels:
      app.kubernetes.io/name: {{ .ReleaseName }}
      app.kubernetes.io/part-of: gitlab
      app.kubernetes.io/managed-by: gitlab-operator

  image:
    pullPolicy: IfNotPresent

  ingress:
    apiVersion: networking.k8s.io/v1
    {{ if .UseCertManager }}
    annotations:
      cert-manager.io/issuer: {{ .ReleaseName }}-issuer
      acme.cert-manager.io/http01-edit-in-place: true
    {{ end }}

  serviceAccount:
    enabled: true
    create: false
    name: {{ .Settings.AppNonRootServiceAccount }}

minio:
  common:
    labels:
      app.kubernetes.io/component: minio
      app.kubernetes.io/instance: {{ .ReleaseName }}-minio

postgresql:
  commonLabels:
    gitlab.io/component: postgresql
  serviceAccount:
    enabled: true
    create: false
    name: {{ .Settings.AppNonRootServiceAccount }}
  securityContext:
    fsGroup: 1000
    runAsUser: 1000

redis:
  commonLabels:
    gitlab.io/component: redis
  serviceAccount:
    name: {{ .Settings.AppNonRootServiceAccount }}
  securityContext:
    fsGroup: 1000
    runAsUser: 1000

registry:
  common:
    labels:
      app.kubernetes.io/component: registry
      app.kubernetes.io/instance: {{ .ReleaseName }}-registry

shared-secrets:
  serviceAccount:
    create: false
    name: {{ .Settings.ManagerServiceAccount }}
  securityContext:
    runAsUser: ''
    fsGroup: ''

prometheus:
  rbac:
    create: false
  serviceAccounts:
    server:
      create: false
      name: {{ .Settings.PrometheusServiceAccount }}
    alertmanager:
      create: false
      name: {{ .Settings.AppNonRootServiceAccount }}
    nodeExporter:
      create: false
      name: {{ .Settings.AppNonRootServiceAccount }}
    pushgateway:
      create: false
      name: {{ .Settings.AppNonRootServiceAccount }}

gitlab-zoekt:
  serviceAccount:
    create: false
    name: {{ .Settings.AppNonRootServiceAccount }}
