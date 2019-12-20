package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "github.com/piersharding/dask-operator/types"
	"github.com/piersharding/dask-operator/utils"
	batchv1 "k8s.io/api/batch/v1"
)

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
  backoffLimit: 3
  completions: 1
  parallelism: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: daskjob-job
        app.kubernetes.io/instance: "{{ .Name }}"
        app.kubernetes.io/managed-by: DaskJobController
    spec:
      restartPolicy: Never
    {{- with .PullSecrets }}
      imagePullSecrets:
      {{range $val := .}}
      - name: {{ $val.name }}
      {{end}}
      {{- end }}
      containers:
      - name: scheduler
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
