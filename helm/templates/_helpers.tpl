{{/* Common labels applied to every object. */}}
{{- define "recurring-task-creator.labels" -}}
app.kubernetes.io/name: recurring-task-creator
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: agent
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- with .Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
{{- end -}}

{{/* Image ref: registry/repository:tag, tag defaulting to appVersion. */}}
{{- define "recurring-task-creator.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.image.repository $tag -}}
{{- end -}}
