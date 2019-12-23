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

// DaskJobSpec defines the desired state of DaskJob
type DaskJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Dask scheduler resource name that is the cluster the job will run against: mandatory
	Cluster string `json:"cluster"`

	// Dask script for this job - FQ file name, HTTP URL, or full script body either .py or .ipynb: mandatory
	Script string `json:"script"`

	// Source image to deploy cluster from - default: daskdev/dask:latest
	Image string `json:"image,omitempty"`

	// Pull Policy for image - default: IfNotPresent
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// Save the report output in /reports to a persistent volume: true/false
	// +optional
	Report bool `json:"report,omitempty"`

	//Report StorageClass - default: standard
	// +optional
	ReportStorageClass string `json:"reportStorageClass,omitempty"`

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

// DaskJobStatus defines the observed state of DaskJob
type DaskJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Succeeded int32  `json:"succeeded"`
	State     string `json:"state"`
	Resources string `json:"resources"`
}

// DaskJob is the Schema for the daskjobs API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Succeeded",type="integer",JSONPath=".status.succeeded",description="The number of Components Launched in the Dask",priority=0
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The number of Components Requested in the Dask",priority=0
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="Status of the Dask",priority=0
// +kubebuilder:printcolumn:name="Resources",type="string",JSONPath=".status.resources",description=" Resource details of the Dask",priority=1
type DaskJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DaskJobSpec   `json:"spec,omitempty"`
	Status DaskJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DaskJobList contains a list of DaskJob
type DaskJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DaskJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DaskJob{}, &DaskJobList{})
}
