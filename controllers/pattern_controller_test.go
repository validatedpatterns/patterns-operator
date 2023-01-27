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
	"time"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	"github.com/hybrid-cloud-patterns/patterns-operator/internal/gitea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/fake"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/fake"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"
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
	gitopsNamespace   = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-gitops"}}
)
var _ = Describe("pattern controller", func() {

	var _ = Context("validates the git drift watcher", func() {
		var (
			p          *api.Pattern
			reconciler *PatternReconciler
			watch      *watcher
		)
		BeforeEach(func() {
			nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			reconciler = newFakeReconciler(nsOperators, gitopsNamespace, buildPatternManifest(10, false))
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

	var _ = Context("validates the gitea server deployment and cloning", func() {
		var (
			reconciler                                     *PatternReconciler
			ctrlMock                                       *gomock.Controller
			giteaMock                                      *gitea.MockClientInterface
			hasDefaultUserBeenCalled, hasConnectBeenCalled bool
		)
		BeforeEach(func() {
			ctrlMock = gomock.NewController(GinkgoT())
			giteaMock = gitea.NewMockClientInterface(ctrlMock)

			operatorsNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			giteaNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name:        gitea.Namespace,
				Annotations: map[string]string{"openshift.io/sa.scc.supplemental-groups": "1001340000/10000", "openshift.io/sa.scc.uid-range": "1001140000/10000"}}}

			reconciler = newFakeReconciler(operatorsNamespace, gitopsNamespace, giteaNamespace, buildPatternManifest(10, true))
			reconciler.giteaClient = giteaMock
			reconciler.routeClient = routev1client.NewSimpleClientset()

		})
		It("captures the steps to deploy a gitea server and cloned repo when the pattern does not include a target repo", func() {
			giteaMock.EXPECT().Connect("https://", gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(url, username, password string) error {
				if !hasConnectBeenCalled {
					if username == gitea.AdminUser {
						hasConnectBeenCalled = true
						return nil
					}
				} else {
					if username == gitea.DefaultUser {
						return nil
					}
				}
				Fail(fmt.Sprintf("unexpected username %s", username))
				return nil
			})
			giteaMock.EXPECT().HasDefaultUser().AnyTimes().DoAndReturn(func() (bool, error) {
				if hasDefaultUserBeenCalled {
					return true, nil
				}
				hasDefaultUserBeenCalled = true
				return false, nil
			})
			giteaMock.EXPECT().CreateUser(gitea.DefaultUser, gomock.Any()).Times(1)
			giteaMock.EXPECT().GetClonedRepositoryURLFor(targetURL, string(plumbing.HEAD)).Times(1).Return("", nil)
			giteaMock.EXPECT().CloneRepository(targetURL, string(plumbing.HEAD), gitea.DefaultUser, gomock.Any()).Return("https://foo.bar", nil).Times(1)

			By("reconciling for the first time")
			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(err).To(HaveOccurred())

			By("ensuring it has created the expected resources")
			s := corev1.Secret{}
			err = reconciler.Client.Get(context.Background(), types.NamespacedName{Namespace: gitea.Namespace, Name: gitea.AdminSecretName}, &s)
			Expect(err).NotTo(HaveOccurred())
			Expect(s.Data).To(HaveLen(3))
			Expect(s.Data["username"]).To(Equal([]byte(gitea.AdminUser)))
			rt, err := reconciler.routeClient.RouteV1().Routes(gitea.Namespace).Get(context.Background(), gitea.RouteName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(rt).NotTo(BeNil())
			// update the application status to reflect it has completed deployment of the helm chart
			app, err := reconciler.argoClient.ArgoprojV1alpha1().Applications("openshift-gitops").Get(context.Background(), gitea.ApplicationName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("updating the application status to confirm it has completed its deployment")
			app.Status.Health.Status = health.HealthStatusHealthy
			_, err = reconciler.argoClient.ArgoprojV1alpha1().Applications("openshift-gitops").Update(context.Background(), app, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("resuming the reconciliation once the status of the gitea application is healthy")
			_, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: patternNamespaced})
			Expect(err).NotTo(HaveOccurred())

			By("validating that the targetURL and targetRepo have been updated")
			p := &api.Pattern{}
			err = reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.InClusterRepo).To(Equal("https://foo.bar"))
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

	gitopsSub := &operatorv1alpha1.Subscription{ObjectMeta: metav1.ObjectMeta{Name: "openshift-gitops-operator", Namespace: "openshift-operators"}}
	watcher, _ := newDriftWatcher(fakeClient, logr.New(log.NullLogSink{}), newGitClient())

	return &PatternReconciler{
		Scheme:         scheme.Scheme,
		Client:         fakeClient,
		olmClient:      olmclient.NewSimpleClientset(gitopsSub),
		driftWatcher:   watcher,
		argoClient:     argocdclient.NewSimpleClientset(),
		configClient:   configclient.NewSimpleClientset(clusterVersion, clusterInfra, ingress),
		operatorClient: operatorclient.NewSimpleClientset(osControlManager).OperatorV1(),
	}
}

func buildPatternManifest(interval int, inClusterFork bool) *api.Pattern {
	return &api.Pattern{ObjectMeta: metav1.ObjectMeta{
		Name:       foo,
		Namespace:  namespace,
		Finalizers: []string{api.PatternFinalizer},
	},
		Spec: api.PatternSpec{
			GitConfig: api.GitConfig{
				OriginRepo:       originURL,
				TargetRepo:       targetURL,
				UseInClusterFork: inClusterFork,
				PollInterval:     interval,
			},
		},
	}
}
