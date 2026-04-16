{{- define "pulse-services.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "pulse-services.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "pulse-services.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "pulse-services.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "pulse-services.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "pulse-services.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pulse-services.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "pulse-services.drainLifecycle" -}}
lifecycle:
  preStop:
    httpGet:
      path: /drainz
      port: {{ default "metrics" .portName }}
      scheme: HTTP
{{- end -}}

{{- define "pulse-services.runtimeSecretName" -}}
{{- if .Values.runtime.secret.existingSecretName -}}
{{- .Values.runtime.secret.existingSecretName | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-runtime-secret" (include "pulse-services.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "pulse-services.serviceAccountName" -}}
{{- if .Values.runtime.serviceAccount.create -}}
{{- default (printf "%s-runtime" (include "pulse-services.fullname" .)) .Values.runtime.serviceAccount.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- .Values.runtime.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "pulse-services.pdbSpec" -}}
{{- if hasKey . "maxUnavailable" }}
maxUnavailable: {{ .maxUnavailable }}
{{- else }}
minAvailable: {{ .minAvailable }}
{{- end }}
{{- end -}}
