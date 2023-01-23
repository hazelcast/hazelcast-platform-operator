{{/*
Expand the name of the chart.
*/}}
{{- define "hazelcast-platform-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "hazelcast-platform-operator.fullname" -}}
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
{{- define "hazelcast-platform-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "hazelcast-platform-operator.labels" -}}
helm.sh/chart: {{ include "hazelcast-platform-operator.chart" . }}
{{ include "hazelcast-platform-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "hazelcast-platform-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hazelcast-platform-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "hazelcast-platform-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "hazelcast-platform-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the image name of the deployment
*/}}
{{- define "hazelcast-platform-operator.imageName" -}}
{{- if .Values.image.imageOverride }}
{{- .Values.image.imageOverride  }}
{{- else }}
{{- .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}
{{- end }}
{{- end }}

{{/*
Watched namespace name
*/}}
{{- define "watched-namespace" -}}
{{- if .Values.watchedNamespace }} 
{{- .Values.watchedNamespace }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}


{{/*
Label selector for watched namespace
*/}}
{{- define "watched-namespace.labelSelector" -}}
matchLabels:
  kubernetes.io/metadata.name: {{ include "watched-namespace" . }}
{{- end }}
