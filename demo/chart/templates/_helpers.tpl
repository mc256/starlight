{{/*
Expand the name of the chart.
*/}}
{{- define "starlight.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 48 | trimSuffix "-" }}
{{- end }}



{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "starlight.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 48 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 48 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 48 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}



{{- define "starlight.fullname-registry" -}}
{{ include "starlight.fullname" . }}-registry
{{- end }}


{{- define "starlight.fullname-edge" -}}
{{ include "starlight.fullname" . }}-edge
{{- end }}


{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "starlight.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}


{{/*
Common labels - Starlight Proxy
*/}}
{{- define "starlight.proxyLabels" -}}
helm.sh/chart: {{ include "starlight.chart" . }}
{{ include "starlight.proxySelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}


{{/*
Common labels - Registry
*/}}
{{- define "starlight.registryLabels" -}}
helm.sh/chart: {{ include "starlight.chart" . }}
{{ include "starlight.registrySelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}



{{/*
Common labels - edge
*/}}
{{- define "starlight.edgeLabels" -}}
helm.sh/chart: {{ include "starlight.chart" . }}
{{ include "starlight.edgeSelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}




{{/*
Selector labels - Starlight Proxy
*/}}
{{- define "starlight.proxySelectorLabels" -}}
app.kubernetes.io/name: {{ include "starlight.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: proxy
{{- end }}


{{/*
Selector labels - Registry
*/}}
{{- define "starlight.registrySelectorLabels" -}}
app.kubernetes.io/name: {{ include "starlight.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: registry
{{- end }}


{{/*
Selector labels - Edge
*/}}
{{- define "starlight.edgeSelectorLabels" -}}
app.kubernetes.io/name: {{ include "starlight.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: edge
{{- end }}


{{/*
Create the name of the service account to use
*/}}
{{- define "starlight.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "starlight.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
