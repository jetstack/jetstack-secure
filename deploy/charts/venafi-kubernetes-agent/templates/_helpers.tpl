{{/*
Expand the name of the chart.
*/}}
{{- define "venafi-kubernetes-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "venafi-kubernetes-agent.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" $name .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "venafi-kubernetes-agent.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "venafi-kubernetes-agent.labels" -}}
helm.sh/chart: {{ include "venafi-kubernetes-agent.chart" . }}
{{ include "venafi-kubernetes-agent.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "venafi-kubernetes-agent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "venafi-kubernetes-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "venafi-kubernetes-agent.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "venafi-kubernetes-agent.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Util function for generating the image URL based on the provided options.
IMPORTANT: This function is standardized across all charts in the cert-manager GH organization.
Any changes to this function should also be made in cert-manager, trust-manager, approver-policy, ...
See https://github.com/cert-manager/cert-manager/issues/6329 for a list of linked PRs.
*/}}
{{- define "image" -}}
{{- /*
Calling convention:
- (tuple <imageValues> <imageRegistry> <imageNamespace> <defaultReference>)
We intentionally pass imageRegistry/imageNamespace as explicit arguments rather than reading
from `.Values` inside this helper, because `helm-tool lint` does not reliably track `.Values.*`
usage through tuple/variable indirection.
*/ -}}
{{- if ne (len .) 4 -}}
	{{- fail (printf "ERROR: template \"image\" expects (tuple <imageValues> <imageRegistry> <imageNamespace> <defaultReference>), got %d arguments" (len .)) -}}
{{- end -}}
{{- $image := index . 0 -}}
{{- $imageRegistry := index . 1 | default "" -}}
{{- $imageNamespace := index . 2 | default "" -}}
{{- $defaultReference := index . 3 -}}
{{- $repository := "" -}}
{{- if $image.repository -}}
	{{- $repository = $image.repository -}}
	{{- /*
		Backwards compatibility: if image.registry is set, additionally prefix the repository with this registry.
	*/ -}}
	{{- if $image.registry -}}
		{{- $repository = printf "%s/%s" $image.registry $repository -}}
	{{- end -}}
{{- else -}}
	{{- $name := required "ERROR: image.name must be set when image.repository is empty" $image.name -}}
	{{- $repository = $name -}}
	{{- if $imageNamespace -}}
		{{- $repository = printf "%s/%s" $imageNamespace $repository -}}
	{{- end -}}
	{{- if $imageRegistry -}}
		{{- $repository = printf "%s/%s" $imageRegistry $repository -}}
	{{- end -}}
	{{- /*
		Backwards compatibility: if image.registry is set, additionally prefix the repository with this registry.
	*/ -}}
	{{- if $image.registry -}}
		{{- $repository = printf "%s/%s" $image.registry $repository -}}
	{{- end -}}
{{- end -}}
{{- $repository -}}
{{- if and $image.tag $image.digest -}}
	{{- printf ":%s@%s" $image.tag $image.digest -}}
{{- else if $image.tag -}}
	{{- printf ":%s" $image.tag -}}
{{- else if $image.digest -}}
	{{- printf "@%s" $image.digest -}}
{{- else -}}
	{{- printf "%s" $defaultReference -}}
{{- end -}}
{{- end }}
