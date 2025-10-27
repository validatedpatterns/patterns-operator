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
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-logr/logr"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/fake"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	gomock "go.uber.org/mock/gomock"

	kubeclient "k8s.io/client-go/kubernetes/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	namespace        = "openshift-operators"
	defaultNamespace = "default"
	foo              = "foo"
	originURL        = "https://origin.url"
	targetURL        = "https://target.url"
)

var (
	patternNamespaced = types.NamespacedName{Name: foo, Namespace: namespace}
	mockGitOps        *MockGitOperations
	gitOptions        *git.CloneOptions
)
var _ = Describe("pattern controller", func() {

	var _ = Context("reconciliation", func() {
		var (
			p          *api.Pattern
			reconciler *PatternReconciler
		)
		BeforeEach(func() {
			nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			reconciler = newFakeReconciler(nsOperators, buildPatternManifest())
			gitOptions = &git.CloneOptions{
				URL:          "https://target.url",
				Progress:     os.Stdout,
				Depth:        0,
				RemoteName:   "origin",
				SingleBranch: false,
				Tags:         git.AllTags,
			}
		})

		It("adding a pattern with application status", func() {
			p = &api.Pattern{}
			err := reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.Applications).To(BeEmpty())
			p.Status.Applications = buildTestApplicationInfoArray()
			err = reconciler.Client.Update(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())
			err = reconciler.Client.Get(context.Background(), patternNamespaced, p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.Applications).To(HaveLen(2))
		})
	})
})

func newFakeReconciler(initObjects ...runtime.Object) *PatternReconciler {
	mockctrl := gomock.NewController(GinkgoT())
	defer mockctrl.Finish()
	mockGitOps = NewMockGitOperations(mockctrl)

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
	return &PatternReconciler{
		Scheme:          scheme.Scheme,
		Client:          fakeClient,
		olmClient:       olmclient.NewSimpleClientset(),
		fullClient:      kubeclient.NewSimpleClientset(),
		configClient:    configclient.NewSimpleClientset(clusterVersion, clusterInfra, ingress),
		operatorClient:  operatorclient.NewSimpleClientset(osControlManager).OperatorV1(),
		AnalyticsClient: AnalyticsInit(true, logr.New(log.NullLogSink{})),
		gitOperations:   mockGitOps,
	}
}

func buildPatternManifest() *api.Pattern {
	return &api.Pattern{ObjectMeta: metav1.ObjectMeta{
		Name:       foo,
		Namespace:  namespace,
		Finalizers: []string{api.PatternFinalizer},
	},
		Spec: api.PatternSpec{
			GitConfig: api.GitConfig{
				OriginRepo: originURL,
				TargetRepo: targetURL,
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
