{{/*
Expand the name of the chart.
*/}}
{{- define "elara.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
Truncated at 63 chars to satisfy the DNS naming spec.
If the release name contains the chart name, it is used as-is.
*/}}
{{- define "elara.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Headless service name for StatefulSet stable DNS.
*/}}
{{- define "elara.headlessServiceName" -}}
{{- printf "%s-headless" (include "elara.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Chart label (name-version).
*/}}
{{- define "elara.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels (applied to every resource).
*/}}
{{- define "elara.labels" -}}
helm.sh/chart: {{ include "elara.chart" . }}
{{ include "elara.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: elara
{{- end }}

{{/*
Selector labels (stable across upgrades — do not change).
*/}}
{{- define "elara.selectorLabels" -}}
app.kubernetes.io/name: {{ include "elara.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name to use.
*/}}
{{- define "elara.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "elara.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Name of the ConfigMap that holds the service env vars.
*/}}
{{- define "elara.configMapName" -}}
{{- printf "%s-config" (include "elara.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Name of the Secret (reserved for future use: OTLP auth tokens, TLS certs).
*/}}
{{- define "elara.secretName" -}}
{{- printf "%s-secrets" (include "elara.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Container image reference. Prefers digest when set, otherwise tag, otherwise AppVersion.
*/}}
{{- define "elara.image" -}}
{{- $repo := .Values.image.repository -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" $repo .Values.image.digest -}}
{{- else -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}
{{- end }}

{{/*
The resolved service name used in Prometheus/OTel resource labels.
Falls back to the chart name when config.serviceName is empty.
*/}}
{{- define "elara.serviceName" -}}
{{- default (include "elara.name" .) .Values.config.serviceName }}
{{- end }}
