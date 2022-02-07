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
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientset "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	"k8s.io/client-go/rest"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
)

const (
	subscriptionNamespace = "openshift-operators"
	applicationNamespace  = "openshift-gitops"
)

func newSubscription(p api.Pattern) *operatorv1alpha1.Subscription {
	//  apiVersion: operators.coreos.com/v1alpha1
	//  kind: Subscription
	//  metadata:
	//    name: openshift-gitops-operator
	//    namespace: openshift-operators
	//  spec:
	//    channel: stable
	//    installPlanApproval: Automatic
	//    name: openshift-gitops-operator
	//    source: redhat-operators
	//    sourceNamespace: openshift-marketplace
	//    startingCSV: openshift-gitops-operator.v1.4.1

	spec := &operatorv1alpha1.SubscriptionSpec{
		CatalogSource:          "redhat-operators",
		CatalogSourceNamespace: "openshift-marketplace",
		Channel:                p.Spec.GitOpsConfig.OperatorChannel,
		Package:                "openshift-gitops-operator",
		InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
	}

	if p.Spec.GitOpsConfig.UseCSV {
		spec.StartingCSV = p.Spec.GitOpsConfig.OperatorCSV
	}

	if p.Spec.GitOpsConfig.InstallPlanApproval == "Manual" {
		spec.InstallPlanApproval = operatorv1alpha1.ApprovalManual
	}

	return &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift-gitops-operator",
			Namespace: subscriptionNamespace,
		},
		Spec: spec,
	}
}

func getSubscription(config *rest.Config, name, namespace string) (error, *operatorv1alpha1.Subscription) {

	if client, err := clientset.NewForConfig(config); err != nil {
		return err, nil
	} else {
		sub, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Get(context.Background(), name, metav1.GetOptions{})
		return err, sub
	}
}

func createSubscription(config *rest.Config, sub *operatorv1alpha1.Subscription) error {
	//       var clusterSubscriptions olmapi.SubscriptionList
	//       if tokenSecret, err = client.Core().Secrets(secret.Namespace).Get(secret.Name); err != nil {
	//       if err := client.List(context.TODO(), &clusterSubscriptions, &client.ListOptions{}); err == nil {
	// ./vendor/k8s.io/client-go/metadata/metadata.go:181:func (c *client) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*metav1.PartialObjectMetadata, error) {
	if client, err := clientset.NewForConfig(config); err != nil {
		return err
	} else {
		_, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Create(context.Background(), sub, metav1.CreateOptions{})
		return err
	}
}

func updateSubscription(config *rest.Config, target, current *operatorv1alpha1.Subscription) (error, bool) {
	changed := false

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
	}

	if changed {
		//		if client, err = argoclient.NewForConfig(config); err != nil {
		//			return err, changed
		//		}

		target.Spec.DeepCopyInto(current.Spec)

		if client, err := clientset.NewForConfig(config); err != nil {
			return err, changed
		} else {
			_, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Update(context.Background(), current, metav1.UpdateOptions{})
			return err, changed
		}
	}

	return nil, changed
}
