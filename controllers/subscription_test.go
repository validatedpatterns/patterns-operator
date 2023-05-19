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

	//	"github.com/go-logr/logr"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("pattern controller", func() {

	var _ = Context("reconciliation", func() {
		var (
			p          *api.Pattern
			qp         *api.Pattern
			reconciler *PatternReconciler
		)
		BeforeEach(func() {
			name := "subsTest"
			nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			reconciler = newFakeReconciler(nsOperators, buildNamedPatternManifest(name))

			By("obtaining the pattern object")
			p = &api.Pattern{}
			Expect(reconciler.Client.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, p)).To(Succeed())

			By("obtaining applying defaults")
			var err error
			err, qp = reconciler.applyDefaults(p)
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("cloning %s", qp.Spec.GitConfig.TargetRepo))
			Expect(cloneRepo(qp.Spec.GitConfig.TargetRepo, qp.Status.Path, "")).To(Succeed())

			By(fmt.Sprintf("checking out %s", qp.Status.Path))
			Expect(checkoutRevision(qp.Status.Path, "", qp.Spec.GitConfig.TargetRevision)).To(Succeed())

		})
		It("pattern into empty cluster", func() {
			By("building the exclusion list")
			valueFiles := newApplicationValueFiles(*qp)
			paramList := newApplicationParameters(*qp, valueFiles, reconciler.olmClient)
			fmt.Println(paramList)

			By("checking the parameter list")
			Expect(paramList).To(HaveLen(10))
		})
		It("pattern into cluster with foreign ACM", func() {
			By("creating an ACM subscription")
			sub := namedSubscription("advanced-cluster-management",
				"open-cluster-management",
				"release-2.6",
				"dummy-dummy",
				"advanced-cluster-management.v2.6.1",
				false,
				false)
			Expect(createSubscription(reconciler.olmClient, sub)).To(Succeed())

			By("building the exclusion list")
			valueFiles := newApplicationValueFiles(*qp)
			paramList := newApplicationParameters(*qp, valueFiles, reconciler.olmClient)
			fmt.Println(paramList)

			By("checking the parameter list")
			Expect(paramList).To(HaveLen(11))
			Expect(paramList[10].Name).To(Equal("clusterGroup.subscriptions.acm.disabled"))
		})
		It("pattern into cluster with DynamicSubscriptions disabled", func() {
			By("creating an ACM subscription")
			sub := namedSubscription("advanced-cluster-management",
				"open-cluster-management",
				"release-2.6",
				"dummy-dummy",
				"advanced-cluster-management.v2.6.1",
				false,
				false)
			Expect(createSubscription(reconciler.olmClient, sub)).To(Succeed())

			By("building the exclusion list")
			qp.Spec.DynamicSubscriptions = false
			valueFiles := newApplicationValueFiles(*qp)
			paramList := newApplicationParameters(*qp, valueFiles, reconciler.olmClient)
			fmt.Println(paramList)

			By("checking the parameter list")
			Expect(paramList).To(HaveLen(10))
		})
		It("pattern into cluster with self-owned ACM", func() {
			By("creating an owned ACM subscription")
			sub := namedSubscription("advanced-cluster-management",
				"open-cluster-management",
				"release-2.6",
				fmt.Sprintf("%s-%s", qp.Name, qp.Spec.ClusterGroupName),
				"advanced-cluster-management.v2.6.1",
				false,
				false)
			//Expect(controllerutil.SetOwnerReference(qp, sub, reconciler.Scheme)).To(Succeed())
			Expect(createSubscription(reconciler.olmClient, sub)).To(Succeed())

			By("building the exclusion list")
			valueFiles := newApplicationValueFiles(*qp)
			paramList := newApplicationParameters(*qp, valueFiles, reconciler.olmClient)
			fmt.Println(paramList)

			By("checking the parameter list")
			Expect(paramList).To(HaveLen(10))
		})
	})
})

func buildNamedPatternManifest(name string) *api.Pattern {
	return &api.Pattern{ObjectMeta: metav1.ObjectMeta{
		Name:       name,
		Namespace:  namespace,
		Finalizers: []string{api.PatternFinalizer},
	},
		Spec: api.PatternSpec{
			ClusterGroupName:     "hub",
			DynamicSubscriptions: true,
			GitConfig: api.GitConfig{
				TargetRevision: "main",
				TargetRepo:     "http://github.com/hybrid-cloud-patterns/multicloud-gitops",
			},
		},
	}
}
