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

	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"gopkg.in/yaml.v3"
)

func haveNamespace(controllerClient kubeclient.Client, name string) bool {
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

func objectYaml(object any) (string, error) {
	yamlBytes, err := yaml.Marshal(object)
	if err != nil {
		return "", fmt.Errorf("error marshaling object: %w", err)
	}
	return string(yamlBytes), nil
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

	return aGV.Version == bGV.Version && aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name && a.UID == b.UID
}

func getRoute(cl kubeclient.Client, routeName, namespace string) (string, error) {
	route := routev1.Route{}

	err := cl.Get(context.Background(), types.NamespacedName{Name: routeName, Namespace: namespace}, &route)
	if err != nil {
		return "", err
	}

	if len(route.Status.Ingress) == 0 {
		return "", fmt.Errorf("no ingress found for route %s", routeName)
	}

	// Return the first ingress host
	url := fmt.Sprintf("https://%s", route.Status.Ingress[0].Host)

	return url, nil
}

// Get a Secret instance
func getSecret(cl kubeclient.Client, name, ns string) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}
