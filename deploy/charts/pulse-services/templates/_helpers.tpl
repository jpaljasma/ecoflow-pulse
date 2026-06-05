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

{{- define "pulse-services.imageRef" -}}
{{- if .digest -}}
{{- printf "%s@%s" .repository .digest -}}
{{- else -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}
{{- end -}}

{{- define "pulse-services.imagePullSecrets" -}}
{{- with .Values.runtime.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}

{{- define "pulse-services.drainLifecycle" -}}
lifecycle:
  preStop:
    httpGet:
      path: /drainz
      port: {{ default "metrics" .portName }}
      scheme: HTTP
{{- end -}}

{{- define "pulse-services.goRuntimeEnvItems" -}}
{{- $goRuntime := default dict .goRuntime -}}
{{- with get $goRuntime "maxProcs" }}
- name: GOMAXPROCS
  value: {{ . | quote }}
{{- end }}
{{- with get $goRuntime "memLimit" }}
- name: GOMEMLIMIT
  value: {{ . | quote }}
{{- end }}
{{- end -}}

{{- define "pulse-services.goRuntimeEnv" -}}
{{- with (include "pulse-services.goRuntimeEnvItems" . | trim) }}
env:
{{ . | nindent 2 }}
{{- end }}
{{- end -}}

{{- define "pulse-services.runtimeSecretName" -}}
{{- if .Values.runtime.secret.existingSecretName -}}
{{- .Values.runtime.secret.existingSecretName | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-runtime-secret" (include "pulse-services.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "pulse-services.gcsCredentialsVolumeItems" -}}
{{- if .Values.runtime.gcsCredentials.enabled }}
- name: gcs-credentials
  secret:
    secretName: {{ required "runtime.gcsCredentials.secretName is required when runtime.gcsCredentials.enabled=true" .Values.runtime.gcsCredentials.secretName | quote }}
    items:
      - key: {{ .Values.runtime.gcsCredentials.secretKey | quote }}
        path: {{ .Values.runtime.gcsCredentials.fileName | quote }}
{{- end }}
{{- end -}}

{{- define "pulse-services.gcsCredentialsVolumeMountItems" -}}
{{- if .Values.runtime.gcsCredentials.enabled }}
- name: gcs-credentials
  mountPath: {{ .Values.runtime.gcsCredentials.mountPath | quote }}
  readOnly: true
{{- end }}
{{- end -}}

{{- define "pulse-services.gcsCredentialsVolumes" -}}
{{- with (include "pulse-services.gcsCredentialsVolumeItems" . | trim) }}
volumes:
{{ . | nindent 2 }}
{{- end }}
{{- end -}}

{{- define "pulse-services.gcsCredentialsVolumeMounts" -}}
{{- with (include "pulse-services.gcsCredentialsVolumeMountItems" . | trim) }}
volumeMounts:
{{ . | nindent 2 }}
{{- end }}
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

{{- define "pulse-services.podScheduling" -}}
{{- $default := default dict .default -}}
{{- $workload := default dict .workload -}}
{{- $nodeSelector := default (get $default "nodeSelector") (get $workload "nodeSelector") -}}
{{- $tolerations := default (get $default "tolerations") (get $workload "tolerations") -}}
{{- $affinity := default (get $default "affinity") (get $workload "affinity") -}}
{{- $topologySpreadConstraints := default (get $default "topologySpreadConstraints") (get $workload "topologySpreadConstraints") -}}
{{- with $nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $affinity }}
affinity:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $topologySpreadConstraints }}
topologySpreadConstraints:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}
