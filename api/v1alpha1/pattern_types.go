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
	argoapi "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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

	GitConfig    GitConfig     `json:"gitSpec"`
	GitOpsConfig *GitOpsConfig `json:"gitOpsSpec,omitempty"`

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
	//Account              string `json:"account,omitempty"`
	//TokenSecret          string `json:"tokenSecret,omitempty"`
	//TokenSecretNamespace string `json:"tokenSecretNamespace,omitempty"`
	//TokenSecretKey       string `json:"tokenSecretKey,omitempty"`

	// Git repo containing the pattern to deploy. Must use https/http
	//+operator-sdk:csv:customresourcedefinitions:type=spec,order=1
	TargetRepo string `json:"targetRepo"`

	// Branch, tag, or commit to deploy.  Does not support short-sha's. Default: HEAD
	//+operator-sdk:csv:customresourcedefinitions:type=spec,order=2
	TargetRevision string `json:"targetRevision,omitempty"`

	// Upstream git repo containing the pattern to deploy. Used when in-cluster fork to point to the upstream pattern repository
	//+operator-sdk:csv:customresourcedefinitions:type=spec,order=3
	OriginRepo string `json:"originRepo,omitempty"`

	// Branch, tag or commit in the upstream git repository. Does not support short-sha's. Default to HEAD
	//+operator-sdk:csv:customresourcedefinitions:type=spec,order=4
	OriginRevision string `json:"originRevision,omitempty"`

	// Interval in seconds to poll for drifts between origin and target repositories. Default: 180 seconds
	//+operator-sdk:csv:customresourcedefinitions:type=spec,order=5
	PollInterval int `json:"pollInterval,omitempty"`

	// Optional. FQDN of the git server if automatic parsing from TargetRepo is broken
	//+operator-sdk:csv:customresourcedefinitions:type=spec,order=6
	Hostname string `json:"hostname,omitempty"`
}

type ApplyChangeType string

const (
	InstallAutomatic ApplyChangeType = "Automatic"
	InstallManual    ApplyChangeType = "Manual"
)

type GitOpsConfig struct {
	// Channel to deploy openshift-gitops from. Default: gitops-1.8
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

// PatternApplicationInfo defines the Applications
// Status for the Pattern.
// This structure is part of the PatternStatus as an array
// The Application Status will be included as part of the Observed state of Pattern
type PatternApplicationInfo struct {
	Name             string                         `json:"name,omitempty"`
	AppSyncStatus    string                         `json:"syncStatus,omitempty"`
	AppHealthStatus  string                         `json:"healthStatus,omitempty"`
	AppHealthMessage string                         `json:"healthMessage,omitempty"`
	AppReconcileTime string                         `json:"lastReconcileTime,omitempty"`
	Conditions       []argoapi.ApplicationCondition `json:"conditions,omitempty"`
	Resources        []argoapi.ResourceStatus       `json:"resources,omitempty"`
}

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
	//+operator-sdk:csv:customerresourcedefinitions:type=status
	Applications []PatternApplicationInfo `json:"applications,omitempty"`
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
	Synced       PatternConditionType = "Synced"
	OutOfSync    PatternConditionType = "OutOfSync"
	Unknown      PatternConditionType = "Unknown"
	Degraded     PatternConditionType = "Degraded"
	Progressing  PatternConditionType = "Progressing"
	Missing      PatternConditionType = "Missing"
	Suspended    PatternConditionType = "Suspended"
)

func init() {
	SchemeBuilder.Register(&Pattern{}, &PatternList{})
}
