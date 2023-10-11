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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	subscriptionNamespace  = "openshift-operators"
	applicationNamespace   = "openshift-gitops"
	gitopsSubscriptionName = "openshift-gitops-operator"
	customConfigFile       = "/gitops-config/config.json"
)

func newSubscriptionFromConfigMap(r kubernetes.Interface) (operatorv1alpha1.Subscription, error) {
	var newSubscription *operatorv1alpha1.Subscription

	// Check if the config map exists and read the config map values
	cm, _ := r.CoreV1().ConfigMaps(OperatorNamespace).Get(context.Background(), OperatorConfigFile, metav1.GetOptions{})

	if cm != nil {
		PatternsOperatorConfig = cm.Data
	}

	var installPlanApproval operatorv1alpha1.Approval

	if configValueWithDefault(PatternsOperatorConfig, "gitops.installApprovalPlan", GitOpsDefaultApprovalPlan) == "Manual" {
		installPlanApproval = operatorv1alpha1.ApprovalManual
	} else {
		installPlanApproval = operatorv1alpha1.ApprovalAutomatic
	}

	newSpec := operatorv1alpha1.SubscriptionSpec{
		CatalogSource:          configValueWithDefault(PatternsOperatorConfig, "gitops.catalogSource", GitOpsDefaultCatalogSource),
		CatalogSourceNamespace: configValueWithDefault(PatternsOperatorConfig, "gitops.sourceNamespace", GitOpsDefaultCatalogSourceNamespace),
		Package:                configValueWithDefault(PatternsOperatorConfig, "gitops.name", GitOpsDefaultPackageName),
		Channel:                configValueWithDefault(PatternsOperatorConfig, "gitops.channel", GitOpsDefaultChannel),
		StartingCSV:            configValueWithDefault(PatternsOperatorConfig, "gitops.csv", ""),
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
			Name:      gitopsSubscriptionName,
			Namespace: subscriptionNamespace,
		},
		Spec: &newSpec,
	}

	return *newSubscription.DeepCopy(), nil
}

func getSubscription(client olmclient.Interface, name, namespace string) (*operatorv1alpha1.Subscription, error) {

	sub, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func createSubscription(client olmclient.Interface, sub *operatorv1alpha1.Subscription) error {
	_, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Create(context.Background(), sub, metav1.CreateOptions{})
	return err
}

func updateSubscription(client olmclient.Interface, target, current *operatorv1alpha1.Subscription) (error, bool) {
	changed := false
	if current == nil || current.Spec == nil {
		return fmt.Errorf("current subscription was nil"), false
	} else if target == nil || target.Spec == nil {
		return fmt.Errorf("target subscription was nil"), false
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
	} else if !reflect.DeepEqual(target.Spec.Config.Env, current.Spec.Config.Env) {
		log.Println("Config Env changed")
		changed = true
	}

	if changed {
		//		if client, err = argoclient.NewForConfig(config); err != nil {
		//			return err, changed
		//		}

		target.Spec.DeepCopyInto(current.Spec)

		_, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Update(context.Background(), current, metav1.UpdateOptions{})
		return err, changed
	}

	return nil, changed
}
