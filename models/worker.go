package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "github.com/piersharding/dask-operator/types"
	"github.com/piersharding/dask-operator/utils"
	appsv1 "k8s.io/api/apps/v1"
)

// DaskWorkerDeployment generates the Deployment description for
// the Dask Worker
func DaskWorkerDeployment(context dtypes.DaskContext) (*appsv1.Deployment, error) {
	// func DaskWorkerDeployment(context dtypes.DaskContext) (interface{}, error) {
	const workerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dask-worker-{{ .Name }}
  labels:
    app.kubernetes.io/name: dask-worker
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: MetaController
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: dask-worker
      app.kubernetes.io/instance: "{{ .Name }}"
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: dask-worker
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
      - name: worker
        image: "{{ .Image }}"
        imagePullPolicy: {{ .PullPolicy }}
{{- if .Resources -}}
{{- with .Resources }}
        resources:
{{ toYaml . | indent 10 }}
{{- end }}
{{- end }}
        command:
          - /start-dask-worker.sh
        env:
          - name: DASK_HOST_NAME
            valueFrom:
              fieldRef:
                fieldPath: status.podIP
          - name: DASK_SCHEDULER
            value: dask-scheduler-{{ .Name }}.{{ .Namespace }}
          - name: DASK_PORT_NANNY
            value: "8789"
          - name: DASK_PORT_WORKER
            value: "8788"
          - name: DASK_PORT_SCHEDULER
            value: "8786"
          - name: DASK_PORT_BOKEH
            value: ":8787"
          - name: DASK_LOCAL_DIRECTORY
            value: "/var/tmp"
          - name: DASK_RESOURCES
            value: ""
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
                containerName: worker
                resource: limits.cpu
          - name: DASK_MEM_LIMIT
            valueFrom:
              resourceFieldRef:
                containerName: worker
                resource: limits.memory
{{- with .Env }}
{{ toYaml . | indent 10 }}
{{- end }}
        ports:
        - name: worker
          containerPort: 8786
        - name: bokeh
          containerPort: 8787
        readinessProbe:
          tcpSocket:
            port: 8787
          initialDelaySeconds: 10
          timeoutSeconds: 10
          periodSeconds: 20
          failureThreshold: 3
        volumeMounts:
        - mountPath: /start-dask-worker.sh
          subPath: start-dask-worker.sh
          name: dask-script
        - mountPath: /var/tmp
          readOnly: false
          name: localdir
{{- with .VolumeMounts }}
{{ toYaml . | indent 8 }}
{{- end }}
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
      # - hostPath:
      #     path: ${WORKER_ARL_DATA}
      #     type: DirectoryOrCreate
      #   name: arldata
      # - name: arldata
      #   persistentVolumeClaim:
      #     claimName: arldata-{{ .Name }}
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
	if context.Daemon {
		log.Infof("Adding Daemon affinity rules")
		if context.Affinity == nil {
			log.Debugf("context.Affinity.podAntiAffinity does not exist")
			context.Affinity = map[string]interface{}{}
		}
		if _, ok := context.Affinity.(map[string]interface{})["podAntiAffinity"]; !ok {
			log.Debugf("context.Affinity.podAntiAffinity does not exist")
			context.Affinity.(map[string]interface{})["podAntiAffinity"] = map[string]interface{}{}
		}
		cAp := context.Affinity.(map[string]interface{})["podAntiAffinity"]

		if _, ok := cAp.(map[string]interface{})["requiredDuringSchedulingIgnoredDuringExecution"]; !ok {
			log.Debugf("context.Affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution does not exist")
			cAp.(map[string]interface{})["requiredDuringSchedulingIgnoredDuringExecution"] = []interface{}{}
		}
		cAp.(map[string]interface{})["requiredDuringSchedulingIgnoredDuringExecution"] =
			append(cAp.(map[string]interface{})["requiredDuringSchedulingIgnoredDuringExecution"].([]interface{}),
				map[string]interface{}{
					"labelSelector": map[string][]map[string]interface{}{
						"matchExpressions": []map[string]interface{}{
							map[string]interface{}{
								"key":      "app.kubernetes.io/instance",
								"operator": "In",
								"values":   []string{context.Name}}}},
					"topologyKey": "kubernetes.io/hostname"})
	}

	result, err := utils.ApplyTemplate(workerDeployment, context)
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