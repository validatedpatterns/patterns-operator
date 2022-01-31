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
	"k8s.io/apimachinery/pkg/types"
)

const (
	// NodeMaintenanceFinalizer is a finalizer for a NodeMaintenance CR deletion
	PatternFinalizer string = "foregroundDeletePattern"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
//  https://pkg.go.dev/encoding/json#Marshal

type PatternParameter struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Pattern. Edit pattern_types.go to remove/update
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PatternSpec defines the desired state of Pattern
type PatternSpec struct {
	// SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SiteName string `json:"siteName"`

	GitConfig    GitConfig    `json:"gitSpec"`
	GitOpsConfig GitOpsConfig `json:"gitOpsSpec,omitempty"`

	AnonymousUsage  bool   `json:"anonymousUsage,omitempty"`
	Validation      bool   `json:"validation,omitempty"`
	ValidationImage string `json:"validationImage,omitempty"`

	Parameters      []PatternParameter `json:"parameters,omitempty"`
	RequiredSecrets []string           `json:"requiredSecrets,omitempty"`
}

type GitConfig struct {
	Hostname       string               `json:"hostname,omitempty"`
	Account        string               `json:"account"`
	TokenSecret    types.NamespacedName `json:"tokenSecret,omitempty"`
	TokenSecretKey string               `json:"tokenSecretKey,omitempty"`

	OriginRepo     string `json:"originRepo,omitempty"`
	TargetRepo     string `json:"targetRepo"`
	TargetRevision string `json:"targetRevision,omitempty"`

	ValuesDirectoryURL string `json:"valuesDirectoryURL,omitempty"`
}

type ApplyChangeType string

const (
	InstallAutomatic ApplyChangeType = "Automatic"
	InstallManual    ApplyChangeType = "Manual"
)

type GitOpsConfig struct {
	OperatorChannel string `json:"operatorChannel,omitempty"`
	OperatorSource  string `json:"operatorSource,omitempty"`
	OperatorCSV     string `json:"operatorCSV,omitempty"`

	SyncPolicy          ApplyChangeType `json:"syncPolicy,omitempty"`
	InstallPlanApproval ApplyChangeType `json:"installPlanApproval,omitempty"`
	UseCSV              bool            `json:"useCSV,omitempty"`
}

//global:
//  imageregistry:
//   type: quay

// PatternStatus defines the observed state of Pattern
type PatternStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LastError string `json:"lastError,omitempty"`
	Path      string `json:"path,omitempty"`
	Version   int    `json:"version,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Pattern is the Schema for the patterns API
type Pattern struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PatternSpec   `json:"spec,omitempty"`
	Status PatternStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PatternList contains a list of Pattern
type PatternList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pattern `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Pattern{}, &PatternList{})
}
