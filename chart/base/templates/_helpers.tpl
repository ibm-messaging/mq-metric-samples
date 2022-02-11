{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "starter-kit.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "starter-kit.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "starter-kit.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "starter-kit.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "starter-kit.host" -}}
{{- $chartName := include "starter-kit.name" . -}}
{{- $host := default $chartName .Values.ingress.host -}}
{{- $subdomain := default .Values.ingress.subdomain .Values.global.ingressSubdomain -}}
{{- if .Values.ingress.namespaceInHost -}}
{{- printf "%s-%s.%s" $host .Release.Namespace $subdomain -}}
{{- else -}}
{{- printf "%s.%s" $host $subdomain -}}
{{- end -}}
{{- end -}}

{{- define "starter-kit.url" -}}
{{- $secretName := include "starter-kit.tlsSecretName" . -}}
{{- $host := include "starter-kit.host" . -}}
{{- if $secretName -}}
{{- printf "https://%s" $host -}}
{{- else -}}
{{- printf "http://%s" $host -}}
{{- end -}}
{{- end -}}

{{- define "starter-kit.protocols" -}}
{{- $secretName := include "starter-kit.tlsSecretName" . -}}
{{- if $secretName -}}
{{- printf "%s,%s" "http" "https" -}}
{{- else -}}
{{- printf "%s" "http" -}}
{{- end -}}
{{- end -}}

{{- define "starter-kit.tlsSecretName" -}}
{{- $secretName := default .Values.ingress.tlsSecretName .Values.global.tlsSecretName -}}
{{- if $secretName }}
{{- printf "%s" $secretName -}}
{{- else -}}
{{- printf "" -}}
{{- end -}}
{{- end -}}

{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "mq-spring-app.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "mq-spring-app.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "mq-spring-app.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "mq-spring-app.labels" -}}
helm.sh/chart: {{ include "mq-spring-app.chart" . }}
{{ include "mq-spring-app.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "mq-spring-app.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mq-spring-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "mq-spring-app.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "mq-spring-app.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}
