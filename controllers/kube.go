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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/ghodss/yaml"
)

func ownedBySame(expected, object metav1.Object) bool {
	ownerReferences := expected.GetOwnerReferences()

	for _, r := range ownerReferences {
		if !ownedBy(object, r) {
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
