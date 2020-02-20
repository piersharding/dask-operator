package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "gitlab.com/piersharding/dask-operator/types"
	"gitlab.com/piersharding/dask-operator/utils"
	networkingv1 "k8s.io/api/networking/v1"
)

// DNSNetworkPolicy generates the NetworkPolicy description for
// the DNS for all resources
func DNSNetworkPolicy(dcontext dtypes.DaskContext) (*networkingv1.NetworkPolicy, error) {
	const dnsNetworkPolicy = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: dask-networkpolicy-dns-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: dask-networkpolicy-dns
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskController
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/managed-by: DaskController
      app.kubernetes.io/instance: "{{ .Name }}"
  policyTypes:
  - Egress
  egress:
  # enable the entire Dask cluster to talk to DNS
  - ports:
    - port: 53
      protocol: UDP
    - port: 53
      protocol: TCP
`

	result, err := utils.ApplyTemplate(dnsNetworkPolicy, dcontext)
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
