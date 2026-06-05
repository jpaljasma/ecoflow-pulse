{{- define "pulse-platform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "pulse-platform.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "pulse-platform.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "pulse-platform.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "pulse-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "pulse-platform.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pulse-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "pulse-platform.imageRef" -}}
{{- if .digest -}}
{{- printf "%s@%s" .repository .digest -}}
{{- else -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}
{{- end -}}

{{- define "pulse-platform.secretManagerServiceAccountName" -}}
{{- if .Values.secretManager.serviceAccount.create -}}
{{- default (printf "%s-gcp-secrets" (include "pulse-platform.fullname" .)) .Values.secretManager.serviceAccount.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- .Values.secretManager.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "pulse-platform.pdbSpec" -}}
{{- if hasKey . "maxUnavailable" }}
maxUnavailable: {{ .maxUnavailable }}
{{- else }}
minAvailable: {{ .minAvailable }}
{{- end }}
{{- end -}}
