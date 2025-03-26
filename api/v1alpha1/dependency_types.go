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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DependencySpec defines the desired state of Dependency
type DependencySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	ServiceName string `json:"serviceName"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	PolicyName string `json:"policyName"`
}

// DependencyStatus defines the observed state of Dependency
type DependencyStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// If the dependency is deployed in EngineName
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Deployed bool `json:"deployed,omitempty"`

	// The engine name where the dependency is scheduled to be deployed
	// It can be set to a non null value even if the dependency is not deployed
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	EngineName []string `json:"engineName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Dependency is the Schema for the dependencies API
type Dependency struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DependencySpec   `json:"spec,omitempty"`
	Status DependencyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DependencyList contains a list of Dependency
type DependencyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dependency `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dependency{}, &DependencyList{})
}
