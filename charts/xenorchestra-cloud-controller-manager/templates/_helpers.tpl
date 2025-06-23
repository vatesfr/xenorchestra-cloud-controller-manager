{{/*
Expand the name of the chart.
*/}}
{{- define "xenorchestra-cloud-controller-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "xenorchestra-cloud-controller-manager.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "xenorchestra-cloud-controller-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "xenorchestra-cloud-controller-manager.labels" -}}
helm.sh/chart: {{ include "xenorchestra-cloud-controller-manager.chart" . }}
{{ include "xenorchestra-cloud-controller-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "xenorchestra-cloud-controller-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "xenorchestra-cloud-controller-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "xenorchestra-cloud-controller-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "xenorchestra-cloud-controller-manager.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Generate string of enabled controllers. Might have a trailing comma (,) which needs to be trimmed.
*/}}
{{- define "xenorchestra-cloud-controller-manager.enabledControllers" }}
{{- range .Values.enabledControllers -}}{{ . }},{{- end -}}
{{- end }}
