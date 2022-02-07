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
	"k8s.io/client-go/rest"

	argoapi "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

//apiVersion: argoproj.io/v1alpha1
//kind: Application
//metadata:
//  name: {{ .Release.Name }}-{{ .Values.main.clusterGroupName }}
//  namespace: openshift-gitops

func newApplication(p api.Pattern) *argoapi.Application {

	// Argo uses...
	// r := regexp.MustCompile("(/|:)")
	// root := filepath.Join(os.TempDir(), r.ReplaceAllString(NormalizeGitURL(rawRepoURL), "_"))

	spec := argoapi.ApplicationSpec{

		// Source is a reference to the location of the application's manifests or chart
		Source: argoapi.ApplicationSource{
			RepoURL:        p.Spec.GitConfig.TargetRepo,
			Path:           "common/clustergroup",
			TargetRevision: p.Spec.GitConfig.TargetRevision,
			Helm: &argoapi.ApplicationSourceHelm{
				ValueFiles: []string{
					// Track the progress of https://github.com/argoproj/argo-cd/pull/6280
					fmt.Sprintf("%s/values-global.yaml", p.Spec.GitConfig.ValuesDirectoryURL),
					fmt.Sprintf("%s/values-%s.yaml", p.Spec.GitConfig.ValuesDirectoryURL, p.Spec.ClusterGroupName),
				},
				// Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation
				Parameters: []argoapi.HelmParameter{
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
						Name:  "global.namespace",
						Value: p.Namespace,
					},
					{
						Name:  "global.valuesDirectoryURL",
						Value: p.Spec.GitConfig.ValuesDirectoryURL,
					},
					{
						Name:  "global.pattern",
						Value: p.Name,
					},
					{
						Name:  "global.hubClusterDomain",
						Value: p.Status.ClusterDomain,
					},
				},
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

	if p.Spec.GitOpsConfig.SyncPolicy == "Automatic" {
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

	app := argoapi.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationName(p),
			Namespace: applicationNamespace,
		},
		Spec: spec,
	}

	log.Printf("Generated: %s\n", objectYaml(&app))
	return &app

}

func applicationName(p api.Pattern) string {
	return fmt.Sprintf("%s-%s", p.Name, p.Spec.ClusterGroupName)
}

func getApplication(config *rest.Config, name string) (error, *argoapi.Application) {
	if client, err := argoclient.NewForConfig(config); err != nil {
		return err, nil
	} else {
		if app, err := client.ArgoprojV1alpha1().Applications(applicationNamespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			return err, nil
		} else {
			//			log.Printf("Retrieved: %s\n", objectYaml(app))
			return nil, app
		}
	}
}

func createApplication(config *rest.Config, app *argoapi.Application) error {
	//	var client argoclient.Interface
	if client, err := argoclient.NewForConfig(config); err != nil {
		return err
	} else {
		_, err := client.ArgoprojV1alpha1().Applications(applicationNamespace).Create(context.Background(), app, metav1.CreateOptions{})
		return err
	}
}

func updateApplication(config *rest.Config, target, current *argoapi.Application) (error, bool) {
	//	var client argoclient.Interface
	changed := false

	if current == nil {
		return fmt.Errorf("current application was nil"), false
	} else if target == nil {
		return fmt.Errorf("target application was nil"), false
	}

	if target.Spec.Source.RepoURL != current.Spec.Source.RepoURL {
		log.Printf("RepoURL changed %s -> %s\n", current.Spec.Source.RepoURL, target.Spec.Source.RepoURL)
		changed = true
	} else if target.Spec.Source.TargetRevision != current.Spec.Source.TargetRevision {
		log.Println("CatalogSource changed")
		changed = true
		// TODO: Add more conditions...
	}

	if changed == false {
		return nil, changed
	}

	if client, err := argoclient.NewForConfig(config); err != nil {
		return err, changed
	} else {
		log.Printf("Updating: %s\n", objectYaml(current))

		spec := current.Spec.DeepCopy()
		target.Spec.DeepCopyInto(spec)
		current.Spec = *spec

		log.Printf("Sending update: %s\n", objectYaml(current))

		_, err := client.ArgoprojV1alpha1().Applications(applicationNamespace).Update(context.Background(), current, metav1.UpdateOptions{})
		return err, changed
	}
}

func removeApplication(config *rest.Config, p api.Pattern) error {
	return fmt.Errorf("not implemented")
}
