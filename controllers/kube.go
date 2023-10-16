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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ghodss/yaml"
)

func haveNamespace(controllerClient client.Client, name string) bool {
	ns := &v1.Namespace{}
	if err := controllerClient.Get(context.Background(), types.NamespacedName{Name: name}, ns); err == nil {
		return true
	}
	return false
}

func ownedBySame(expected, object metav1.Object) bool {
	ownerReferences := expected.GetOwnerReferences()

	for r := range ownerReferences {
		if !ownedBy(object, &ownerReferences[r]) {
			return false
		}
	}
	return true
}

func ownedBy(object metav1.Object, ref *metav1.OwnerReference) bool {

	ownerReferences := object.GetOwnerReferences()

	for r := range ownerReferences {
		if referSameObject(&ownerReferences[r], ref) {
			return true
		}
	}

	return false
}

func objectYaml(object metav1.Object) string {
	if yamlString, err := yaml.Marshal(object); err != nil {
		return fmt.Sprintf("Error marshaling object: %s\n", err.Error())
	} else {
		return string(yamlString)
	}
}

// Returns true if a and b point to the same object.
func referSameObject(a, b *metav1.OwnerReference) bool {
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
