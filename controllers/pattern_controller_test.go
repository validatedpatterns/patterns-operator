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
	"time"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/go-logr/logr"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/fake"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	namespace       = "openshift-operators"
	defaultInterval = time.Duration(180) * time.Second
)

var (
	patternNamespaced = types.NamespacedName{Name: foo, Namespace: namespace}
	gitopsNamespace   = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-gitops"}}
)
var _ = Describe("pattern controller", func() {

	var _ = Context("reconciliation", func() {
		var (
			p          *api.Pattern
			reconciler *PatternReconciler
			watch      *watcher
		)
		BeforeEach(func() {
			nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			reconciler = newFakeReconciler(nsOperators, buildPatternManifest(10))
			watch = reconciler.driftWatcher.(*watcher)

		})
		It("adding a pattern with origin, target and interval >-1", func() {
			By("adding the pattern to the watch")
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(1))
			Expect(watch.repoPairs[0].name).To(Equal(foo))
			Expect(watch.repoPairs[0].namespace).To(Equal(namespace))
			Expect(watch.repoPairs[0].interval).To(Equal(defaultInterval))
		})

		It("adding a pattern without origin Repository", func() {
			p = &api.Pattern{}
			err := reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitConfig.OriginRepo = ""
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())

			By("validating the watch slice is empty")
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(0))
		})

		It("adding a pattern with interval == -1", func() {
			p = &api.Pattern{}
			err := reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitConfig.PollInterval = -1
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())

			By("validating the watch slice is empty")
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(0))
		})
		It("validates changes to the poll interval in the manifest", func() {
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(1))

			By("updating the pattern's interval")
			p = &api.Pattern{}
			err := reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitConfig.PollInterval = 200
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(1))
			Expect(watch.repoPairs[0].name).To(Equal(foo))
			Expect(watch.repoPairs[0].namespace).To(Equal(namespace))
			Expect(watch.repoPairs[0].interval).To(Equal(time.Duration(200) * time.Second))

			By("disabling the watch by updating the interval to be -1")
			err = reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitConfig.PollInterval = -1
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(0))
			By("reenabling the watch by setting the interval to a value greater than 0 but below the minimum interval of 180 seconds")
			err = reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitConfig.PollInterval = 100
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(1))
			Expect(watch.repoPairs[0].name).To(Equal(foo))
			Expect(watch.repoPairs[0].namespace).To(Equal(namespace))
			Expect(watch.repoPairs[0].interval).To(Equal(defaultInterval))
		})
		It("removes an existing pattern from the drift watcher by changing the originRepository to empty", func() {
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(1))

			By("disabling the watch by updating the originRepository to be empty")
			p = &api.Pattern{}
			err := reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitConfig.OriginRepo = ""
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(0))
			By("reenabling the watch by resetting the originRepository value")
			err = reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitConfig.OriginRepo = originURL
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())
			_, _ = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(watch.repoPairs).To(HaveLen(1))
			Expect(watch.repoPairs[0].name).To(Equal(foo))
			Expect(watch.repoPairs[0].namespace).To(Equal(namespace))
			Expect(watch.repoPairs[0].interval).To(Equal(defaultInterval))
		})
	})

	var _ = Context("Validates the gitops subscription updates due to pattern specs", func() {
		var (
			reconciler         *PatternReconciler
			pattern1, pattern2 api.Pattern
		)
		BeforeEach(func() {

			pattern1 = api.Pattern{ObjectMeta: metav1.ObjectMeta{
				Name:       foo,
				Namespace:  namespace,
				Finalizers: []string{api.PatternFinalizer},
			},
				Spec: api.PatternSpec{
					ClusterGroupName: "default",
					GitConfig: api.GitConfig{
						TargetRepo: targetURL,
					},
					GitOpsConfig: &api.GitOpsConfig{OperatorChannel: "foo"},
				},
			}

			pattern2 = api.Pattern{ObjectMeta: metav1.ObjectMeta{
				Name:       bar,
				Namespace:  namespace,
				Finalizers: []string{api.PatternFinalizer},
			},
				Spec: api.PatternSpec{
					ClusterGroupName: "default",
					GitConfig: api.GitConfig{
						TargetRepo: originURL,
					},
					GitOpsConfig: &api.GitOpsConfig{OperatorChannel: "bar"},
				},
			}

			operatorsNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			reconciler = newFakeReconciler(operatorsNamespace, gitopsNamespace, &pattern1)

		})
		It("when deploying a second pattern", func() {
			By("reconciling the first pattern when the gitops subscription does not exist")
			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(err).NotTo(HaveOccurred())
			err = reconciler.Client.Create(context.Background(), &pattern2, &client.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			subs, err := reconciler.olmClient.OperatorsV1alpha1().Subscriptions("openshift-operators").Get(context.Background(), "openshift-gitops-operator", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(subs).NotTo(BeNil())
			Expect(subs.Spec.Channel).To(Equal("foo"))
			By("reconciling the second pattern")
			_, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: bar}})
			Expect(err).NotTo(HaveOccurred())
			By("Ensuring that the gitops subscription has not been changed")
			subs, err = reconciler.olmClient.OperatorsV1alpha1().Subscriptions("openshift-operators").Get(context.Background(), "openshift-gitops-operator", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(subs).NotTo(BeNil())
			Expect(subs.Spec.Channel).To(Equal("foo"))
		})
		It("when the subscription has been create before the pattern outside of the reconciliation loop", func() {
			gitopsSub := &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{Name: "openshift-gitops-operator", Namespace: "openshift-operators"},
				Spec:       &operatorv1alpha1.SubscriptionSpec{Channel: "stable"}}
			reconciler.olmClient = olmclient.NewSimpleClientset(gitopsSub)
			By("reconciling the first pattern")
			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(err).NotTo(HaveOccurred())
			By("ensuring that the gitops subscription has not been changed")
			subs, err := reconciler.olmClient.OperatorsV1alpha1().Subscriptions("openshift-operators").Get(context.Background(), "openshift-gitops-operator", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(subs).NotTo(BeNil())
			Expect(subs.Spec.Channel).To(Equal("stable"))
		})

		It("when there are updates to the pattern owner's gitopsconfig spec", func() {
			By("reconciling the first pattern when the gitops subscription does not exist")
			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(err).NotTo(HaveOccurred())
			err = reconciler.Client.Create(context.Background(), &pattern2, &client.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			subs, err := reconciler.olmClient.OperatorsV1alpha1().Subscriptions("openshift-operators").Get(context.Background(), "openshift-gitops-operator", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(subs).NotTo(BeNil())
			Expect(subs.Spec.Channel).To(Equal("foo"))
			By("making changes to the pattern's gitops config section")
			p := &api.Pattern{}
			err = reconciler.Client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: foo}, p)
			Expect(err).NotTo(HaveOccurred())
			p.Spec.GitOpsConfig.OperatorChannel = "new-channel"
			err = reconciler.Client.Update(context.Background(), p, &client.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(err).NotTo(HaveOccurred())
			By("ensuring that the gitops subscription has been updated")
			subs, err = reconciler.olmClient.OperatorsV1alpha1().Subscriptions("openshift-operators").Get(context.Background(), "openshift-gitops-operator", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(subs).NotTo(BeNil())
			Expect(subs.Spec.Channel).To(Equal("new-channel"))
		})
	})
})

