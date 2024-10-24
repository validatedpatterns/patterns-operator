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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func haveACMHub(r *PatternReconciler) bool {
	gvrMCH := schema.GroupVersionResource{Group: "operator.open-cluster-management.io", Version: "v1", Resource: "multiclusterhubs"}

	serverNamespace := ""

	cms, err := r.fullClient.CoreV1().ConfigMaps("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%v = %v", "ocm-configmap-type", "image-manifest"),
	})
	if (err != nil || len(cms.Items) == 0) && serverNamespace != "" {
		cms, err = r.fullClient.CoreV1().ConfigMaps(serverNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%v = %v", "ocm-configmap-type", "image-manifest"),
		})
	}
	if err != nil || len(cms.Items) == 0 {
		cms, err = r.fullClient.CoreV1().ConfigMaps("open-cluster-management").List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%v = %v", "ocm-configmap-type", "image-manifest"),
		})
	}
	if err != nil {
		log.Printf("config map error: %s\n", err.Error())
		return false
	}
	if len(cms.Items) == 0 {
		log.Printf("No config map\n")
		return false
	}
	ns := cms.Items[0].Namespace

	umch, err := r.dynamicClient.Resource(gvrMCH).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error obtaining hub: %s\n", err)
		return false
	} else if len(umch.Items) == 0 {
		log.Printf("No hub in %s\n", ns)
		return false
	}
	return true
}
