package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "github.com/piersharding/dask-operator/types"
	"github.com/piersharding/dask-operator/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// JupyterService generates the Service description for
// the Jupyter Notebook server
func JupyterService(dcontext dtypes.DaskContext) (*corev1.Service, error) {

	const jupyterService = `
apiVersion: v1
kind: Service
metadata:
  name: jupyter-notebook-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: jupyter-notebook
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: MetaController
spec:
  selector:
    app.kubernetes.io/name:  jupyter-notebook
    app.kubernetes.io/instance: "{{ .Name }}"
  type: {{ .ServiceType }}
  ports:
  - name: jupyter
    port: 8888
    targetPort: jupyter
    protocol: TCP
`
	result, err := utils.ApplyTemplate(jupyterService, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}
	service := &corev1.Service{}
	if err := json.Unmarshal([]byte(result), service); err != nil {
		return nil, err
	}
	return service, err
}

// JupyterDeployment generates the Deployment description for
// the Jupyter Notebook
func JupyterDeployment(dcontext dtypes.DaskContext) (*appsv1.Deployment, error) {

	const jupyterDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jupyter-notebook-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: jupyter-notebook
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: MetaController
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: jupyter-notebook
      app.kubernetes.io/instance: "{{ .Name }}"
  replicas: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: jupyter-notebook
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
      - name: jupyter
        image: "{{ .JupyterImage }}"
        imagePullPolicy: {{ .PullPolicy }}
        command:
          - /start-jupyter-notebook.sh
        env:
          - name: DASK_SCHEDULER
            value: dask-scheduler-{{ .Name }}.{{ .Namespace }}:8786
          - name: JUPYTER_PASSWORD
            value: "{{ .JupyterPassword }}"
          - name: NOTEBOOK_PORT
            value: "8888"
{{- with .Env }}
{{ toYaml . | indent 10 }}
{{- end }}
        ports:
        - name: jupyter
          containerPort: 8888
        volumeMounts:
        - mountPath: /start-jupyter-notebook.sh
          subPath: start-jupyter-notebook.sh
          name: dask-script
        - mountPath: /jupyter_notebook_config.py
          subPath: jupyter_notebook_config.py
          name: dask-script
        - mountPath: /var/tmp
          readOnly: false
          name: localdir
{{- with .VolumeMounts }}
{{ toYaml . | indent 8 }}
{{- end }}
        readinessProbe:
          httpGet:
            path: /api
            port: 8888
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

	result, err := utils.ApplyTemplate(jupyterDeployment, dcontext)
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
