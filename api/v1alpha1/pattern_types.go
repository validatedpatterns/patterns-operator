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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// NodeMaintenanceFinalizer is a finalizer for a NodeMaintenance CR deletion
	PatternFinalizer string = "foregroundDeletePattern"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
//  https://pkg.go.dev/encoding/json#Marshal

type PatternParameter struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make generate" to regenerate code after modifying this file
	// Foo is an example field of Pattern. Edit pattern_types.go to remove/update

	//+operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name"`

	//+operator-sdk:csv:customresourcedefinitions:type=spec
	Value string `json:"value"`
}

// Future fields...
//   SendAnonymousUsage   bool   `json:"anonymousUsage,omitempty"`
//   Validation       bool   `json:"validation,omitempty"`
//   ValidationImage  string `json:"validationImage,omitempty"`
//   RequiredSecrets []string `json:"requiredSecrets,omitempty"`
// It would be great to use this, instead of ExtraParameters, but controller-gen barfs on it
//   Values      map[string]interface{} `json:"values,omitempty" yaml:"valuesLocal,omitempty"`

// PatternSpec defines the desired state of Pattern
type PatternSpec struct {
	// SPEC FIELDS - desired state of cluster
	// Important: Run "make generate" to regenerate code after modifying this file

	GitConfig GitConfig `json:"gitSpec"`

	//+operator-sdk:csv:customresourcedefinitions:type=spec
	ClusterGroupName string `json:"clusterGroupName"`

	// .Name is dot separated per the helm --set syntax, such as:
	//   global.something.field
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	ExtraParameters []PatternParameter `json:"extraParameters,omitempty"`

	// URLs to additional Helm parameter files
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	ExtraValueFiles []string `json:"extraValueFiles,omitempty"`

	// Look for external changes every N minutes
	// ReconcileMinutes int    `json:"reconcileMinutes,omitempty"`
}

type GitConfig struct {
	// Optional. FQDN of the git server if automatic parsing from TargetRepo is broken
	Hostname string `json:"hostname,omitempty"`

	//Account              string `json:"account,omitempty"`
	//TokenSecret          string `json:"tokenSecret,omitempty"`
	//TokenSecretNamespace string `json:"tokenSecretNamespace,omitempty"`
	//TokenSecretKey       string `json:"tokenSecretKey,omitempty"`

	// Upstream git repo containing the pattern to deploy. Used when in-cluster fork to point to the upstream pattern repository
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	OriginRepo string `json:"originRepo,omitempty"`

	// Branch, tag or commit in the upstream git repository. Does not support short-sha's. Default to HEAD
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	OriginRevision string `json:"originRevision,omitempty"`

	// Interval in seconds to poll for drifts between origin and target repositories. Default: 180 seconds
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	PollInterval int `json:"pollInterval,omitempty"`

	// Git repo containing the pattern to deploy. Must use https/http
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	TargetRepo string `json:"targetRepo"`

	// Branch, tag, or commit to deploy.  Does not support short-sha's. Default: HEAD
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	TargetRevision string `json:"targetRevision,omitempty"`
}

type ApplyChangeType string

const (
	InstallAutomatic ApplyChangeType = "Automatic"
	InstallManual    ApplyChangeType = "Manual"
)

// PatternStatus defines the observed state of Pattern
type PatternStatus struct {
	// Observed state of the pattern

	// Last action related to the pattern
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastStep string `json:"lastStep,omitempty"`

	// Last error encountered by the pattern
	//+operator-sdk:csv:customresourcedefinitions:type=status
	LastError string `json:"lastError,omitempty"`

	// Number of updates to the pattern
	//+operator-sdk:csv:customresourcedefinitions:type=status
	Version int `json:"version,omitempty"`

	//+operator-sdk:csv:customresourcedefinitions:type=status
	ClusterName string `json:"clusterName,omitempty"`
	//+operator-sdk:csv:customresourcedefinitions:type=status
	AppClusterDomain string `json:"appClusterDomain,omitempty"`
	//+operator-sdk:csv:customresourcedefinitions:type=status
	ClusterDomain string `json:"clusterDomain,omitempty"`
	//+operator-sdk:csv:customresourcedefinitions:type=status
	ClusterID string `json:"clusterID,omitempty"`
	//+operator-sdk:csv:customresourcedefinitions:type=status
	ClusterPlatform string `json:"clusterPlatform,omitempty"`
	//+operator-sdk:csv:customresourcedefinitions:type=status
	ClusterVersion string `json:"clusterVersion,omitempty"`
	//+operator-sdk:csv:customerresourcedefinitions:type=conditions
	Conditions []PatternCondition `json:"conditions,omitempty"`
}

// See: https://book.kubebuilder.io/reference/markers/crd.html
//      https://sdk.operatorframework.io/docs/building-operators/golang/references/markers/
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=patt
//+kubebuilder:printcolumn:name="Step",type=string,JSONPath=`.status.lastStep`,priority=1
//+kubebuilder:printcolumn:name="Error",type=string,JSONPath=`.status.lastError`,priority=2
//+operator-sdk:csv:customresourcedefinitions:resources={{"Pattern","v1alpha1","patterns"}}

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

type PatternCondition struct {
	// Type of deployment condition.
	Type PatternConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

type PatternConditionType string

const (
	GitOutOfSync PatternConditionType = "GitOutOfSync"
	GitInSync    PatternConditionType = "GitInSync"
)

func init() {
	SchemeBuilder.Register(&Pattern{}, &PatternList{})
}
