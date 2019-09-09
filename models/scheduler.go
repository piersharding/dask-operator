package models

import (
	"bytes"
	"fmt"
	"text/template"

	ejson "encoding/json"

	"github.com/piersharding/dask-operator/utils"
)

// DaskSchedulerService generates the Service description for
// the Dask Scheduler
func DaskSchedulerService(context DaskContext) (string, error) {

	const scheduler_service = `
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

	t := template.Must(template.New("scheduler_service").Parse(scheduler_service))
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, context); err != nil {
		return "", err
	}
	result := tpl.String()

	resp := utils.YamlToJSON(result)
	prettyJSON, _ := ejson.MarshalIndent(resp, "", "    ")
	// fmt.Printf("Output: %s\n", prettyJSON)
	return string(prettyJSON), nil
}

// DaskSchedulerDSeployment generates the Deployment description for
// the Dask Scheduler
func DaskSchedulerDeployment(context DaskContext) (string, error) {

	const scheduler_deployment = `
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
      imagePullSecrets:
      - name: {{ .PullSecret }}
      containers:
      - name: scheduler
        image: "{{ .Repository }}:{{ .Tag }}"
        imagePullPolicy: {{ .PullPolicy }}
        command:
          - /usr/local/bin/start-dask-scheduler.sh
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
            value: "8787"
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
          - name: DASK_SCHEDULER
            value: dask-scheduler-{{ .Name }}.{{ .Namespace }}
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
      volumeMounts:
      - mountPath: /var/tmp
        name: localdir
        readOnly: false
      ports:
      - name: scheduler
        containerPort: 8786
      - name: bokeh
        containerPort: 8787
      readinessProbe:
        httpGet:
          path: /json/identity.json
          port: 8787
        initialDelaySeconds: 60
        timeoutSeconds: 10
        periodSeconds: 20
        failureThreshold: 3
      volumes:
      - hostPath:
          path: /var/tmp
          type: DirectoryOrCreate
        name: localdir
  {{- with .NodeSelector }}
    nodeSelector:
{{ . }}
  {{- end }}
  {{- with .Affinity }}
    affinity:
{{ . }}
  {{- end }}
  {{- with .Tolerations }}
    tolerations:
{{ . }}
  {{- end }}
`

	t := template.Must(template.New("scheduler_deployment").Parse(scheduler_deployment))
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, context); err != nil {
		fmt.Printf("Error: %+v\n", err)
		return "", err
	}

	result := tpl.String()

	resp := utils.YamlToJSON(result)
	prettyJSON, _ := ejson.MarshalIndent(resp, "", "    ")
	fmt.Printf("Output: %s\n", prettyJSON)
	return string(prettyJSON), nil
}
