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

package console

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// CatalogDeploymentName is the name of the pattern-catalog Deployment
	CatalogDeploymentName = "patterns-operator-pattern-catalog"
	// CatalogContainerName is the name of the container inside the catalog Deployment
	CatalogContainerName = "patterns-operator-pattern-catalog"
	// operatorConfigMap is the name of the operator ConfigMap (mirrors controllers.OperatorConfigMap)
	operatorConfigMap = "patterns-operator-config"
)

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch

// UpdateCatalogImageIfOverridden reads the "catalog.image" key from the
// patterns-operator-config ConfigMap. If the value is non-empty it patches the
// patterns-operator-pattern-catalog Deployment to use that image. When the key
// is missing or empty, the built-in default (set by kustomize/OLM) is kept.
func UpdateCatalogImageIfOverridden(ctx context.Context, cl client.Client, reader client.Reader) error {
	logger := log.FromContext(ctx).WithName("catalog")
	ns := getDeploymentNamespace()

	// Read the operator ConfigMap via the API reader (bypasses cache, safe at startup)
	var cm corev1.ConfigMap
	if err := reader.Get(ctx, client.ObjectKey{
		Namespace: defaultNamespace,
		Name:      operatorConfigMap,
	}, &cm); err != nil {
		return fmt.Errorf("could not read operator configmap: %w", err)
	}

	image := cm.Data["catalog.image"]
	if image == "" {
		logger.Info("no catalog.image override configured, using built-in default")
		return nil
	}

	// Fetch the catalog Deployment
	deploy := &appsv1.Deployment{}
	deployKey := client.ObjectKey{Namespace: ns, Name: CatalogDeploymentName}
	if err := cl.Get(ctx, deployKey, deploy); err != nil {
		return fmt.Errorf("could not get catalog deployment %s/%s: %w", ns, CatalogDeploymentName, err)
	}

	// Find and patch the container image
	updated := false
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == CatalogContainerName {
			if deploy.Spec.Template.Spec.Containers[i].Image != image {
				deploy.Spec.Template.Spec.Containers[i].Image = image
				updated = true
			}
			break
		}
	}

	if !updated {
		logger.Info("catalog deployment already uses the configured image", "image", image)
		return nil
	}

	if err := cl.Update(ctx, deploy); err != nil {
		return fmt.Errorf("could not update catalog deployment image: %w", err)
	}

	logger.Info("catalog deployment image overridden", "image", image)
	return nil
}
