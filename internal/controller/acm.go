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

// https://github.com/stolostron/cm-cli/blob/64e944330f6ca20c559abcd382d7712f10cb904f/pkg/cmd/cmd.go#L75
import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var mchGVK = schema.GroupVersionKind{
	Group:   "operator.open-cluster-management.io",
	Kind:    "multiclusterhubs",
	Version: "v1",
}

func haveACMHub(cl client.Client) bool {
	labelSelector, err := labels.Parse(fmt.Sprintf("%v = %v", "ocm-configmap-type", "image-manifest"))

	if err != nil {
		log.Printf("config map error: %s\n", err.Error())
		return false
	}

	cms := corev1.ConfigMapList{}
	err = cl.List(context.Background(), &cms, &client.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		log.Printf("config map error: %s\n", err.Error())
		return false
	}
	if len(cms.Items) == 0 {
		log.Printf("No config map\n")
		return false
	}
	ns := cms.Items[0].Namespace

	umch := &unstructured.UnstructuredList{}
	umch.SetGroupVersionKind(mchGVK)

	err = cl.List(context.Background(), umch, &client.ListOptions{Namespace: ns})

	if err != nil {
		log.Printf("Error obtaining hub: %s\n", err)
		return false
	} else if len(umch.Items) == 0 {
		log.Printf("No hub in %s\n", ns)
		return false
	}
	return true
}
