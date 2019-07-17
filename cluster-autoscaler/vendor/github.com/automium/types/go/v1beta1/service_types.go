/*
Copyright 2018 The Automium Authors.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ServiceSpec defines the desired state of Service
type ServiceSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Minimum=0
	Replicas int             `json:"replicas"`
	Flavor   string          `json:"flavor"`
	Version  string          `json:"version"`
	Tags     []string        `json:"tags,omitempty"`
	Env      []corev1.EnvVar `json:"env,omitempty"`
}

// ServiceStatus defines the observed state of Service
type ServiceStatus struct {
	Phase     string `json:"phase"`
	ModuleRef string `json:"moduleRef"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Service is the Schema for the services API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Application",type="string",JSONPath=".metadata.labels['app']",description="The desired application"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="The application version"
// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".spec.replicas",description="The application replicas"
// +kubebuilder:printcolumn:name="Flavor",type="string",JSONPath=".spec.flavor",description="The instance flavor"
// +kubebuilder:printcolumn:name="Module",type="string",JSONPath=".status.moduleRef",description="module.core.automium.io reference"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="The execution phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Service struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceSpec   `json:"spec,omitempty"`
	Status ServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceList contains a list of Service
type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Service `json:"items"`
}