func newFakeReconciler(initObjects ...runtime.Object) *PatternReconciler {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(initObjects...).Build()
	clusterVersion := &v1.ClusterVersion{ObjectMeta: metav1.ObjectMeta{Name: "version"}, Spec: v1.ClusterVersionSpec{ClusterID: "10"}}
	clusterInfra := &v1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}, Spec: v1.InfrastructureSpec{PlatformSpec: v1.PlatformSpec{Type: "AWS"}}}
	osControlManager := &operatorv1.OpenShiftControllerManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       operatorv1.OpenShiftControllerManagerSpec{},
		Status:     operatorv1.OpenShiftControllerManagerStatus{OperatorStatus: operatorv1.OperatorStatus{Version: "4.10.3"}}}
	ingress := &v1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}, Spec: v1.IngressSpec{Domain: "hello.world"}}
	watcher, _ := newDriftWatcher(fakeClient, logr.New(log.NullLogSink{}), newGitClient())
	return &PatternReconciler{
		Scheme:         scheme.Scheme,
		Client:         fakeClient,
		olmClient:      olmclient.NewSimpleClientset(),
		driftWatcher:   watcher,
		argoClient:     argocdclient.NewSimpleClientset(),
		configClient:   configclient.NewSimpleClientset(clusterVersion, clusterInfra, ingress),
		operatorClient: operatorclient.NewSimpleClientset(osControlManager).OperatorV1(),
	}
}

func buildPatternManifest(interval int) *api.Pattern {
	return &api.Pattern{ObjectMeta: metav1.ObjectMeta{
		Name:       foo,
		Namespace:  namespace,
		Finalizers: []string{api.PatternFinalizer},
	},
		Spec: api.PatternSpec{
			GitConfig: api.GitConfig{
				OriginRepo:   originURL,
				TargetRepo:   targetURL,
				PollInterval: interval,
			},
		},
	}
}
