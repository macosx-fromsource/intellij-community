global:
  image:
    pullPolicy: IfNotPresent

  ingress:
    apiVersion: networking.k8s.io/v1
  
  gatewayApi:
    class:
      controllerName: "gateway.envoyproxy.io/gatewayclass-controller"

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
  master:
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
  controller:
    # Port securty context related values from chart 8.5.0+
    # for SCC compatability with older charts.
    # Can be removed once the minimum chart version is 8.5.0.
    image:
      allowPrivilegeEscalation: false

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
    # Port securty context related values from chart 8.5.0+
    # for SCC compatability with older charts.
    # Can be removed once the minimum chart version is 8.5.0.
    image:
      allowPrivilegeEscalation: false
