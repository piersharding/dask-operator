package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "github.com/piersharding/dask-operator/types"
	"github.com/piersharding/dask-operator/utils"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

// DaskSchedulerService generates the Service description for
// the Dask Scheduler
func DaskSchedulerService(context dtypes.DaskContext) (*v1.Service, error) {

	const schedulerService = `
apiVersion: v1
kind: Service
metadata:
  name: dask-scheduler-{{ .Name }}
  labels:
    app.kubernetes.io/name: dask-scheduler
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: MetaController
spec:
  selector:
    app.kubernetes.io/name:  dask-scheduler
    app.kubernetes.io/instance: "{{ .Name }}"
  type: {{ .ServiceType }}
  ports:
  - name: scheduler
    port: {{ .Port }}
    targetPort: scheduler
    protocol: TCP
  - name: bokeh
    port: {{ .BokehPort }}
    targetPort: bokeh
    protocol: TCP
`
	result, err := utils.ApplyTemplate(schedulerService, context)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}
	service := &v1.Service{}
	if err := json.Unmarshal([]byte(result), service); err != nil {
		return nil, err
	}
	return service, err
}

// DaskSchedulerDeployment generates the Deployment description for
// the Dask Scheduler
func DaskSchedulerDeployment(context dtypes.DaskContext) (*appsv1.Deployment, error) {

	const schedulerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dask-scheduler-{{ .Name }}
  labels:
    app.kubernetes.io/name: dask-scheduler
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: MetaController
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: dask-scheduler
      app.kubernetes.io/instance: "{{ .Name }}"
  replicas: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: dask-scheduler
        app.kubernetes.io/instance: "{{ .Name }}"
        app.kubernetes.io/managed-by: MetaController
    spec:
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
          - /start-dask-scheduler.sh
        env:
          - name: DASK_HOST_NAME
            valueFrom:
              fieldRef:
                fieldPath: status.podIP
          - name: DASK_SCHEDULER
            value: dask-scheduler-{{ .Name }}.{{ .Namespace }}
          - name: DASK_PORT_SCHEDULER
            value: "8786"
          - name: DASK_PORT_BOKEH
            value: ":8787"
          - name: DASK_BOKEH_WHITELIST
            value: dask-scheduler-{{ .Name }}.{{ .Namespace }}
          - name: DASK_BOKEH_APP_PREFIX
            value: "/"
          - name: DASK_LOCAL_DIRECTORY
            value: "/var/tmp"
          - name: K8S_APP_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: DASK_UID
            valueFrom:
              fieldRef:
                fieldPath: metadata.uid
          - name: DASK_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: DASK_CPU_LIMIT
            valueFrom:
              resourceFieldRef:
                containerName: scheduler
                resource: limits.cpu
          - name: DASK_MEM_LIMIT
            valueFrom:
              resourceFieldRef:
                containerName: scheduler
                resource: limits.memory
                {{- with .Env }}
{{ toYaml . | indent 10 }}
{{- end }}
        ports:
        - name: scheduler
          containerPort: 8786
        - name: bokeh
          containerPort: 8787
        volumeMounts:
        - mountPath: /start-dask-scheduler.sh
          subPath: start-dask-scheduler.sh
          name: dask-script
        - mountPath: /var/tmp
          readOnly: false
          name: localdir
{{- with .VolumeMounts }}
{{ toYaml . | indent 8 }}
{{- end }}
        readinessProbe:
          httpGet:
            path: /json/identity.json
            port: 8787
          initialDelaySeconds: 10
          timeoutSeconds: 10
          periodSeconds: 20
          failureThreshold: 3
      volumes:
      - configMap:
          name: dask-configs-{{ .Name }}
          defaultMode: 0777
        name: dask-script
      - hostPath:
          path: /var/tmp
          type: DirectoryOrCreate
        name: localdir

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

	result, err := utils.ApplyTemplate(schedulerDeployment, context)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}

	deployment := &appsv1.Deployment{}
	if err := json.Unmarshal([]byte(result), deployment); err != nil {
		return nil, err
	}
	return deployment, err
}
