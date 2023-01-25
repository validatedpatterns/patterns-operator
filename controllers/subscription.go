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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
)

const (
	subscriptionNamespace  = "openshift-operators"
	applicationNamespace   = "openshift-gitops"
	gitopsSubscriptionName = "openshift-gitops-operator"
)

func newSubscription(p api.Pattern) *operatorv1alpha1.Subscription {

	return &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitopsSubscriptionName,
			Namespace: subscriptionNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          "redhat-operators",
			CatalogSourceNamespace: "openshift-marketplace",
			Channel:                "stable",
			Package:                "openshift-gitops-operator",
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}
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
