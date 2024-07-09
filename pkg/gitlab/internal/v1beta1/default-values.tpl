certmanager-issuer:
  email: {{ .Settings.CertmanagerIssuerEmail }}

gitlab:
  webservice:
    serviceAccount:
      name: {{ .Settings.AppNonRootServiceAccount }}
