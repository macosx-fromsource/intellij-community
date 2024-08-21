certmanager-issuer:
  email: {{ .Settings.CertmanagerIssuerEmail }}

gitlab:
  webservice:
    serviceAccount:
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

