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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func haveACMHub(r *PatternReconciler) bool {
	gvrMCH := schema.GroupVersionResource{Group: "operator.open-cluster-management.io", Version: "v1", Resource: "multiclusterhubs"}

	_, err := r.dynamicClient.Resource(gvrMCH).Namespace("open-cluster-management").Get(context.Background(), "multiclusterhub", metav1.GetOptions{})
	if err != nil {
		log.Printf("Error obtaining hub: %s\n", err)
		return false
	}
	// var mangedClusters []string
	// mangedClusters, err = r.listManagedClusters(context.Background())
	// if err != nil {
	// 	log.Printf("error obtaining managed clusters: %s\n", err)
	// 	return false
	// }
	// if len(mangedClusters) == 0 {
	// 	return false
	// }
	return true
}

// listManagedClusters lists all ManagedCluster resources (excluding local-cluster)
// Returns a list of cluster names and an error
func (r *PatternReconciler) listManagedClusters(ctx context.Context) ([]string, error) {
	gvrMC := schema.GroupVersionResource{
		Group:    "cluster.open-cluster-management.io",
		Version:  "v1",
		Resource: "managedclusters",
	}

	// ManagedCluster is a cluster-scoped resource, so no namespace needed
	mcList, err := r.dynamicClient.Resource(gvrMC).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ManagedClusters: %w", err)
	}

	var clusterNames []string
	for _, item := range mcList.Items {
		name := item.GetName()
		// Exclude local-cluster (hub cluster)
		if name != "local-cluster" {
			clusterNames = append(clusterNames, name)
		}
	}

	return clusterNames, nil
}

// deleteManagedClusters deletes all ManagedCluster resources (excluding local-cluster)
// Returns the number of clusters deleted and an error
func (r *PatternReconciler) deleteManagedClusters(ctx context.Context) (int, error) {
	gvrMC := schema.GroupVersionResource{
		Group:    "cluster.open-cluster-management.io",
		Version:  "v1",
		Resource: "managedclusters",
	}

	// ManagedCluster is a cluster-scoped resource, so no namespace needed
	mcList, err := r.dynamicClient.Resource(gvrMC).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list ManagedClusters: %w", err)
	}

	deletedCount := 0
	for _, item := range mcList.Items {
		name := item.GetName()
		// Exclude local-cluster (hub cluster)
		if name == "local-cluster" {
			continue
		}

		// Delete the managed cluster
		err := r.dynamicClient.Resource(gvrMC).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			// If already deleted, that's fine
			if kerrors.IsNotFound(err) {
				continue
			}
			return deletedCount, fmt.Errorf("failed to delete ManagedCluster %q: %w", name, err)
		}
		log.Printf("Deleted ManagedCluster: %q", name)
		deletedCount++
	}

	return deletedCount, nil
}
