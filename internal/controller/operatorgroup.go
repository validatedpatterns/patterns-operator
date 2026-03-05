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

	v1 "github.com/operator-framework/api/pkg/operators/v1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getOperatorGroup(client olmclient.Interface, name string) (*v1.OperatorGroup, error) {
	var og *v1.OperatorGroup
	var err error
	if og, err = client.OperatorsV1().OperatorGroups(name).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return og, nil
}

func createOperatorGroup(client olmclient.Interface, name string) error {
	_, err := client.OperatorsV1().OperatorGroups(name).Create(
		context.Background(),
		&v1.OperatorGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: name},
			Spec: v1.OperatorGroupSpec{}},
		metav1.CreateOptions{})
	return err
}
