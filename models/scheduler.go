package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "github.com/piersharding/dask-operator/types"
	"github.com/piersharding/dask-operator/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// DaskSchedulerService generates the Service description for
// the Dask Scheduler
func DaskSchedulerService(dcontext dtypes.DaskContext) (*corev1.Service, error) {

	const schedulerService = `
apiVersion: v1
kind: Service
metadata:
  name: dask-scheduler-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: dask-scheduler
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskController
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
	result, err := utils.ApplyTemplate(schedulerService, dcontext)
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

// DaskSchedulerDeployment generates the Deployment description for
// the Dask Scheduler
func DaskSchedulerDeployment(dcontext dtypes.DaskContext) (*appsv1.Deployment, error) {

	const schedulerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dask-scheduler-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: dask-scheduler
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskController
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
        app.kubernetes.io/managed-by: DaskController
    spec:
      serviceAccountName: "dask-cluster-serviceaccount-{{ .Name }}"
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
            value: "{{ .Port }}"
          - name: DASK_PORT_BOKEH
            value: ":{{ .BokehPort }}"
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
          containerPort: {{ .Port }}
        - name: bokeh
          containerPort: {{ .BokehPort }}
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
            port: {{ .BokehPort }}
          initialDelaySeconds: 10
          timeoutSeconds: 10
          periodSeconds: 20
          failureThreshold: 3
      volumes:
      - configMap:
          name: dask-configs-{{ .Name }}
          defaultMode: 0777
        name: dask-script
      #- hostPath:
      #    path: /var/tmp
      #    type: DirectoryOrCreate
      #  name: localdir
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

	result, err := utils.ApplyTemplate(schedulerDeployment, dcontext)
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

// DaskSchedulerNetworkPolicy generates the NetworkPolicy description for
// the Dask Scheduler
func DaskSchedulerNetworkPolicy(dcontext dtypes.DaskContext) (*networkingv1.NetworkPolicy, error) {
	const schedulerNetworkPolicy = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: dask-scheduler-networkpolicy-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: dask-scheduler-networkpolicy
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskController
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: dask-scheduler
      app.kubernetes.io/instance: "{{ .Name }}"
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
    # enable the scheduler interface for workers
        matchLabels:
          app.kubernetes.io/name:  dask-worker
          app.kubernetes.io/instance: "{{ .Name }}"
    - podSelector:
    # enable the scheduler interface for the notebook
        matchLabels:
          app.kubernetes.io/name:  jupyter-notebook
          app.kubernetes.io/instance: "{{ .Name }}"
    - podSelector:
    # enable the scheduler interface for any DaskJob
        matchLabels:
          app.kubernetes.io/managed-by: DaskJobController
          app.kubernetes.io/name: daskjob-job
    ports:
    - port: scheduler
      protocol: TCP
  - from:
    - namespaceSelector: {}
      podSelector:
    # enable the scheduler monitor interface for everyone
        matchLabels:
          app: nginx-ingress
          component: controller
    ports:
    - port: bokeh
      protocol: TCP
  egress:
  - to:
    - podSelector:
    # enable the scheduler interface for workers
        matchLabels:
          app.kubernetes.io/name:  dask-worker
          app.kubernetes.io/instance: "{{ .Name }}"
`
	result, err := utils.ApplyTemplate(schedulerNetworkPolicy, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}

	networkpolicy := &networkingv1.NetworkPolicy{}
	if err := json.Unmarshal([]byte(result), networkpolicy); err != nil {
		return nil, err
	}
	return networkpolicy, err
}
