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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/validation"

	"github.com/ghodss/yaml"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

func applyYamlFile(filename string) error {

	kubeConfigFlags := genericclioptions.NewConfigFlags(false)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	//matchVersionKubeConfigFlags.AddFlags(flags)

	//	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	//	builder := resource.NewBuilder(f.clientGetter)
	builder := resource.NewBuilder(matchVersionKubeConfigFlags)
	r := builder.
		Unstructured().
		Schema(validation.NullSchema{}).
		ContinueOnError().
		NamespaceParam("default").DefaultNamespace().
		Path(false, filename).
		SelectAllParam(true).
		Flatten().
		Do()
	if objects, err := r.Infos(); err != nil {
		log.Printf("Could not extract objects: %s\n", err.Error())
		return err
	} else {
		for _, info := range objects {
			if err := applyOneObject(info); err != nil {
				log.Printf("Could not apply objects: %s\n", err.Error())
				return err
			}
		}
	}

	//manifestsJSON, err := yaml.YAMLToJSON(manifests)
	//if err != nil {
	//	log.Printf("Error parsing manifests: %s\n", err.Error())
	//	return err, -1
	//}

	// manifests.String() =>
	//
	// WARNING: This chart or one of its subcharts contains CRDs. Rendering may fail or contain inaccuracies.
	// ---
	// # Source: pattern-install/templates/argocd/namespace.yaml
	// # Pre-create so we can create our argo app for keeping subscriptions in sync
	// # Do it here so that we don't try to sync it in the future
	// ---
	// # Source: pattern-install/templates/argocd/application.yaml
	// apiVersion: argoproj.io/v1alpha1
	// kind: Application
	// metadata:
	//   name: pattern-sample-hub
	//   namespace: openshift-gitops
	// spec:
	//   destination:
	//     name: in-cluster
	//     namespace: pattern-sample-hub
	//   project: default
	//   source:
	//     repoURL: https://github.com/hybrid-cloud-patterns/multicloud-gitops
	//     targetRevision: main
	//     path: common/clustergroup
	//     helm:
	//       valueFiles:
	//       - "https://github.com/hybrid-cloud-patterns/multicloud-gitops/raw/main/values-global.yaml"
	//       - "https://github.com/hybrid-cloud-patterns/multicloud-gitops/raw/main/values-hub.yaml"
	//       # Track the progress of https://github.com/argoproj/argo-cd/pull/6280
	//       parameters:
	//         - name: global.repoURL
	//           value: $ARGOCD_APP_SOURCE_REPO_URL
	//         - name: global.targetRevision
	//           value: $ARGOCD_APP_SOURCE_TARGET_REVISION
	//         - name: global.namespace
	//           value: $ARGOCD_APP_NAMESPACE
	//         - name: global.valuesDirectoryURL
	//           value: https://github.com/hybrid-cloud-patterns/multicloud-gitops/raw/main
	//         - name: global.pattern
	//           value: pattern-sample
	//         - name: global.hubClusterDomain
	//           value: apps.beekhof-1.blueprints.rhecoeng.com
	//   syncPolicy:
	//     automated: {}

	return nil
}

func applyOneObject(info *resource.Info) error {
	if len(info.Name) == 0 {
		metadata, _ := meta.Accessor(info.Object)
		generatedName := metadata.GetGenerateName()
		if len(generatedName) > 0 {
			return fmt.Errorf("from %s: cannot use generate name with apply", generatedName)
		}
	}

	helper := resource.NewHelper(info.Client, info.Mapping).
		DryRun(false).
		WithFieldManager(apply.FieldManagerClientSideApply)

	// Send the full object to be applied on the server side.
	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, info.Object)
	if err != nil {
		return err
	}

	forceConflicts := true
	options := metav1.PatchOptions{
		Force: &forceConflicts,
	}

	obj, err := helper.Patch(
		info.Namespace,
		info.Name,
		types.ApplyPatchType,
		data,
		&options,
	)
	if err != nil {
		return err
	}

	info.Refresh(obj, true)
	return nil
}

func haveNamespace(config *rest.Config, name string) bool {
	if client, err := clientset.NewForConfig(config); err != nil {
		return false
	} else if _, err := client.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{}); err == nil {
		return true
	}
	return false
}

func createOwnerRef(p *api.Pattern) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: api.GroupVersion.String(),
		Kind:       p.Kind, // String
		UID:        p.GetUID(),
		Name:       p.GetName(),
	}
}

func ownedBySame(expected, object metav1.Object) bool {
	ownerReferences := expected.GetOwnerReferences()

	for _, r := range ownerReferences {
		if ownedBy(object, r) == false {
			return false
		}
	}
	return true
}

func ownedBy(object metav1.Object, ref metav1.OwnerReference) bool {

	ownerReferences := object.GetOwnerReferences()

	for _, r := range ownerReferences {
		if referSameObject(r, ref) {
			return true
		}
	}

	return false
}

func objectYaml(object metav1.Object) string {

	if yamlString, err := yaml.Marshal(object); err != nil {
		return fmt.Sprintf("Error marshalling object: %s\n", err.Error())
	} else {
		return string(yamlString)
	}
}

// Returns true if a and b point to the same object.
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}
