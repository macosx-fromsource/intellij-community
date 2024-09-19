{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create the name of the manager service account to use
*/}}
{{- define "manager.serviceAccount.name" -}}
{{- if .Values.manager.serviceAccount.create }}
{{- $name := printf "%s-%s" (include "name" .) "manager" }}
{{- default $name .Values.manager.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.manager.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the app service account to use
*/}}
{{- define "app.serviceAccount.name" -}}
{{- if .Values.app.serviceAccount.create }}
{{- $name := printf "%s-%s" (include "name" .) "app" }}
{{- default $name .Values.app.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.app.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the nginx-ingress service account to use
*/}}
{{- define "nginx-ingress.serviceAccount.name" -}}
{{- if (index .Values "nginx-ingress").serviceAccount.create }}
{{- $name := printf "%s-%s" (include "name" .) "nginx-ingress" }}
{{- default $name (index .Values "nginx-ingress").serviceAccount.name }}
{{- else }}
{{- default "default" (index .Values "nginx-ingress").serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the prometheus server service account to use
*/}}
{{- define "prometheus.serviceAccount.server.name" -}}
{{- if .Values.prometheus.serviceAccount.server.create }}
{{-   $name := printf "%s-%s" (include "name" .) "prometheus-server" }}
{{-   default $name .Values.prometheus.serviceAccount.server.name }}
{{- else }}
{{-   default "default" .Values.prometheus.serviceAccount.server.name }}
{{- end }}
{{- end }}

{{/*
SecurityContextConstraints apiVersion
*/}}
{{- define "scc.apiVersion" -}}
{{- if .Values.scc.apiVersion }}
{{- .Values.scc.apiVersion }}
{{- else if .Capabilities.APIVersions.Has "security.openshift.io/v1" -}}
security.openshift.io/v1
{{- end }}
{{- end }}

{{/*
Returns true if OpenShift cluster is detected.
*/}}
{{- define "isOpenShift" -}}
{{- $sccApiVersion := include "scc.apiVersion" . -}}
{{- not (empty $sccApiVersion) }}
{{- end }}

{{/*
Returns a image tag from the passed in app version or branchname
Usage:
{{ include "gitlab.parseAppVersion" (    \
     dict                                \
         "appVersion" .Chart.AppVersion  \
         "prepend" "false"               \
     ) }}
1. If the version is a semver version, we check the prepend flag.
   1. If it is true, we prepend a `v` and return `vx.y.z` image tag.
   2. If it is false, we do not prepend a `v` and just use the input version
2. Else we just use the version passed as the image tag
*/}}
{{- define "gitlab.parseAppVersion" -}}
{{- $appVersion := coalesce .appVersion "latest" -}}
{{- if regexMatch "^\\d+\\.\\d+\\.\\d+(-rc\\d+)?(-pre)?$" $appVersion -}}
{{-   if eq .prepend "true" -}}
{{-      printf "v%s" $appVersion -}}
{{-   else -}}
{{-      $appVersion -}}
{{-   end -}}
{{- else -}}
{{- $appVersion -}}
{{- end -}}
{{- end -}}

{{/*
Return the version tag used to fetch the GitLab images
Defaults to using the information from the chart appVersion field
*/}}
{{- define "gitlab.image.tag" -}}
{{-   $prepend := coalesce .image.prepend "false" -}}
{{-   $appVersion := include "gitlab.parseAppVersion" (dict "appVersion" .context.Chart.AppVersion "prepend" $prepend) -}}
{{-   coalesce .image.tag $appVersion }}
{{- end -}}

{{/*
Return the image digest to use.
*/}}
{{- define "gitlab.image.digest" -}}
{{-   if .image.digest -}}
{{-     printf "@%s" .image.digest -}}
{{-   end -}}
{{- end -}}

{{/*
Creates the full image path for use in manifests.
*/}}
{{- define "gitlab.image.fullPath" -}}
{{-   $registry := .image.registry -}}
{{-   $repository := .image.repository -}}
{{-   $name := .image.name -}}
{{-   $tag := include "gitlab.image.tag" . -}}
{{-   $digest := include "gitlab.image.digest" . -}}
{{-   printf "%s/%s/%s:%s%s" $registry $repository $name $tag $digest | quote -}}
{{- end -}}

{{/*
Create the name of the app service account to use
*/}}
{{- define "webhook.service.name" -}}
{{- printf "%s-%s" (include "name" .) "webhook-service" }}
{{- end }}
