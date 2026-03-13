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
	defaultNamespace = "default"
	foo              = "foo"
	originURL        = "https://origin.url"
	targetURL        = "https://target.url"
)

var (
	namespace         = suggestedOperatorNamespace
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

var _ = Describe("pattern controller - preValidation", func() {
	var reconciler *PatternReconciler

	BeforeEach(func() {
		nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		reconciler = newFakeReconciler(nsOperators, buildPatternManifest())
	})

	It("should pass with valid https target and origin repos", func() {
		p := &api.Pattern{
			Spec: api.PatternSpec{
				GitConfig: api.GitConfig{
					TargetRepo: "https://github.com/test/repo",
					OriginRepo: "https://github.com/upstream/repo",
				},
			},
		}
		err := reconciler.preValidation(p)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should pass with valid ssh target repo", func() {
		p := &api.Pattern{
			Spec: api.PatternSpec{
				GitConfig: api.GitConfig{
					TargetRepo: "git@github.com:test/repo.git",
				},
			},
		}
		err := reconciler.preValidation(p)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fail when target repo is empty", func() {
		p := &api.Pattern{
			Spec: api.PatternSpec{
				GitConfig: api.GitConfig{
					TargetRepo: "",
				},
			},
		}
		err := reconciler.preValidation(p)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TargetRepo cannot be empty"))
	})

	It("should fail with invalid origin repo URL", func() {
		p := &api.Pattern{
			Spec: api.PatternSpec{
				GitConfig: api.GitConfig{
					OriginRepo: "invalid-url",
					TargetRepo: "https://github.com/test/repo",
				},
			},
		}
		err := reconciler.preValidation(p)
		Expect(err).To(HaveOccurred())
	})

	It("should fail with invalid target repo URL", func() {
		p := &api.Pattern{
			Spec: api.PatternSpec{
				GitConfig: api.GitConfig{
					TargetRepo: "invalid-url",
				},
			},
		}
		err := reconciler.preValidation(p)
		Expect(err).To(HaveOccurred())
	})

	It("should pass when origin repo is empty but target repo is valid", func() {
		p := &api.Pattern{
			Spec: api.PatternSpec{
				GitConfig: api.GitConfig{
					OriginRepo: "",
					TargetRepo: "https://github.com/test/repo",
				},
			},
		}
		err := reconciler.preValidation(p)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("pattern controller - applyDefaults", func() {
	var reconciler *PatternReconciler

	BeforeEach(func() {
		nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		reconciler = newFakeReconciler(nsOperators, buildPatternManifest())
	})

	It("should set cluster info from configClient", func() {
		p := buildPatternManifest()
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Status.ClusterPlatform).To(Equal("AWS"))
		Expect(output.Status.ClusterVersion).To(Equal("4.10"))
	})

	It("should set the cluster domain from ingress", func() {
		p := buildPatternManifest()
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Status.AppClusterDomain).To(Equal("hello.world"))
	})

	It("should default TargetRevision to HEAD when empty", func() {
		p := buildPatternManifest()
		p.Spec.GitConfig.TargetRevision = ""
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.GitConfig.TargetRevision).To(Equal(GitHEAD))
	})

	It("should preserve TargetRevision when set", func() {
		p := buildPatternManifest()
		p.Spec.GitConfig.TargetRevision = "v1.0.0"
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.GitConfig.TargetRevision).To(Equal("v1.0.0"))
	})

	It("should default OriginRevision to HEAD when empty", func() {
		p := buildPatternManifest()
		p.Spec.GitConfig.OriginRevision = ""
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.GitConfig.OriginRevision).To(Equal(GitHEAD))
	})

	It("should default MultiSourceConfig.Enabled to true when nil", func() {
		p := buildPatternManifest()
		p.Spec.MultiSourceConfig.Enabled = nil
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.MultiSourceConfig.Enabled).ToNot(BeNil())
		Expect(*output.Spec.MultiSourceConfig.Enabled).To(BeTrue())
	})

	It("should default ClusterGroupName to 'default' when empty", func() {
		p := buildPatternManifest()
		p.Spec.ClusterGroupName = ""
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.ClusterGroupName).To(Equal("default"))
	})

	It("should default HelmRepoUrl when empty", func() {
		p := buildPatternManifest()
		p.Spec.MultiSourceConfig.HelmRepoUrl = ""
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.MultiSourceConfig.HelmRepoUrl).To(Equal("https://charts.validatedpatterns.io/"))
	})

	It("should initialize GitOpsConfig when nil", func() {
		p := buildPatternManifest()
		p.Spec.GitOpsConfig = nil
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.GitOpsConfig).ToNot(BeNil())
	})

	It("should extract hostname from TargetRepo when Hostname is empty", func() {
		p := buildPatternManifest()
		p.Spec.GitConfig.Hostname = ""
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.GitConfig.Hostname).To(Equal("target.url"))
	})

	It("should preserve hostname when already set", func() {
		p := buildPatternManifest()
		p.Spec.GitConfig.Hostname = "custom.hostname.io"
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.GitConfig.Hostname).To(Equal("custom.hostname.io"))
	})

	It("should set LocalCheckoutPath", func() {
		p := buildPatternManifest()
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Status.LocalCheckoutPath).ToNot(BeEmpty())
	})
})

