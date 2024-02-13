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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GiteaServerSpec defines the desired state of GiteaServer
type GiteaServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Helm Chart URL. Default value: https://charts.validatedpatterns.io
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1
	HelmChartUrl string `json:"helmChartUrl,omitempty"`
	// Namespace where helm chart will be deployed to. Default: gitea
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=2
	Namespace string `json:"namespace,omitempty"`
	// Helm Repo name. Default: helm-charts
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=3
	RepoName string `json:"repoName,omitempty"`
	// Chart Name that we will deploy. Default: gitea-chart
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=4
	ChartName string `json:"chartName,omitempty"`
	// Version for the chart. Default: 0.0.3
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=5
	Version string `json:"version,omitempty"`
	// Release name used to deploy the chart.  Default: gitea
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=6
	ReleaseName string `json:"releaseName,omitempty"`
}

const (
	GiteaServerFinalizer string = "foregroundDeleteGiteaServer"
)

// GiteaServerStatus defines the observed state of GiteaServer
type GiteaServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Status of the chart
	ChartStatus string `json:"chartStatus,omitempty"`

	// Route for the service
	Route string `json:"route,omitempty"`

	// Last action related to the Gitea deployment
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastStep string `json:"lastStep,omitempty"`

	// Last error encountered by the pattern
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastError string `json:"lastError,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GiteaServer is the Schema for the giteaservers API
type GiteaServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GiteaServerSpec   `json:"spec,omitempty"`
	Status GiteaServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GiteaServerList contains a list of GiteaServer
type GiteaServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GiteaServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GiteaServer{}, &GiteaServerList{})
}
