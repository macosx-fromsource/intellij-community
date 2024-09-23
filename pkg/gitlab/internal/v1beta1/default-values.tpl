global:
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

certmanager-issuer:
  email: {{ .Settings.CertmanagerIssuerEmail }}

postgresql:
  serviceAccount:
    enabled: true
    create: false
    name: {{ .Settings.AppNonRootServiceAccount }}
  securityContext:
    fsGroup: 1000
    runAsUser: 1000

redis:
  serviceAccount:
    name: {{ .Settings.AppNonRootServiceAccount }}
  securityContext:
    fsGroup: 1000
    runAsUser: 1000

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

nginx-ingress:
  labels:
    app.kubernetes.io/name: {{ .ReleaseName }}
    app.kubernetes.io/part-of: gitlab
    app.kubernetes.io/managed-by: gitlab-operator
    app.kubernetes.io/component: nginx-ingress
    app.kubernetes.io/instance: {{ .ReleaseName }}-nginx-ingress
  rbac:
    create: false
  serviceAccount:
    create: false
    name: {{ .Settings.NginxServiceAccount }}
  defaultBackend:
    serviceAccount:
      name: {{ .Settings.AppNonRootServiceAccount }}

nginx-ingress-geo:
  labels:
    app.kubernetes.io/name: {{ .ReleaseName }}
    app.kubernetes.io/part-of: gitlab
    app.kubernetes.io/managed-by: gitlab-operator
    app.kubernetes.io/component: nginx-ingress-geo
    app.kubernetes.io/instance: {{ .ReleaseName }}-nginx-ingress-geo
  rbac:
    create: false
  serviceAccount:
    create: false
    name: {{ .Settings.NginxServiceAccount }}
  defaultBackend:
    serviceAccount:
      name: {{ .Settings.AppNonRootServiceAccount }}
  controller:
    ingressClassResource:
      controllerValue: k8s.io/ingress-nginx-geo

