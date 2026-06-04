{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "venafi-connection.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "venafi-connection.labels" -}}
helm.sh/chart: {{ include "venafi-connection.chart" . }}
{{ include "venafi-connection.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "venafi-connection.selectorLabels" -}}
app.kubernetes.io/name: "venafi-connection"
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
