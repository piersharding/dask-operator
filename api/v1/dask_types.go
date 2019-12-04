/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DaskSpec defines the desired state of Dask
type DaskSpec struct {

	// +kubebuilder:validation:Default=false

	// Include Jupyter Notebook in deployment: true/false
	// +optional
	Jupyter bool `json:"jupyter,omitempty"`

	// +kubebuilder:validation:Default=false

	// Deploy workers like a DaemonSet - scattered one per node
	// +optional
	Daemon bool `json:"daemon,omitempty"`

	// +kubebuilder:validation:Minimum=0

	// Number of workers to spawn - default will be 5
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:MinLength=0
	// +kubebuilder:validation:Default=daskdev/dask:latest

	// Source image to deploy cluster from - default: daskdev/dask:latest
	Image string `json:"image,omitempty"`

	// Pull Policy for image - default: IfNotPresent
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// Jupyter Ingress hostname - eg: dask.local.net
	// +optional
	JupyterIngress string `json:"jupyterIngress,omitempty"`

	// Jupyter Password hostname - eg: password
	// +optional
	JupyterPassword string `json:"jupyterPassword,omitempty"`

	// Scheduler Ingress hostname - eg: scheduler.local.net
	// +optional
	SchedulerIngress string `json:"schedulerIngress,omitempty"`

	// Scheduler Monitor (bokeh) Ingress hostname - eg: monitor.local.net
	// +optional
	MonitorIngress string `json:"monitorIngress,omitempty"`

	// Specifies the Volumes.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Specifies the VolumeMounts.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Specifies the Environment variables.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies the Pull Secrets.
	// +optional
	PullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Specifies the NodeSelector configuration.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Specifies the Affinity configuration.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Specifies the Toleration configuration.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies the Environment variables.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Specifies the Scheduler specfic variables.
	// +optional
	Scheduler *DaskDeploymentSpec `json:"scheduler,omitempty"`

	// Specifies the Worker specfic variables.
	// +optional
	Worker *DaskDeploymentSpec `json:"worker,omitempty"`

	// Specifies the Jupyter notebook specfic variables.
	// +optional
	Notebook *DaskDeploymentSpec `json:"notebook,omitempty"`
}

type DaskDeploymentSpec struct {
	// Specifies the Volumes.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Specifies the VolumeMounts.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Specifies the Environment variables.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies the Pull Secrets.
	// +optional
	PullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Specifies the NodeSelector configuration.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Specifies the Affinity configuration.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Specifies the Toleration configuration.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies the Environment variables.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// DaskStatus defines the observed state of Dask
type DaskStatus struct {
	Replicas  int32  `json:"replicas"`
	Succeeded int32  `json:"succeeded"`
	State     string `json:"state"`
	Resources string `json:"resources"`
}

// Dask is the Schema for the dasks API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Components",type="integer",JSONPath=".status.replicas",description="The number of Components Requested in the Dask",priority=0
// +kubebuilder:printcolumn:name="Succeeded",type="integer",JSONPath=".status.succeeded",description="The number of Components Launched in the Dask",priority=0
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The number of Components Requested in the Dask",priority=0
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="Status of the Dask",priority=0
// +kubebuilder:printcolumn:name="Resources",type="string",JSONPath=".status.resources",description=" Resource details of the Dask",priority=1
type Dask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DaskSpec   `json:"spec,omitempty"`
	Status DaskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DaskList contains a list of Dask
type DaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dask{}, &DaskList{})
}
