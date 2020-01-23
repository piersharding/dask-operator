package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "github.com/piersharding/dask-operator/types"
	"github.com/piersharding/dask-operator/utils"
	corev1 "k8s.io/api/core/v1"
)

// ClusterServiceAccount generates the ServiceAccount description for
// for all resources
func ClusterServiceAccount(dcontext dtypes.DaskContext) (*corev1.ServiceAccount, error) {
	const clusterServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dask-cluster-serviceaccount-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: dask-cluster-serviceaccount
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskController
`

	result, err := utils.ApplyTemplate(clusterServiceAccount, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}

	serviceaccount := &corev1.ServiceAccount{}
	if err := json.Unmarshal([]byte(result), serviceaccount); err != nil {
		return nil, err
	}
	return serviceaccount, err
}

// JobServiceAccount generates the ServiceAccount description for
// for the Dask Job resource
func JobServiceAccount(dcontext dtypes.DaskContext) (*corev1.ServiceAccount, error) {
	const jobServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: daskjob-serviceaccount-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: daskjob-serviceaccount
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskJobController
`

	result, err := utils.ApplyTemplate(jobServiceAccount, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}

	serviceaccount := &corev1.ServiceAccount{}
	if err := json.Unmarshal([]byte(result), serviceaccount); err != nil {
		return nil, err
	}
	return serviceaccount, err
}
