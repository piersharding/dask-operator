package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "gitlab.com/piersharding/dask-operator/types"
	"gitlab.com/piersharding/dask-operator/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// DaskJob generates the Job description for
// the Dask Job
func DaskJobReportStorage(dcontext dtypes.DaskContext) (*corev1.PersistentVolumeClaim, error) {

	const daskJobReportStorage = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: daskjob-report-pvc-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: daskjob-job-report-pvc
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskJobController
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: {{ .ReportStorageClass }}
  resources:
    requests:
      storage: 1Gi
`

	result, err := utils.ApplyTemplate(daskJobReportStorage, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := json.Unmarshal([]byte(result), pvc); err != nil {
		return nil, err
	}
	return pvc, err
}

// DaskJob generates the Job description for
// the Dask Job
func DaskJob(dcontext dtypes.DaskContext) (*batchv1.Job, error) {

	const daskJob = `
apiVersion: batch/v1
kind: Job
metadata:
  name: daskjob-job-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: daskjob-job
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskJobController
spec:
  backoffLimit: 2
  completions: 1
  parallelism: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: daskjob-job
        app.kubernetes.io/instance: "{{ .Name }}"
        app.kubernetes.io/managed-by: DaskJobController
    spec:
      serviceAccountName: "daskjob-serviceaccount-{{ .Name }}"
      restartPolicy: Never
    {{- with .PullSecrets }}
      imagePullSecrets:
      {{range $val := .}}
      - name: {{ $val.name }}
      {{end}}
      {{- end }}
      containers:
      - name: scheduler
        securityContext:
          runAsUser: 0
        image: "{{ .Image }}"
        imagePullPolicy: {{ .PullPolicy }}
        command:
          - /start-dask-job.sh
        env:
          - name: DASK_HOST_NAME
            valueFrom:
              fieldRef:
                fieldPath: status.podIP
          - name: DASK_SCHEDULER
            value: "dask-scheduler-{{ .Cluster }}.{{ .Namespace }}:{{ .Port }}"
          - name: DASK_PORT_SCHEDULER
            value: "{{ .Port }}"
          - name: DASK_LOCAL_DIRECTORY
            value: "/var/tmp"
          - name: K8S_APP_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
                {{- with .Env }}
{{ toYaml . | indent 10 }}
{{- end }}
        volumeMounts:
{{- if .Report }}
        - name: reports
          mountPath: /reports
{{- end }}
        - mountPath: /start-dask-job.sh
          subPath: start-dask-job.sh
          name: dask-script
        - mountPath: /app.{{ .ScriptType }}
          subPath: app.{{ .ScriptType }}
          name: dask-script
        - mountPath: /var/tmp
          readOnly: false
          name: localdir
{{- with .VolumeMounts }}
{{ toYaml . | indent 8 }}
{{- end }}
      volumes:
{{- if .Report }}
      - name: reports
        persistentVolumeClaim:
          claimName: daskjob-report-pvc-{{ .Name }}
{{- end }}
      - configMap:
          name: daskjob-configs-{{ .Name }}
          defaultMode: 0777
        name: dask-script
      - name: localdir
        emptyDir: {}
{{- with .Volumes }}
{{ toYaml . | indent 6 }}
{{- end }}
{{- with .NodeSelector }}
      nodeSelector:
{{ toYaml . | indent 8 }}
{{- end }}
{{- with .Affinity }}
      affinity:
{{ toYaml . | indent 8 }}
{{- end }}
{{- with .Tolerations }}
      tolerations:
{{ toYaml . | indent 8 }}
{{- end }}

`

	result, err := utils.ApplyTemplate(daskJob, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}

	job := &batchv1.Job{}
	if err := json.Unmarshal([]byte(result), job); err != nil {
		return nil, err
	}
	return job, err
}
