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
	"encoding/json"
	"fmt"
	"os"
	"time"

	//argoclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/go-logr/logr"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/fake"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	namespace       = "openshift-operators"
	defaultInterval = time.Duration(180) * time.Second
)

var (
	patternNamespaced = types.NamespacedName{Name: foo, Namespace: namespace}
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
		It("adding a pattern with application status", func() {
			p = &api.Pattern{}
			err := reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.Applications).To(HaveLen(0))
			p.Status.Applications = buildTestApplicationInfoArray()
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())
			err = reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.Applications).To(HaveLen(2))
		})
	})

	var _ = Context("deploy gitops subscription", func() {
		var (
			reconciler *PatternReconciler
		)
		When("no subscription exists", func() {

			BeforeEach(func() {
				nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
				reconciler = newFakeReconciler(nsOperators, buildPatternManifest(10))
			})

			It("deploys the default subscription when no custom configuration is found", func() {

				err := reconciler.deployGitopsSubscription("/file-does-not-exist")
				Expect(err).NotTo(HaveOccurred())
				s, err := reconciler.olmClient.client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Get(context.TODO(), gitopsSubscriptionName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(s.Spec).To(BeEquivalentTo(newSubscription(&v1alpha1.SubscriptionSpec{}).Spec))
			})

			It("deploys a custom subscription when a custom configuration is found in the filesystem", func() {
				f, err := os.CreateTemp("", "gitops-config.yaml.*")
				Expect(err).NotTo(HaveOccurred())
				defer os.Remove(f.Name())

				customSpec := &v1alpha1.SubscriptionSpec{
					CatalogSource:       "my-source",
					Channel:             "my-channel",
					StartingCSV:         "starting-csv",
					InstallPlanApproval: v1alpha1.ApprovalManual,
				}
				b, err := json.Marshal(customSpec)
				Expect(err).NotTo(HaveOccurred())
				_, err = f.Write(b)
				Expect(err).NotTo(HaveOccurred())
				err = f.Close()
				Expect(err).NotTo(HaveOccurred())
				err = reconciler.deployGitopsSubscription(f.Name())
				Expect(err).NotTo(HaveOccurred())
				s, err := reconciler.olmClient.client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Get(context.TODO(), gitopsSubscriptionName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(s.Spec).To(BeEquivalentTo(newSubscription(customSpec).Spec))
			})

		})
		When("a subscription already exists", func() {
			var (
				customSpec *v1alpha1.SubscriptionSpec
			)
			BeforeEach(func() {
				nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
				reconciler = newFakeReconciler(nsOperators, buildPatternManifest(10))
				customSpec = &v1alpha1.SubscriptionSpec{
					CatalogSource:       "my-source",
					Channel:             "my-channel",
					StartingCSV:         "starting-csv",
					InstallPlanApproval: v1alpha1.ApprovalManual,
				}
			})
			It("reconciles a subscription with custom values to default configuration when no custom configuration is found", func() {
				reconciler.olmClient = newOLMClient(olmclient.NewSimpleClientset(newSubscription(customSpec)))

				err := reconciler.deployGitopsSubscription("/file-does-not-exist")
				Expect(err).NotTo(HaveOccurred())
				s, err := reconciler.olmClient.client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Get(context.TODO(), gitopsSubscriptionName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(s.Spec).To(BeEquivalentTo(newSubscription(&v1alpha1.SubscriptionSpec{}).Spec))
			})

			It("reconciles a subscription with default values when a custom configuration is found in the filesystem", func() {
				reconciler.olmClient = newOLMClient(olmclient.NewSimpleClientset(newSubscription(&v1alpha1.SubscriptionSpec{})))
				f, err := os.CreateTemp("", "gitops-config.yaml.*")
				Expect(err).NotTo(HaveOccurred())
				defer os.Remove(f.Name())

				b, err := json.Marshal(customSpec)
				Expect(err).NotTo(HaveOccurred())
				_, err = f.Write(b)
				Expect(err).NotTo(HaveOccurred())
				err = f.Close()
				Expect(err).NotTo(HaveOccurred())
				err = reconciler.deployGitopsSubscription(f.Name())
				Expect(err).NotTo(HaveOccurred())
				s, err := reconciler.olmClient.client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Get(context.TODO(), gitopsSubscriptionName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(s.Spec).To(BeEquivalentTo(newSubscription(customSpec).Spec))
			})

			It("validates that the reconciliation will wait until the gitops installplan has completed", func() {
				reconciler.olmClient = newOLMClient(olmclient.NewSimpleClientset(newSubscription(&v1alpha1.SubscriptionSpec{})))
				By("reconciling the first pattern when the gitops subscription does not exist")
				_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
				Expect(err).To(HaveOccurred())
				subs, err := reconciler.olmClient.getSubscription(gitopsSubscriptionName, subscriptionNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(subs).NotTo(BeNil())
				subs.Status.InstallPlanRef = &corev1.ObjectReference{
					Kind:       "InstallPlan",
					APIVersion: "operators.coreos.com/v1alpha1",
					Namespace:  subscriptionNamespace,
					Name:       "my-installplan",
				}
				By("updating the subscription to include the reference to the installplan")
				_, err = reconciler.olmClient.client.OperatorsV1alpha1().Subscriptions(subscriptionNamespace).Update(context.Background(), subs, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())
				_, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				By("creating the installplan instance with empty phase")
				ip := &v1alpha1.InstallPlan{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-installplan",
						Namespace: subscriptionNamespace,
					},
					Status: v1alpha1.InstallPlanStatus{Phase: v1alpha1.InstallPlanPhaseInstalling},
				}
				_, err = reconciler.olmClient.client.OperatorsV1alpha1().InstallPlans(subscriptionNamespace).Create(context.Background(), ip, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				_, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
				Expect(err).To(Equal(fmt.Errorf("gitops subscription deployment is not yet complete")))

				By("updating the installplan phase to be completed")
				ip.Status.Phase = v1alpha1.InstallPlanPhaseComplete
				_, err = reconciler.olmClient.client.OperatorsV1alpha1().InstallPlans(subscriptionNamespace).Update(context.Background(), ip, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("validating that the reconciliation completes successfully after the gitops operator has completed its deployment")
				_, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

func newFakeReconciler(initObjects ...runtime.Object) *PatternReconciler {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(initObjects...).Build()
	clusterVersion := &v1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Spec:       v1.ClusterVersionSpec{ClusterID: "10"},
		Status: v1.ClusterVersionStatus{
			History: []v1.UpdateHistory{
				{
					State:   "Completed",
					Version: "4.10.3",
				},
			},
		},
	}
	clusterInfra := &v1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}, Spec: v1.InfrastructureSpec{PlatformSpec: v1.PlatformSpec{Type: "AWS"}}}
	osControlManager := &operatorv1.OpenShiftControllerManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       operatorv1.OpenShiftControllerManagerSpec{},
		Status:     operatorv1.OpenShiftControllerManagerStatus{OperatorStatus: operatorv1.OperatorStatus{Version: "4.10.3"}}}
	ingress := &v1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}, Spec: v1.IngressSpec{Domain: "hello.world"}}
	watcher, _ := newDriftWatcher(fakeClient, logr.New(log.NullLogSink{}), newGitClient())
	return &PatternReconciler{
		Scheme:       scheme.Scheme,
		Client:       fakeClient,
		olmClient:    newOLMClient(olmclient.NewSimpleClientset()),
		driftWatcher: watcher,
		//	argoClient:      argoclient.NewSimpleClientset(),
		configClient:    configclient.NewSimpleClientset(clusterVersion, clusterInfra, ingress),
		operatorClient:  operatorclient.NewSimpleClientset(osControlManager).OperatorV1(),
		AnalyticsClient: AnalyticsInit(true, logr.New(log.NullLogSink{})),
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
		Status: api.PatternStatus{
			ClusterPlatform: "AWS",
			ClusterVersion:  "1.2.3",
		},
	}
}

func buildTestApplicationInfoArray() []api.PatternApplicationInfo {
	applications := []api.PatternApplicationInfo{
		{
			Name:            "hello-world",
			Namespace:       "pattern-namespace",
			AppSyncStatus:   "Synced",
			AppHealthStatus: "Healthy",
		},
		{
			Name:            "foo",
			Namespace:       "pattern-namespace",
			AppSyncStatus:   "Degraded",
			AppHealthStatus: "Synced",
		},
	}

	return applications
}

var _ = Describe("ExtractRepositoryName", func() {
	It("should extract the repository name from various URL formats", func() {
		testCases := []struct {
			inputURL     string
			expectedName string
		}{
			{"https://github.com/username/repo.git", "repo"},
			{"https://github.com/username/repo", "repo"},
			{"https://github.com/username/repo.git/", "repo"},
			{"https://github.com/username/repo/", "repo"},
			{"https://gitlab.com/username/my-project.git", "my-project"},
			{"https://gitlab.com/username/my-project", "my-project"},
			{"https://bitbucket.org/username/myrepo.git", "myrepo"},
			{"https://bitbucket.org/username/myrepo", "myrepo"},
			{"https://example.com/username/repo.git", "repo"},
			{"https://example.com/username/repo", "repo"},
			{"https://example.com/username/repo.git/", "repo"},
			{"https://example.com/username/repo/", "repo"},
		}

		for _, testCase := range testCases {
			repoName, err := extractRepositoryName(testCase.inputURL)
			Expect(err).To(BeNil())
			Expect(repoName).To(Equal(testCase.expectedName))
		}
	})

	It("should return an error for an invalid URL", func() {
		invalidURL := "invalid-url"
		_, err := extractRepositoryName(invalidURL)
		Expect(err).NotTo(BeNil())
	})
})
