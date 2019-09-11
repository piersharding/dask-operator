package types

import (
	"github.com/appscode/go/log"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DaskContext is the set of parameters to configures this instance
type DaskContext struct {
	JupyterIngress   string
	SchedulerIngress string
	Daemon           bool
	Jupyter          bool
	Namespace        string
	Name             string
	ServiceType      string
	Port             int
	BokehPort        int
	Replicas         int
	Image            string
	Repository       string
	Tag              string
	PullSecrets      interface{}
	PullPolicy       string
	NodeSelector     interface{}
	Affinity         interface{}
	Tolerations      interface{}
	Resources        interface{}
	VolumeMounts     interface{}
	Volumes          interface{}
	Env              interface{}
	JupyterImage     string
	Password         string
}

// Controller struct root object from payload
type Controller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ControllerSpec   `json:"spec"`
	Status            ControllerStatus `json:"status"`
}

// ControllerSpec struct defines Spec expected from Kind: Dask
type ControllerSpec struct {
	JupyterIngress   string      `json:"jupyterIngress"`
	SchedulerIngress string      `json:"schedulerIngress"`
	Replicas         int         `json:"replicas"`
	Daemon           bool        `json:"daemon"`
	Jupyter          bool        `json:"jupyter"`
	Password         string      `json:"password"`
	Image            string      `json:"image"`
	PullPolicy       string      `json:"imagePullPolicy"`
	PullSecrets      interface{} `json:"imagePullSecrets"`
	NodeSelector     interface{} `json:"nodeSelector"`
	Affinity         interface{} `json:"affinity"`
	Tolerations      interface{} `json:"tolerations"`
	Resources        interface{} `json:"resources"`
	VolumeMounts     interface{} `json:"volumeMounts"`
	Volumes          interface{} `json:"volumes"`
	Env              interface{} `json:"env"`
}

// ControllerStatus struct response status
type ControllerStatus struct {
	Replicas  int    `json:"replicas"`
	Succeeded int    `json:"succeeded"`
	State     string `json:"state"`
	Resources string `json:"resources"`
}

// SyncRequest struct root request object
type SyncRequest struct {
	Parent   Controller          `json:"parent"`
	Children SyncRequestChildren `json:"children"`
}

// SyncRequestChildren struct children objects returned
type SyncRequestChildren struct {
	// Pods         map[string]*v1.Pod             `json:"Pod.v1"`
	// StatefulSets map[string]*appsv1.StatefulSet `json:"StatefulSet.apps/v1"`
	Deployments map[string]*appsv1.Deployment `json:"Deployment.apps/v1"`
}

// SyncResponse struct root response object
type SyncResponse struct {
	Status   ControllerStatus `json:"status"`
	Children []runtime.Object `json:"children"`
}

// SetConfig setup the configuration
func SetConfig(request *SyncRequest) DaskContext {

	context := DaskContext{
		JupyterIngress:   request.Parent.Spec.JupyterIngress,
		SchedulerIngress: request.Parent.Spec.SchedulerIngress,
		Daemon:           request.Parent.Spec.Daemon,
		Jupyter:          request.Parent.Spec.Jupyter,
		Namespace:        request.Parent.Namespace,
		Name:             request.Parent.Name,
		ServiceType:      "ClusterIP",
		Port:             8786,
		BokehPort:        8787,
		Replicas:         request.Parent.Spec.Replicas,
		Image:            request.Parent.Spec.Image,
		PullSecrets:      request.Parent.Spec.PullSecrets,
		PullPolicy:       request.Parent.Spec.PullPolicy,
		NodeSelector:     request.Parent.Spec.NodeSelector,
		Affinity:         request.Parent.Spec.Affinity,
		Tolerations:      request.Parent.Spec.Tolerations,
		Resources:        request.Parent.Spec.Resources,
		VolumeMounts:     request.Parent.Spec.VolumeMounts,
		Volumes:          request.Parent.Spec.Volumes,
		Env:              request.Parent.Spec.Env,
		JupyterImage:     "jupyter/scipy-notebook:latest",
		Password:         request.Parent.Spec.Password}

	if context.Password == "" {
		context.Password = "password"
	}
	log.Debugf("context: %+v", context)
	return context
}
