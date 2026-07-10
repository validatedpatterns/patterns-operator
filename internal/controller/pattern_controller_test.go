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
	"net/http"
	"net/http/httptest"
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

	Context("values file validation", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "pattern-test-")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			if tempDir != "" {
				os.RemoveAll(tempDir)
			}
		})

		It("should pass when cluster group values file exists", func() {
			// Create the required values file
			valuesFile := tempDir + "/values-test-cluster.yaml"
			err := os.WriteFile(valuesFile, []byte("# Test values file\nglobal:\n  key: value\n"), 0600)
			Expect(err).ToNot(HaveOccurred())

			p := &api.Pattern{
				Spec: api.PatternSpec{
					ClusterGroupName: "test-cluster",
					GitConfig: api.GitConfig{
						TargetRepo: "https://github.com/test/repo",
					},
				},
				Status: api.PatternStatus{
					LocalCheckoutPath: tempDir,
				},
			}
			err = reconciler.preValidation(p)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail when cluster group values file is missing", func() {
			p := &api.Pattern{
				Spec: api.PatternSpec{
					ClusterGroupName: "missing-cluster",
					GitConfig: api.GitConfig{
						TargetRepo: "https://github.com/test/repo",
					},
				},
				Status: api.PatternStatus{
					LocalCheckoutPath: tempDir,
				},
			}
			err := reconciler.preValidation(p)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("required values file not found: values-missing-cluster.yaml"))
		})

		It("should pass when cluster group is empty (no validation needed)", func() {
			p := &api.Pattern{
				Spec: api.PatternSpec{
					ClusterGroupName: "",
					GitConfig: api.GitConfig{
						TargetRepo: "https://github.com/test/repo",
					},
				},
				Status: api.PatternStatus{
					LocalCheckoutPath: tempDir,
				},
			}
			err := reconciler.preValidation(p)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should pass when local checkout path is empty (validation skipped)", func() {
			p := &api.Pattern{
				Spec: api.PatternSpec{
					ClusterGroupName: "test-cluster",
					GitConfig: api.GitConfig{
						TargetRepo: "https://github.com/test/repo",
					},
				},
				Status: api.PatternStatus{
					LocalCheckoutPath: "",
				},
			}
			err := reconciler.preValidation(p)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("pattern controller - condition utilities", func() {
	var pattern *api.Pattern

	BeforeEach(func() {
		pattern = &api.Pattern{
			Status: api.PatternStatus{
				Conditions: []api.PatternCondition{},
			},
		}
	})

	Context("setPatternCondition", func() {
		It("should add a new condition when none exists", func() {
			setPatternCondition(pattern, api.Missing, corev1.ConditionTrue, "values file missing")

			Expect(pattern.Status.Conditions).To(HaveLen(1))
			condition := pattern.Status.Conditions[0]
			Expect(condition.Type).To(Equal(api.Missing))
			Expect(condition.Status).To(Equal(corev1.ConditionTrue))
			Expect(condition.Message).To(Equal("values file missing"))
			Expect(condition.LastUpdateTime).ToNot(BeZero())
			Expect(condition.LastTransitionTime).ToNot(BeZero())
		})

		It("should update an existing condition with new status", func() {
			// Add initial condition
			setPatternCondition(pattern, api.Missing, corev1.ConditionFalse, "initial message")
			initialTransitionTime := pattern.Status.Conditions[0].LastTransitionTime

			// Update with new status
			setPatternCondition(pattern, api.Missing, corev1.ConditionTrue, "updated message")

			Expect(pattern.Status.Conditions).To(HaveLen(1))
			condition := pattern.Status.Conditions[0]
			Expect(condition.Type).To(Equal(api.Missing))
			Expect(condition.Status).To(Equal(corev1.ConditionTrue))
			Expect(condition.Message).To(Equal("updated message"))
			Expect(condition.LastTransitionTime).ToNot(Equal(initialTransitionTime))
		})

		It("should preserve transition time when status doesn't change", func() {
			// Add initial condition
			setPatternCondition(pattern, api.Missing, corev1.ConditionTrue, "initial message")
			initialTransitionTime := pattern.Status.Conditions[0].LastTransitionTime

			// Update with same status but different message
			setPatternCondition(pattern, api.Missing, corev1.ConditionTrue, "updated message")

			Expect(pattern.Status.Conditions).To(HaveLen(1))
			condition := pattern.Status.Conditions[0]
			Expect(condition.Status).To(Equal(corev1.ConditionTrue))
			Expect(condition.Message).To(Equal("updated message"))
			Expect(condition.LastTransitionTime).To(Equal(initialTransitionTime))
		})
	})

	Context("removePatternCondition", func() {
		It("should remove an existing condition", func() {
			// Add multiple conditions
			setPatternCondition(pattern, api.Missing, corev1.ConditionTrue, "missing message")
			setPatternCondition(pattern, api.Progressing, corev1.ConditionTrue, "progressing message")
			Expect(pattern.Status.Conditions).To(HaveLen(2))

			// Remove one condition
			removePatternCondition(pattern, api.Missing)

			Expect(pattern.Status.Conditions).To(HaveLen(1))
			Expect(pattern.Status.Conditions[0].Type).To(Equal(api.Progressing))
		})

		It("should handle removing non-existent condition gracefully", func() {
			setPatternCondition(pattern, api.Progressing, corev1.ConditionTrue, "progressing message")

			removePatternCondition(pattern, api.Missing) // This doesn't exist

			Expect(pattern.Status.Conditions).To(HaveLen(1))
			Expect(pattern.Status.Conditions[0].Type).To(Equal(api.Progressing))
		})
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

	It("should sync Variant from ClusterGroupName when Variant is empty", func() {
		p := buildPatternManifest()
		p.Spec.ClusterGroupName = "hub"
		p.Spec.Variant = ""
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.ClusterGroupName).To(Equal("hub"))
		Expect(output.Spec.Variant).To(Equal("hub"))
	})

	It("should sync ClusterGroupName from Variant when ClusterGroupName is empty", func() {
		p := buildPatternManifest()
		p.Spec.ClusterGroupName = ""
		p.Spec.Variant = "factory"
		output, err := reconciler.applyDefaults(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(output.Spec.ClusterGroupName).To(Equal("factory"))
		Expect(output.Spec.Variant).To(Equal("factory"))
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

var _ = Describe("pattern controller - checkSpokeApplicationsGone", func() {
	var (
		reconciler  *PatternReconciler
		server      *httptest.Server
		originalURL string
		originalTok string
		hasURL      bool
		hasTok      bool
	)

	BeforeEach(func() {
		nsOperators := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		reconciler = newFakeReconciler(nsOperators, buildPatternManifest())
		originalURL, hasURL = os.LookupEnv("ACM_SEARCH_API_URL")
		originalTok, hasTok = os.LookupEnv("ACM_SEARCH_API_TOKEN")
		os.Setenv("ACM_SEARCH_API_TOKEN", "test-token")
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
		if hasURL {
			os.Setenv("ACM_SEARCH_API_URL", originalURL)
		} else {
			os.Unsetenv("ACM_SEARCH_API_URL")
		}
		if hasTok {
			os.Setenv("ACM_SEARCH_API_TOKEN", originalTok)
		} else {
			os.Unsetenv("ACM_SEARCH_API_TOKEN")
		}
	})

	startServer := func(handler http.HandlerFunc) {
		server = httptest.NewServer(handler)
		os.Setenv("ACM_SEARCH_API_URL", server.URL+"/searchapi/graphql")
	}

	emptySearchResponse := func() []byte {
		resp := map[string]any{
			"data": map[string]any{
				"searchResult": []any{
					map[string]any{
						"items": []any{},
					},
				},
			},
		}
		b, _ := json.Marshal(resp)
		return b
	}

	appsSearchResponse := func(apps []map[string]string) []byte {
		items := make([]any, 0, len(apps))
		for _, app := range apps {
			items = append(items, app)
		}
		resp := map[string]any{
			"data": map[string]any{
				"searchResult": []any{
					map[string]any{
						"items": items,
					},
				},
			},
		}
		b, _ := json.Marshal(resp)
		return b
	}

	Context("when all spoke applications are gone", func() {
		It("should return true with empty search results", func() {
			startServer(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(emptySearchResponse())
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).ToNot(HaveOccurred())
			Expect(gone).To(BeTrue())
		})

		It("should return true when searchResult has no items array", func() {
			startServer(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]any{
					"data": map[string]any{
						"searchResult": []any{},
					},
				}
				b, _ := json.Marshal(resp)
				_, _ = w.Write(b)
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).ToNot(HaveOccurred())
			Expect(gone).To(BeTrue())
		})
	})

	Context("when spoke applications still exist", func() {
		It("should return false with app names in error", func() {
			startServer(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(appsSearchResponse([]map[string]string{
					{"name": "myapp", "namespace": "myns", "cluster": "spoke1"},
				}))
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spoke cluster apps still exist"))
			Expect(err.Error()).To(ContainSubstring("myns/myapp in spoke1"))
			Expect(gone).To(BeFalse())
		})

		It("should list multiple remaining applications", func() {
			startServer(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(appsSearchResponse([]map[string]string{
					{"name": "app1", "namespace": "ns1", "cluster": "spoke1"},
					{"name": "app2", "namespace": "ns2", "cluster": "spoke2"},
				}))
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ns1/app1 in spoke1"))
			Expect(err.Error()).To(ContainSubstring("ns2/app2 in spoke2"))
			Expect(gone).To(BeFalse())
		})
	})

	Context("appOfApps parameter", func() {
		It("should filter for child apps when appOfApps is false", func() {
			var receivedBody map[string]any
			startServer(func(w http.ResponseWriter, r *http.Request) {
				Expect(json.NewDecoder(r.Body).Decode(&receivedBody)).To(Succeed())
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(emptySearchResponse())
			})

			_, _ = reconciler.checkSpokeApplicationsGone(false)

			variables := receivedBody["variables"].(map[string]any)
			input := variables["input"].([]any)[0].(map[string]any)
			filters := input["filters"].([]any)
			var nsFilter map[string]any
			for _, f := range filters {
				fm := f.(map[string]any)
				if fm["property"] == "namespace" {
					nsFilter = fm
					break
				}
			}
			Expect(nsFilter).ToNot(BeNil())
			nsValues := nsFilter["values"].([]any)
			Expect(nsValues).To(HaveLen(1))
			Expect(nsValues[0]).To(Equal(fmt.Sprintf("!%s", getClusterWideArgoNamespace())))
		})

		It("should filter for app-of-apps when appOfApps is true", func() {
			var receivedBody map[string]any
			startServer(func(w http.ResponseWriter, r *http.Request) {
				Expect(json.NewDecoder(r.Body).Decode(&receivedBody)).To(Succeed())
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(emptySearchResponse())
			})

			_, _ = reconciler.checkSpokeApplicationsGone(true)

			variables := receivedBody["variables"].(map[string]any)
			input := variables["input"].([]any)[0].(map[string]any)
			filters := input["filters"].([]any)
			var nsFilter map[string]any
			for _, f := range filters {
				fm := f.(map[string]any)
				if fm["property"] == "namespace" {
					nsFilter = fm
					break
				}
			}
			Expect(nsFilter).ToNot(BeNil())
			nsValues := nsFilter["values"].([]any)
			Expect(nsValues).To(HaveLen(1))
			Expect(nsValues[0]).To(Equal(getClusterWideArgoNamespace()))
		})
	})

	Context("request validation", func() {
		It("should send correct headers", func() {
			var receivedHeaders http.Header
			startServer(func(w http.ResponseWriter, r *http.Request) {
				receivedHeaders = r.Header
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(emptySearchResponse())
			})

			_, _ = reconciler.checkSpokeApplicationsGone(false)

			Expect(receivedHeaders.Get("Authorization")).To(Equal("Bearer test-token"))
			Expect(receivedHeaders.Get("Content-Type")).To(Equal("application/json"))
			Expect(receivedHeaders.Get("Accept")).To(Equal("application/json"))
		})

		It("should send a POST request", func() {
			var receivedMethod string
			startServer(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(emptySearchResponse())
			})

			_, _ = reconciler.checkSpokeApplicationsGone(false)

			Expect(receivedMethod).To(Equal("POST"))
		})
	})

	Context("error handling", func() {
		It("should return error for invalid URL scheme", func() {
			os.Setenv("ACM_SEARCH_API_URL", "ftp://invalid-scheme/api")

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid search API URL"))
			Expect(gone).To(BeFalse())
		})

		It("should return error when server returns non-200 status", func() {
			startServer(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("search service returned status 500"))
			Expect(gone).To(BeFalse())
		})

		It("should return error when server returns 403", func() {
			startServer(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("forbidden"))
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("search service returned status 403"))
			Expect(gone).To(BeFalse())
		})

		It("should return error for malformed JSON response", func() {
			startServer(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("not valid json"))
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse JSON response"))
			Expect(gone).To(BeFalse())
		})

		It("should return error when token env var is empty and token file is missing", func() {
			os.Unsetenv("ACM_SEARCH_API_TOKEN")
			startServer(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(emptySearchResponse())
			})

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read serviceaccount token"))
			Expect(gone).To(BeFalse())
		})

		It("should return error when server is unreachable", func() {
			os.Setenv("ACM_SEARCH_API_URL", "http://127.0.0.1:1/searchapi/graphql")

			gone, err := reconciler.checkSpokeApplicationsGone(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to make HTTP request to search service"))
			Expect(gone).To(BeFalse())
		})
	})
})
