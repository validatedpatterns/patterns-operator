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

package controllers

import (
	"context"
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoapi "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

func newApplicationParameters(p api.Pattern) []argoapi.HelmParameter {

	parameters := []argoapi.HelmParameter{
		{
			Name:  "global.pattern",
			Value: p.Name,
		},
		{
			Name:  "global.namespace",
			Value: p.Namespace,
		},
		{
			Name:  "global.repoURL",
			Value: p.Spec.GitConfig.TargetRepo,
			//						ForceString true,
		},
		{
			Name:  "global.targetRevision",
			Value: p.Spec.GitConfig.TargetRevision,
		},
		{
			Name:  "global.hubClusterDomain",
			Value: p.Status.AppClusterDomain,
		},
		{
			Name:  "global.localClusterDomain",
			Value: p.Status.AppClusterDomain,
		},
		{
			Name:  "global.clusterDomain",
			Value: p.Status.ClusterDomain,
		},
		{
			Name:  "global.clusterVersion",
			Value: p.Status.ClusterVersion,
		},
		{
			Name:  "global.clusterPlatform",
			Value: p.Status.ClusterPlatform,
		},
		{
			Name:  "global.localClusterName",
			Value: p.Status.ClusterName,
		},
	}

	for _, extra := range p.Spec.ExtraParameters {
		if !updateHelmParameter(extra, parameters) {
			log.Printf("Parameter %q = %q added", extra.Name, extra.Value)
			parameters = append(parameters, argoapi.HelmParameter{
				Name:  extra.Name,
				Value: extra.Value,
			})
		}
	}
	if !p.ObjectMeta.DeletionTimestamp.IsZero() {
		parameters = append(parameters, argoapi.HelmParameter{
			Name:        "global.deletePattern",
			Value:       "1",
			ForceString: true,
		})
	}
	return parameters
}

func newApplicationValueFiles(p api.Pattern) []string {

	files := []string{
		"/values-global.yaml",
		fmt.Sprintf("/values-%s.yaml", p.Spec.ClusterGroupName),
		fmt.Sprintf("/values-%s.yaml", p.Status.ClusterPlatform),
		fmt.Sprintf("/values-%s-%s.yaml", p.Status.ClusterPlatform, p.Status.ClusterVersion),
		fmt.Sprintf("/values-%s-%s.yaml", p.Status.ClusterPlatform, p.Spec.ClusterGroupName),
		fmt.Sprintf("/values-%s-%s.yaml", p.Status.ClusterVersion, p.Spec.ClusterGroupName),
		fmt.Sprintf("/values-%s.yaml", p.Status.ClusterName),
	}

	for _, extra := range p.Spec.ExtraValueFiles {
		log.Printf("Values file %q added", extra)
		files = append(files, extra)
	}
	return files
}

func newApplicationValues(p api.Pattern) string {
	s := "extraParametersNested:\n"
	for _, extra := range p.Spec.ExtraParameters {
		line := fmt.Sprintf("  %s: %s\n", extra.Name, extra.Value)
		s = s + line
	}
	return s
}

func newApplication(p api.Pattern) *argoapi.Application {

	// Argo uses...
	// r := regexp.MustCompile("(/|:)")
	// root := filepath.Join(os.TempDir(), r.ReplaceAllString(NormalizeGitURL(rawRepoURL), "_"))

	spec := argoapi.ApplicationSpec{

		// Source is a reference to the location of the application's manifests or chart
		Source: &argoapi.ApplicationSource{
			RepoURL:        p.Spec.GitConfig.TargetRepo,
			Path:           "common/clustergroup",
			TargetRevision: p.Spec.GitConfig.TargetRevision,
			Helm: &argoapi.ApplicationSourceHelm{
				ValueFiles: newApplicationValueFiles(p),

				// Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation
				Parameters: newApplicationParameters(p),

				// This is to be able to pass down the extraParams to the single applications
				Values: newApplicationValues(p),
				// ReleaseName is the Helm release name to use. If omitted it will use the application name
				// ReleaseName string `json:"releaseName,omitempty" protobuf:"bytes,3,opt,name=releaseName"`
				// Values specifies Helm values to be passed to helm template, typically defined as a block
				// Values string `json:"values,omitempty" protobuf:"bytes,4,opt,name=values"`
				// FileParameters are file parameters to the helm template
				// FileParameters []HelmFileParameter `json:"fileParameters,omitempty" protobuf:"bytes,5,opt,name=fileParameters"`
				// Version is the Helm version to use for templating (either "2" or "3")
				// Version string `json:"version,omitempty" protobuf:"bytes,6,opt,name=version"`
				// PassCredentials pass credentials to all domains (Helm's --pass-credentials)
				// PassCredentials bool `json:"passCredentials,omitempty" protobuf:"bytes,7,opt,name=passCredentials"`
				// IgnoreMissingValueFiles prevents helm template from failing when valueFiles do not exist locally by not appending them to helm template --values
				// Only applies to local files
				IgnoreMissingValueFiles: true,
				// SkipCrds skips custom resource definition installation step (Helm's --skip-crds)
				// SkipCrds bool `json:"skipCrds,omitempty" protobuf:"bytes,9,opt,name=skipCrds"`
			},
		},
		Destination: argoapi.ApplicationDestination{
			Name:      "in-cluster",
			Namespace: p.Namespace,
		},
		// Project is a reference to the project this application belongs to.
		// The empty string means that application belongs to the 'default' project.
		Project: "default",

		// IgnoreDifferences is a list of resources and their fields which should be ignored during comparison
		// IgnoreDifferences []ResourceIgnoreDifferences `json:"ignoreDifferences,omitempty" protobuf:"bytes,5,name=ignoreDifferences"`
		// Info contains a list of information (URLs, email addresses, and plain text) that relates to the application
		// Info []Info `json:"info,omitempty" protobuf:"bytes,6,name=info"`
		// RevisionHistoryLimit limits the number of items kept in the application's revision history, which is used for informational purposes as well as for rollbacks to previous versions.
		// This should only be changed in exceptional circumstances.
		// Setting to zero will store no history. This will reduce storage used.
		// Increasing will increase the space used to store the history, so we do not recommend increasing it.
		// Default is 10.
		// RevisionHistoryLimit *int64 `json:"revisionHistoryLimit,omitempty" protobuf:"bytes,7,name=revisionHistoryLimit"`

	}

	if !p.ObjectMeta.DeletionTimestamp.IsZero() {
		spec.SyncPolicy = &argoapi.SyncPolicy{
			// Automated will keep an application synced to the target revision
			Automated: &argoapi.SyncPolicyAutomated{
				Prune: true,
			},
			// Options allow you to specify whole app sync-SyncOptions
			SyncOptions: []string{"Prune=true"},
		}

	} else if !p.Spec.GitOpsConfig.ManualSync {
		// SyncPolicy controls when and how a sync will be performed
		spec.SyncPolicy = &argoapi.SyncPolicy{
			// Automated will keep an application synced to the target revision
			Automated: &argoapi.SyncPolicyAutomated{},
			// Options allow you to specify whole app sync-options
			SyncOptions: []string{},
			// Retry controls failed sync retry behavior
			// Retry *RetryStrategy `json:"retry,omitempty" protobuf:"bytes,3,opt,name=retry"`
		}
	}

	labels := make(map[string]string)
	labels["pattern"] = applicationName(p)
	app := argoapi.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationName(p),
			Namespace: applicationNamespace,
			Labels:    labels,
		},
		Spec: spec,
	}

	controllerutil.AddFinalizer(&app, argoapi.ForegroundPropagationPolicyFinalizer)
	return &app

}

