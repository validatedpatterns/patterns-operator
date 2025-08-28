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
	"reflect"

	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newSubscriptionFromConfigMap(cl client.Client) (*operatorv1alpha1.Subscription, error) {
	var newSubscription *operatorv1alpha1.Subscription

	// Check if the config map exists and read the config map values
	cm := &corev1.ConfigMap{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: OperatorConfigMap, Namespace: OperatorNamespace}, cm)
	// If we hit an error that is not related to the configmap not existing bubble it up
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if cm.Data != nil {
		PatternsOperatorConfig = cm.Data
	}

	var installPlanApproval operatorv1alpha1.Approval

	if PatternsOperatorConfig.getValueWithDefault("gitops.installApprovalPlan") == "Manual" {
		installPlanApproval = operatorv1alpha1.ApprovalManual
	} else {
		installPlanApproval = operatorv1alpha1.ApprovalAutomatic
	}

	spec := &operatorv1alpha1.SubscriptionSpec{
		CatalogSource:          PatternsOperatorConfig.getValueWithDefault("gitops.catalogSource"),
		CatalogSourceNamespace: PatternsOperatorConfig.getValueWithDefault("gitops.sourceNamespace"),
		Package:                PatternsOperatorConfig.getValueWithDefault("gitops.name"),
		Channel:                PatternsOperatorConfig.getValueWithDefault("gitops.channel"),
		StartingCSV:            PatternsOperatorConfig.getValueWithDefault("gitops.csv"),
		InstallPlanApproval:    installPlanApproval,
		Config: &operatorv1alpha1.SubscriptionConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "ARGOCD_CLUSTER_CONFIG_NAMESPACES",
					Value: "*",
				},
			},
		},
	}

	newSubscription = &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GitOpsDefaultPackageName,
			Namespace: SubscriptionNamespace,
		},
		Spec: spec,
	}

	return newSubscription, nil
}

func getSubscription(cl client.Client, name string) (*operatorv1alpha1.Subscription, error) {
	sub := &operatorv1alpha1.Subscription{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: name, Namespace: SubscriptionNamespace}, sub)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func createSubscription(cl client.Client, sub *operatorv1alpha1.Subscription) error {
	return cl.Create(context.Background(), sub)
}

func updateSubscription(cl client.Client, target, current *operatorv1alpha1.Subscription) (bool, error) {
	changed := false
	if current == nil || current.Spec == nil {
		return false, fmt.Errorf("current subscription was nil")
	} else if target == nil || target.Spec == nil {
		return false, fmt.Errorf("target subscription was nil")
	}

	if target.Spec.CatalogSourceNamespace != current.Spec.CatalogSourceNamespace {
		log.Println("CatalogSourceNamespace changed")
		changed = true
	} else if target.Spec.CatalogSource != current.Spec.CatalogSource {
		log.Println("CatalogSource changed")
		changed = true
	} else if target.Spec.Channel != current.Spec.Channel {
		log.Println("Channel changed")
		changed = true
	} else if target.Spec.Package != current.Spec.Package {
		log.Println("Package changed")
		changed = true
	} else if target.Spec.InstallPlanApproval != current.Spec.InstallPlanApproval {
		log.Println("InstallPlanApproval changed")
		changed = true
	} else if target.Spec.StartingCSV != current.Spec.StartingCSV {
		log.Println("StartingCSV changed")
		changed = true
	} else if target.Spec.Config != nil && current.Spec.Config != nil &&
		!reflect.DeepEqual(target.Spec.Config.Env, current.Spec.Config.Env) {
		log.Println("Config Env changed")
		changed = true
	} else if target.Spec.Config == nil && current.Spec.Config != nil {
		log.Println("Config Env changed")
		changed = true
	} else if current.Spec.Config == nil && target.Spec.Config != nil {
		log.Println("Config Env changed")
		changed = true
	}

	if changed {
		target.Spec.DeepCopyInto(current.Spec)
		err := cl.Update(context.Background(), current)
		return changed, err
	}

	return changed, nil
}
