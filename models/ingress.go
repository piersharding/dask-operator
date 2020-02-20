package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "gitlab.com/piersharding/dask-operator/types"
	"gitlab.com/piersharding/dask-operator/utils"
	v1beta1 "k8s.io/api/extensions/v1beta1"
)

// DaskIngress generates the Ingress description for
// the Dask cluster
func DaskIngress(dcontext dtypes.DaskContext) (*v1beta1.Ingress, error) {

	const daskIngress = `
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: dask-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: dask
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskController
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/x-forwarded-prefix: "true"
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
{{- if and .Jupyter .JupyterIngress}}
  - host: {{ .JupyterIngress }}
    http:
      paths:
      - path: /
        backend:
          serviceName:  jupyter-notebook-{{ .Name }}
          servicePort: 8888
{{- end }}
{{- if .SchedulerIngress}}
  - host: {{ .SchedulerIngress }}
    http:
      paths:
      - path: /
        backend:
          serviceName:  dask-scheduler-{{ .Name }}
          servicePort: {{.Port }}
  - host: {{ .MonitorIngress }}
    http:
      paths:
      - path: /
        backend:
          serviceName:  dask-scheduler-{{ .Name }}
          servicePort: {{ .BokehPort }}
{{- end }}
`
	result, err := utils.ApplyTemplate(daskIngress, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}
	ingress := &v1beta1.Ingress{}
	if err := json.Unmarshal([]byte(result), ingress); err != nil {
		return nil, err
	}
	return ingress, err
}
