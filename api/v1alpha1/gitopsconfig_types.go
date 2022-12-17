/*
Copyright 2022.

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

const (
	// NodeMaintenanceFinalizer is a finalizer for a NodeMaintenance CR deletion
	GitOpsConfigFinalizer string = "foregroundDeleteGitOpsConfig"
)

// See: https://book.kubebuilder.io/reference/markers/crd.html
//      https://sdk.operatorframework.io/docs/building-operators/golang/references/markers/
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=goc
//+kubebuilder:printcolumn:name="Step",type=string,JSONPath=`.status.lastStep`,priority=1
//+kubebuilder:printcolumn:name="Error",type=string,JSONPath=`.status.lastError`,priority=2
//+operator-sdk:csv:customresourcedefinitions:resources={{"GitOpsConfig","v1alpha1","gitopsconfigs"}}
type GitOpsConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitOpsConfigSpec   `json:"spec,omitempty"`
	Status GitOpsConfigStatus `json:"status,omitempty"`
}
type GitOpsConfigSpec struct {
	// Channel to deploy openshift-gitops from. Default: stable
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	OperatorChannel string `json:"operatorChannel,omitempty"`
	// Source to deploy openshift-gitops from. Default: redhat-operators
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	OperatorSource string `json:"operatorSource,omitempty"`

	// Require manual intervention before Argo will sync new content. Default: False
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	ManualSync bool `json:"manualSync,omitempty"`
	// Require manual confirmation before installing and upgrading operators. Default: False
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	ManualApproval bool `json:"manualApproval,omitempty"`

	// Specific version of openshift-gitops to deploy.  Requires UseCSV=True
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	OperatorCSV string `json:"operatorCSV,omitempty"`
	// Dangerous. Force a specific version to be installed. Default: False
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	UseCSV bool `json:"useCSV,omitempty"`
}

type GitOpsConfigStatus struct {
	// Last action related to the pattern
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastStep string `json:"lastStep,omitempty"`

	// Last error encountered by the pattern
	//+operator-sdk:csv:customresourcedefinitions:type=status
	LastError string `json:"lastError,omitempty"`

	// Number of updates to the pattern
	//+operator-sdk:csv:customresourcedefinitions:type=status
	Version int `json:"version,omitempty"`
}

//+kubebuilder:object:root=true

// GitOpsConfigList contains a list of GitOpsConfig
type GitOpsConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitOpsConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitOpsConfig{}, &GitOpsConfigList{})
}
