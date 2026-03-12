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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/operator-framework/api/pkg/operators/v1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"
)

var _ = Describe("OperatorGroup Functions", func() {
	Context("getOperatorGroup", func() {
		var fakeOlmClient *olmclient.Clientset
		const testNamespace = "openshift-gitops-operator"

		BeforeEach(func() {
			fakeOlmClient = olmclient.NewSimpleClientset()
		})

		It("should return nil, nil when OperatorGroup does not exist", func() {
			og, err := getOperatorGroup(fakeOlmClient, testNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(og).To(BeNil())
		})

		It("should return the OperatorGroup when it exists", func() {
			existing := &v1.OperatorGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testNamespace,
					Namespace: testNamespace,
				},
				Spec: v1.OperatorGroupSpec{},
			}
			_, err := fakeOlmClient.OperatorsV1().OperatorGroups(testNamespace).Create(
				context.Background(), existing, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			og, err := getOperatorGroup(fakeOlmClient, testNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(og).ToNot(BeNil())
			Expect(og.Name).To(Equal(testNamespace))
			Expect(og.Namespace).To(Equal(testNamespace))
			Expect(og.Spec).To(Equal(v1.OperatorGroupSpec{}))
		})

		It("should return error on non-NotFound API errors", func() {
			fakeOlmClient.PrependReactor("get", "operatorgroups", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("server error")
			})

			og, err := getOperatorGroup(fakeOlmClient, testNamespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("server error"))
			Expect(og).To(BeNil())
		})
	})

	Context("createOperatorGroup", func() {
		var fakeOlmClient *olmclient.Clientset
		const testNamespace = "openshift-gitops-operator"

		BeforeEach(func() {
			fakeOlmClient = olmclient.NewSimpleClientset()
		})

		It("should create an OperatorGroup with the given name in the same namespace", func() {
			err := createOperatorGroup(fakeOlmClient, testNamespace)
			Expect(err).ToNot(HaveOccurred())

			og, err := fakeOlmClient.OperatorsV1().OperatorGroups(testNamespace).Get(
				context.Background(), testNamespace, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(og).ToNot(BeNil())
			Expect(og.Name).To(Equal(testNamespace))
			Expect(og.Namespace).To(Equal(testNamespace))
			Expect(og.Spec).To(Equal(v1.OperatorGroupSpec{}))
		})

		It("should return error when create fails", func() {
			fakeOlmClient.PrependReactor("create", "operatorgroups", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("create failed")
			})

			err := createOperatorGroup(fakeOlmClient, testNamespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("create failed"))
		})

		It("should return error when OperatorGroup already exists", func() {
			err := createOperatorGroup(fakeOlmClient, testNamespace)
			Expect(err).ToNot(HaveOccurred())

			err = createOperatorGroup(fakeOlmClient, testNamespace)
			Expect(err).To(HaveOccurred())
		})
	})
})