var _ = Describe("pattern controller - postValidation", func() {
	It("should always return nil", func() {
		nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		reconciler := newFakeReconciler(nsOperators, buildPatternManifest())
		p := buildPatternManifest()
		err := reconciler.postValidation(p)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("pattern controller - buildPatternManifest helpers", func() {
	It("should create a pattern with the correct name", func() {
		p := buildPatternManifest()
		Expect(p.Name).To(Equal(foo))
	})

	It("should create a pattern with the correct namespace", func() {
		p := buildPatternManifest()
		Expect(p.Namespace).To(Equal(namespace))
	})

	It("should include the pattern finalizer", func() {
		p := buildPatternManifest()
		Expect(p.Finalizers).To(ContainElement(api.PatternFinalizer))
	})

	It("should set the git config", func() {
		p := buildPatternManifest()
		Expect(p.Spec.GitConfig.OriginRepo).To(Equal(originURL))
		Expect(p.Spec.GitConfig.TargetRepo).To(Equal(targetURL))
	})

	It("should set the cluster platform status", func() {
		p := buildPatternManifest()
		Expect(p.Status.ClusterPlatform).To(Equal("AWS"))
		Expect(p.Status.ClusterVersion).To(Equal("1.2.3"))
	})
})

var _ = Describe("pattern controller - reconciler creation", func() {
	It("should create a reconciler with all required clients", func() {
		nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		reconciler := newFakeReconciler(nsOperators, buildPatternManifest())
		Expect(reconciler).ToNot(BeNil())
		Expect(reconciler.Client).ToNot(BeNil())
		Expect(reconciler.Scheme).ToNot(BeNil())
		Expect(reconciler.olmClient).ToNot(BeNil())
		Expect(reconciler.fullClient).ToNot(BeNil())
		Expect(reconciler.configClient).ToNot(BeNil())
		Expect(reconciler.operatorClient).ToNot(BeNil())
		Expect(reconciler.AnalyticsClient).ToNot(BeNil())
		Expect(reconciler.gitOperations).ToNot(BeNil())
	})
})

var _ = Describe("pattern controller - fetching pattern", func() {
	It("should be able to get the pattern after creation", func() {
		nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		reconciler := newFakeReconciler(nsOperators, buildPatternManifest())
		p := &api.Pattern{}
		err := reconciler.Client.Get(context.Background(), patternNamespaced, p)
		Expect(err).ToNot(HaveOccurred())
		Expect(p.Name).To(Equal(foo))
		Expect(p.Namespace).To(Equal(namespace))
	})

	It("should return error for nonexistent pattern", func() {
		nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		reconciler := newFakeReconciler(nsOperators, buildPatternManifest())
		p := &api.Pattern{}
		err := reconciler.Client.Get(context.Background(),
			types.NamespacedName{Name: "nonexistent", Namespace: namespace}, p)
		Expect(err).To(HaveOccurred())
	})
})
