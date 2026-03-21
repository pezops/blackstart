{{- if .Values.serviceAccount.create }}
{{- $annotations := dict }}
{{- with .Values.serviceAccount.annotations }}
{{- $annotations = merge $annotations . }}
{{- end }}
{{- $gwi := .Values.serviceAccount.gcpWorkloadIdentity }}
{{- if $gwi.enabled }}
{{- $gsaUsername := required "serviceAccount.gcpWorkloadIdentity.username is required when gcpWorkloadIdentity.enabled=true" $gwi.username }}
{{- $gsaProjectID := required "serviceAccount.gcpWorkloadIdentity.projectID is required when gcpWorkloadIdentity.enabled=true" $gwi.projectID }}
{{- $_ := set $annotations "iam.gke.io/gcp-service-account" (printf "%s@%s.iam.gserviceaccount.com" $gsaUsername $gsaProjectID) }}
{{- end }}
{{- $irsa := .Values.serviceAccount.awsIRSA }}
{{- if $irsa.enabled }}
{{- $irsaRoleARN := required "serviceAccount.awsIRSA.roleARN is required when awsIRSA.enabled=true" $irsa.roleARN }}
{{- $_ := set $annotations "eks.amazonaws.com/role-arn" $irsaRoleARN }}
{{- if $irsa.stsRegionalEndpoints }}
{{- $_ := set $annotations "eks.amazonaws.com/sts-regional-endpoints" "true" }}
{{- end }}
{{- end }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.serviceAccount.name }}
  namespace: {{ .Release.Namespace }}
  {{- if gt (len $annotations) 0 }}
  annotations:
    {{- toYaml $annotations | nindent 4 }}
  {{- end }}
{{- end }}
