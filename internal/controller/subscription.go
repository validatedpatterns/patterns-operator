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
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newSubscription(patternsOperatorConfig PatternsOperatorConfig, disableDefaultInstance bool) *operatorv1alpha1.Subscription {
	var newSubscription *operatorv1alpha1.Subscription

	var installPlanApproval operatorv1alpha1.Approval

	if patternsOperatorConfig.getValueWithDefault("gitops.installApprovalPlan") == "Manual" {
		installPlanApproval = operatorv1alpha1.ApprovalManual
	} else {
		installPlanApproval = operatorv1alpha1.ApprovalAutomatic
	}

	spec := &operatorv1alpha1.SubscriptionSpec{
		CatalogSource:          patternsOperatorConfig.getValueWithDefault("gitops.catalogSource"),
		CatalogSourceNamespace: patternsOperatorConfig.getValueWithDefault("gitops.sourceNamespace"),
		Package:                GitOpsDefaultPackageName,
		Channel:                patternsOperatorConfig.getValueWithDefault("gitops.channel"),
		StartingCSV:            patternsOperatorConfig.getValueWithDefault("gitops.csv"),
		InstallPlanApproval:    installPlanApproval,
		Config: &operatorv1alpha1.SubscriptionConfig{
			Env: newSubscriptionEnvVars(disableDefaultInstance),
		},
	}
	subscriptionName, subscriptionNamespace := DetectGitOpsSubscription()

	newSubscription = &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      subscriptionName,
			Namespace: subscriptionNamespace,
		},
		Spec: spec,
	}

	return newSubscription
}

// newSubscriptionEnvVars returns the environment variables for the GitOps operator subscription.
// When disableDefaultInstance is true, the DISABLE_DEFAULT_ARGOCD_INSTANCE env var is added
// to prevent the gitops-operator from creating (and actively deleting) the default ArgoCD
// instance in the openshift-gitops namespace. This is safe when using a non-default ArgoCD
// name/namespace (e.g. vp-gitops) since the gitops-operator only targets "openshift-gitops".
func newSubscriptionEnvVars(disableDefaultInstance bool) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "ARGOCD_CLUSTER_CONFIG_NAMESPACES",
			Value: "*",
		},
	}
	if disableDefaultInstance {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "DISABLE_DEFAULT_ARGOCD_INSTANCE",
			Value: "true",
		})
	}
	return envVars
}

func getSubscription(client olmclient.Interface, name, namespace string) (*operatorv1alpha1.Subscription, error) {
	var subscription *operatorv1alpha1.Subscription
	var err error
	if subscription, err = client.OperatorsV1alpha1().Subscriptions(namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return subscription, nil
}

func createSubscription(client olmclient.Interface, sub *operatorv1alpha1.Subscription) error {
	_, err := client.OperatorsV1alpha1().Subscriptions(sub.Namespace).Create(context.Background(), sub, metav1.CreateOptions{})
	return err
}

func updateSubscription(client olmclient.Interface, target, current *operatorv1alpha1.Subscription) (bool, error) {
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
		_, err := client.OperatorsV1alpha1().Subscriptions(current.Namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		return changed, err
	}

	return changed, nil
}
