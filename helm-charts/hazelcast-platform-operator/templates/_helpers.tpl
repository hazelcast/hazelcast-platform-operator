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
Return the proper Image Pull Secret Names
*/}}
{{- define "hazelcast-platform-operator.imagePullSecrets" -}}
{{ if .Values.image.pullSecrets }}
{{- range .Values.image.pullSecrets }}
- name: {{ . }}
{{- end }}
{{- else }}
{{- range .Values.imagePullSecrets }}
- name: {{ . }}
{{- end }}
{{- end }}
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
Watched namespaces list concatenated as comma separated string
*/}}
{{- define "watched-namespaces.string" -}}
{{- $str := "" -}}
{{- range .Values.watchedNamespaces -}}
  {{- $str = print $str (trim .) "," }}
{{- end }}
{{- trimSuffix "," $str -}}
{{- end -}}

{{/*
Label selector for watched namespace
*/}}
{{- define "watched-namespaces.labelSelector" -}}
{{- if has (include "watched-namespaces.string" .) (list "" "*" ) -}}
matchLabels: {}
{{- else -}}
matchExpressions:
- key: kubernetes.io/metadata.name
  operator: In
  values:
{{- range $watchedNamespace := .Values.watchedNamespaces }}
  - {{ trim $watchedNamespace }}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Rules needed for giving Hazelcast node read permissions
*/}}
{{- define "hazelcast-platform-operator.hazelcastNodeReadRules" -}}
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - clusterrolebindings
  - clusterroles
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
{{- end -}}

{{/*
Rules needed for operator in its own namespace
- Deployment rule is used for reading the UID
- Lease rule is used for leader election
*/}}
{{- define "hazelcast-platform-operator.operatorNamespaceRules" -}}
- apiGroups:
  - apps
  resources:
  - deployments
  verbs:
  - get
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
{{- end }}

{{/*
Rules needed for operator watched namespaces
*/}}
{{- define "hazelcast-platform-operator.watchedNamespaceRules" -}}
- apiGroups:
  - ""
  resources:
  - configmaps
  - events
  - pods
  - secrets
  - serviceaccounts
  - services
  - endpoints
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - apps
  resources:
  - statefulsets
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - hazelcast.com
  resources:
  - caches
  - cronhotbackups
  - hazelcastendpoints
  - hazelcasts
  - hotbackups
  - jetjobs
  - jetjobsnapshots
  - managementcenters
  - maps
  - multimaps
  - queues
  - replicatedmaps
  - topics
  - usercodenamespaces
  - wansyncs
  - wanreplications
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - hazelcast.com
  resources:
  - caches/finalizers
  - cronhotbackups/finalizers
  - hazelcastendpoints/finalizers
  - hazelcasts/finalizers
  - hotbackups/finalizers
  - jetjobs/finalizers
  - jetjobsnapshots/finalizers
  - managementcenters/finalizers
  - maps/finalizers
  - multimaps/finalizers
  - queues/finalizers
  - replicatedmaps/finalizers
  - topics/finalizers
  - usercodenamespaces/finalizers
  - wansyncs/finalizers
  - wanreplications/finalizers
  verbs:
  - update
- apiGroups:
  - hazelcast.com
  resources:
  - caches/status
  - cronhotbackups/status
  - hazelcastendpoints/status
  - hazelcasts/status
  - hotbackups/status
  - jetjobs/status
  - jetjobsnapshots/status
  - managementcenters/status
  - maps/status
  - multimaps/status
  - queues/status
  - replicatedmaps/status
  - topics/status
  - usercodenamespaces/status
  - wansyncs/status
  - wanreplications/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - rolebindings
  - roles
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - route.openshift.io
  resources:
  - routes
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - route.openshift.io
  resources:
  - routes/custom-host
  verbs:
  - create
- apiGroups:
  - route.openshift.io
  resources:
  - routes/status
  verbs:
  - get
{{- end }}
