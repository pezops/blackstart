{{- if .Values.controller.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .Release.Name }}
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .Release.Name }}
        app.kubernetes.io/instance: {{ .Release.Name }}
    spec:
      serviceAccountName: {{ .Values.serviceAccount.name }}
      containers:
        - name: blackstart
          image: "{{ if .Values.image.registry }}{{ .Values.image.registry }}/{{ end }}{{ .Values.image.repository }}:{{ default .Chart.AppVersion .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          env:
            - name: BLACKSTART_RUNTIME_MODE
              value: "controller"
            - name: BLACKSTART_MAX_PARALLEL_RECONCILIATIONS
              value: {{ .Values.controller.maxParallelReconciliations | quote }}
            - name: BLACKSTART_CONTROLLER_RESYNC_INTERVAL
              value: {{ .Values.controller.resyncInterval | quote }}
            - name: BLACKSTART_QUEUE_WAIT_WARNING_THRESHOLD
              value: {{ .Values.controller.queueWaitWarningThreshold | quote }}
            {{- if not .Values.watchAllNamespaces }}
            - name: BLACKSTART_K8S_NAMESPACE
              value: {{ .Release.Namespace | quote }}
            {{- end }}
{{- end }}