func applicationName(p api.Pattern) string {
	return fmt.Sprintf("%s-%s", p.Name, p.Spec.ClusterGroupName)
}

func getApplication(client argoclient.Interface, name string) (error, *argoapi.Application) {
	if app, err := client.ArgoprojV1alpha1().Applications(applicationNamespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
		return err, nil
	} else {
		//			log.Printf("Retrieved: %s\n", objectYaml(app))
		return nil, app
	}
}

func createApplication(client argoclient.Interface, app *argoapi.Application) error {
	saved, err := client.ArgoprojV1alpha1().Applications(applicationNamespace).Create(context.Background(), app, metav1.CreateOptions{})
	log.Printf("Created: %s\n", objectYaml(saved))
	return err
}

func updateApplication(client argoclient.Interface, target, current *argoapi.Application) (error, bool) {
	//	var client argoclient.Interface
	if current == nil {
		return fmt.Errorf("current application was nil"), false
	} else if target == nil {
		return fmt.Errorf("target application was nil"), false
	}

	if compareSource(target.Spec.Source, current.Spec.Source) {
		return nil, false
	}

	spec := current.Spec.DeepCopy()
	target.Spec.DeepCopyInto(spec)
	current.Spec = *spec

	_, err := client.ArgoprojV1alpha1().Applications(applicationNamespace).Update(context.Background(), current, metav1.UpdateOptions{})
	return err, true
}

