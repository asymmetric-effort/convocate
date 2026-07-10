{{/*
Common labels for all resources
*/}}
{{- define "convocate.labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: convocate
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/*
Standard security context for pods (non-root, seccomp)
*/}}
{{- define "convocate.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 65534
seccompProfile:
  type: RuntimeDefault
{{- end }}

{{/*
Standard security context for containers
*/}}
{{- define "convocate.containerSecurityContext" -}}
allowPrivilegeEscalation: false
capabilities:
  drop: ["ALL"]
{{- end }}

{{/*
Standard security context for containers with read-only root filesystem
*/}}
{{- define "convocate.containerSecurityContextReadOnly" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop: ["ALL"]
{{- end }}

{{/*
Namespace helpers
*/}}
{{- define "convocate.namespace.app" -}}
{{ .Values.namespaces.app | default "convocate" }}
{{- end }}

{{- define "convocate.namespace.agents" -}}
{{ .Values.namespaces.agents | default "convocate-agents" }}
{{- end }}

{{- define "convocate.namespace.data" -}}
{{ .Values.namespaces.data | default "data-layer" }}
{{- end }}

{{- define "convocate.namespace.security" -}}
{{ .Values.namespaces.security | default "security" }}
{{- end }}

{{- define "convocate.namespace.o11y" -}}
{{ .Values.namespaces.o11y | default "o11y" }}
{{- end }}

{{/*
Image helper: builds full image reference
Usage: {{ include "convocate.image" (dict "global" .Values.global "name" "api" "tag" .Values.imageTag) }}
*/}}
{{- define "convocate.image" -}}
{{ .global.registry }}/convocate/{{ .name }}:{{ .tag | default .global.imageTag | default "latest" }}
{{- end }}
