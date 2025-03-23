/*
Copyright 2025 Matteo Brambilla <matteo15.brambilla@polimi.it>.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpaEngineSpec defines the desired state of OpaEngine
type OpaEngineSpec struct {
	// Image to use for the OPA engine
	// +kubebuilder:default:value="openpolicyagent/opa:latest-envoy"
	Image string `json:"image,omitempty"`

	// Number of replicas for the OPA engine
	// +kubebuilder:default:value=1
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Resources for the OPA engine
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:required
	// +kubebuilder:validation:MinLength=1
	InstanceName string `json:"instanceName"`

	// The expected lists of policies to be loaded in the OPA engine
	// +kubebuilder:default:={[]}
	Policies []string `json:"policies"`
}

// OpaEngineStatus defines the observed state of OpaEngine
type OpaEngineStatus struct {
	// Represent the observations of a OpaEngine's current state
	// OpaEngine.status.conditions.type are : "Available", "Progressing", "Degraded"
	// OpaEngine.status.conditions.status are : "True", "False", "Unknown"

	// The expected lists of policies loaded in the OPA engine
	// +kubebuilder:default:={}
	Policies []string `json:"policies"`

	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OpaEngine is the Schema for the opaengines API
type OpaEngine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpaEngineSpec   `json:"spec,omitempty"`
	Status OpaEngineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpaEngineList contains a list of OpaEngine
type OpaEngineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpaEngine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpaEngine{}, &OpaEngineList{})
}
