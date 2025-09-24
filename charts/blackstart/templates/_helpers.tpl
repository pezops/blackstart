{{/*
Create the full image name and tag
*/}}
{{- define "blackstart.image" -}}
{{- if .Values.image.registry -}}
{{- .Values.image.registry }}/
{{- end -}}
{{- .Values.image.repository -}}
{{- if .Values.image.tag -}}
:{{ .Values.image.tag }}
{{- end -}}
{{- end -}}

