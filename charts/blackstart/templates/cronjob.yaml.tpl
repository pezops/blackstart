{{- if .Values.cronJob.enabled }}
apiVersion: batch/v1
kind: CronJob
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
spec:
  schedule: "{{ .Values.cronJob.schedule }}"
  concurrencyPolicy: {{ .Values.cronJob.concurrencyPolicy }}
  startingDeadlineSeconds: {{ .Values.cronJob.startingDeadlineSeconds }}
  successfulJobsHistoryLimit: {{ .Values.cronJob.successfulJobsHistoryLimit }}
  failedJobsHistoryLimit: {{ .Values.cronJob.failedJobsHistoryLimit }}
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: {{ .Values.serviceAccount.name }}
          containers:
            - name: blackstart
              image: "{{ if .Values.image.registry }}{{ .Values.image.registry }}/{{ end }}{{ .Values.image.repository }}:{{ default .Chart.AppVersion .Values.image.tag }}"
              imagePullPolicy: {{ .Values.image.pullPolicy }}
              env:
                - name: BLACKSTART_RUNTIME_MODE
                  value: "once"
              {{- if not .Values.watchAllNamespaces }}
                - name: BLACKSTART_K8S_NAMESPACE
                  value: {{ .Release.Namespace | quote }}
              {{- end }}
          restartPolicy: OnFailure
{{- end }}
