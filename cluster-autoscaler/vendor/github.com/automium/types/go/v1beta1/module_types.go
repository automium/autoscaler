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

// ModuleSpec defines the desired state of Module
type ModuleSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	Source   string          `json:"source"`
	Image    string          `json:"image"`
	Replicas int             `json:"replicas"`
	Flavor   string          `json:"flavor"`
	Action   string          `json:"action"`
	Env      []corev1.EnvVar `json:"env,omitempty"`
}

// ModuleStatus defines the observed state of Module
type ModuleStatus struct {
	Phase           string `json:"phase"`
	Replicas        int    `json:"replicas"`
	CurrentReplicas int    `json:"currentReplicas"`
	UpdatedReplicas int    `json:"updatedReplicas"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Module is the Schema for the modules API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Service",type="string",JSONPath=".metadata.ownerReferences[0].name",description="the parent service.core.automium.io"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="the execution phase"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.updatedReplicas",description="Ready node replicas"
// +kubebuilder:printcolumn:name="Requested",type="string",JSONPath=".status.replicas",description="Requested node replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type Module struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleSpec   `json:"spec,omitempty"`
	Status ModuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleList contains a list of Module
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Module `json:"items"`
}