func removeApplication(client argoclient.Interface, name string) error {
	return client.ArgoprojV1alpha1().Applications(applicationNamespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func compareSource(goal, actual *argoapi.ApplicationSource) bool {
	if goal.RepoURL != actual.RepoURL {
		log.Printf("RepoURL changed %s -> %s\n", actual.RepoURL, goal.RepoURL)
		return false
	}

	if goal.TargetRevision != actual.TargetRevision {
		log.Printf("TargetRevision changed %s -> %s\n", actual.TargetRevision, goal.TargetRevision)
		return false
	}

	if goal.Path != actual.Path {
		log.Printf("Path changed %s -> %s\n", actual.Path, goal.Path)
		return false
	}

	return compareHelmSource(*goal.Helm, *actual.Helm)

}

func compareHelmSource(goal, actual argoapi.ApplicationSourceHelm) bool {
	if !compareHelmValueFiles(goal.ValueFiles, actual.ValueFiles) {
		return false
	}
	if !compareHelmParameters(goal.Parameters, actual.Parameters) {
		return false
	}
	return true
}

func compareHelmParameter(goal argoapi.HelmParameter, actual []argoapi.HelmParameter) bool {
	for _, param := range actual {
		if goal.Name == param.Name {
			if goal.Value == param.Value {
				return true
			}
			log.Printf("Parameter %q changed: %q -> %q", goal.Name, param.Value, goal.Value)
			return false
		}
	}
	log.Printf("Parameter %q not found", goal.Name)
	return false

}

func compareHelmParameters(goal, actual []argoapi.HelmParameter) bool {
	if len(goal) != len(actual) {
		return false
	}

	for _, gP := range goal {
		if !compareHelmParameter(gP, actual) {
			return false
		}

	}
	return true
}

func compareHelmValueFile(goal string, actual []string) bool {
	for _, value := range actual {
		if goal == value {
			return true
		}
	}
	log.Printf("Values file %q not found", goal)
	return false

}
func compareHelmValueFiles(goal []string, actual []string) bool {
	if len(goal) != len(actual) {
		return false
	}
	for _, gV := range goal {
		if !compareHelmValueFile(gV, actual) {
			return false
		}
	}
	return true
}

func updateHelmParameter(goal api.PatternParameter, actual []argoapi.HelmParameter) bool {
	for _, param := range actual {
		if goal.Name == param.Name {
			if goal.Value == param.Value {
				return true
			}
			log.Printf("Parameter %q updated: %q -> %q", goal.Name, param.Value, goal.Value)
			param.Value = goal.Value
			return true
		}
	}
	return false
}
