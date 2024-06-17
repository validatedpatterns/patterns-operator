package controllers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	argooperator "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	routev1 "github.com/openshift/api/route/v1"

	argoapi "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func prefixArray(a []string, prefix string) []string {
	b := []string{}
	for _, i := range a {
		b = append(b, fmt.Sprintf("%s%s", prefix, i))
	}
	return b
}

var _ = Describe("Argo Pattern", func() {
	var pattern *api.Pattern
	var defaultValueFiles []string
	var argoApp, multiSourceArgoApp *argoapi.Application
	var appSource *argoapi.ApplicationSource
	BeforeEach(func() {
		tmpFalse := false
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "multicloud-gitops-test", Namespace: defaultNamespace},
			TypeMeta:   metav1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
			Spec: api.PatternSpec{
				ClusterGroupName: "foogroup",
				GitConfig: api.GitConfig{
					TargetRepo:     "https://github.com/validatedpatterns/multicloud-gitops",
					TargetRevision: "main",
				},
				MultiSourceConfig: api.MultiSourceConfig{
					Enabled:                  &tmpFalse,
					HelmRepoUrl:              "https://charts.validatedpatterns.io/",
					ClusterGroupChartVersion: "0.0.*",
				},
				GitOpsConfig: &api.GitOpsConfig{
					ManualSync: false,
				},
			},
			Status: api.PatternStatus{
				ClusterPlatform:  "AWS",
				ClusterVersion:   "4.12",
				ClusterName:      "barcluster",
				AppClusterDomain: "apps.hub-cluster.validatedpatterns.io",
				ClusterDomain:    "hub-cluster.validatedpatterns.io",
			},
		}
		defaultValueFiles = []string{
			"/values-global.yaml",
			"/values-foogroup.yaml",
			"/values-AWS.yaml",
			"/values-AWS-4.12.yaml",
			"/values-AWS-foogroup.yaml",
			"/values-4.12-foogroup.yaml",
			"/values-barcluster.yaml",
		}

		appSource = &argoapi.ApplicationSource{
			RepoURL:        pattern.Spec.GitConfig.TargetRepo,
			Path:           "common/clustergroup",
			TargetRevision: pattern.Spec.GitConfig.TargetRevision,
			Helm: &argoapi.ApplicationSourceHelm{
				ValueFiles:              newApplicationValueFiles(pattern, ""),
				Parameters:              newApplicationParameters(pattern),
				Values:                  newApplicationValues(pattern),
				IgnoreMissingValueFiles: true,
			},
		}
		argoApp = &argoapi.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      applicationName(pattern),
				Namespace: "openshift-gitops",
				Labels: map[string]string{
					"validatedpatterns.io/pattern": pattern.Name,
				},
			},
			Spec: argoapi.ApplicationSpec{
				Source: appSource,
				Destination: argoapi.ApplicationDestination{
					Name:      "in-cluster",
					Namespace: pattern.Namespace,
				},
				Project: "default",
				SyncPolicy: &argoapi.SyncPolicy{
					Automated:   &argoapi.SyncPolicyAutomated{},
					SyncOptions: []string{},
				},
			},
		}
		controllerutil.AddFinalizer(argoApp, argoapi.ForegroundPropagationPolicyFinalizer)
	})

	Describe("Testing applicationName function", func() {
		Context("Default", func() {
			It("Returns default application name", func() {
				Expect(applicationName(pattern)).To(Equal("multicloud-gitops-test-foogroup"))
			})
		})
	})

	Describe("Testing newApplication function", func() {
		Context("Default single source", func() {
			It("Returns an argo application", func() {
				// This is needed to debug any failures as gomega truncates the diff output
				format.MaxDepth = 100
				format.MaxLength = 0
				Expect(newArgoApplication(pattern)).To(Equal(argoApp))
			})
		})
		Context("Default multi source", func() {
			It("Returns an argo application with multiple sources", func() {
				// This is needed to debug any failures as gomega truncates the diff output
				format.MaxDepth = 100
				format.MaxLength = 0
				appSource.RepoURL = pattern.Spec.MultiSourceConfig.HelmRepoUrl
				appSource.Chart = "clustergroup"
				appSource.Path = ""
				appSource.TargetRevision = pattern.Spec.MultiSourceConfig.ClusterGroupChartVersion
				multiSourceArgoApp = argoApp.DeepCopy()
				multiSourceArgoApp.Spec.Source = nil
				multiSourceArgoApp.Spec.Sources = []argoapi.ApplicationSource{
					{
						RepoURL:        pattern.Spec.GitConfig.TargetRepo,
						TargetRevision: pattern.Spec.GitConfig.TargetRevision,
						Ref:            "patternref",
					},
					*appSource,
				}
				multiSourceArgoApp.Spec.Sources[1].Helm.ValueFiles = newApplicationValueFiles(pattern, "$patternref")
				Expect(newMultiSourceApplication(pattern)).To(Equal(multiSourceArgoApp))
			})
		})
		Context("multiSource with MultiSourceClusterGroupChartGitRevision set", func() {
			It("Returns an argo application with multiple sources with clustergroup pointing to a git repo", func() {
				format.MaxDepth = 100
				format.MaxLength = 0
				pattern.Spec.MultiSourceConfig.ClusterGroupGitRepoUrl = "https://github.com/validatedpatterns/clustergroup-chart"
				pattern.Spec.MultiSourceConfig.ClusterGroupChartGitRevision = "testbranch"
				appSource.RepoURL = pattern.Spec.MultiSourceConfig.ClusterGroupGitRepoUrl
				appSource.Chart = ""
				appSource.Path = "."
				appSource.TargetRevision = pattern.Spec.MultiSourceConfig.ClusterGroupChartGitRevision
				multiSourceArgoApp = argoApp.DeepCopy()
				multiSourceArgoApp.Spec.Source = nil
				multiSourceArgoApp.Spec.Sources = []argoapi.ApplicationSource{
					{
						RepoURL:        pattern.Spec.GitConfig.TargetRepo,
						TargetRevision: pattern.Spec.GitConfig.TargetRevision,
						Ref:            "patternref",
					},
					*appSource,
				}
				multiSourceArgoApp.Spec.Sources[1].Helm.ValueFiles = newApplicationValueFiles(pattern, "$patternref")
				Expect(newMultiSourceApplication(pattern)).To(Equal(multiSourceArgoApp))
			})
		})
	})

	Describe("Testing newApplicationValueFiles function", func() {
		Context("Default", func() {
			It("Returns a default set of values", func() {
				valueFiles := newApplicationValueFiles(pattern, "")
				Expect(valueFiles).To(Equal(defaultValueFiles))
			})
			It("Returns a default set of values with prefix", func() {
				valueFiles := newApplicationValueFiles(pattern, "myprefix")
				Expect(valueFiles).To(Equal(prefixArray(defaultValueFiles, "myprefix")))
			})
		})

		Context("With extra valuefiles", func() {
			BeforeEach(func() {
				pattern.Spec.ExtraValueFiles = []string{
					"test1.yaml",
					"test2.yaml",
				}
			})
			It("Returns a default set of values and extravaluefiles without prefix", func() {
				valueFiles := newApplicationValueFiles(pattern, "")
				Expect(valueFiles).To(Equal(append(defaultValueFiles,
					"/test1.yaml",
					"/test2.yaml")))
			})
			It("Returns a default set of values and extravaluefiles with prefix", func() {
				valueFiles := newApplicationValueFiles(pattern, "myprefix")
				Expect(valueFiles).To(Equal(append(prefixArray(defaultValueFiles, "myprefix"),
					"myprefix/test1.yaml",
					"myprefix/test2.yaml")))
			})
		})

		Context("With extra valuefiles with leading slashes", func() {
			BeforeEach(func() {
				pattern.Spec.ExtraValueFiles = []string{
					"/test1.yaml",
					"/test2.yaml",
				}
			})
			It("Returns a default set of values and extravaluefiles", func() {
				valueFiles := newApplicationValueFiles(pattern, "")
				Expect(valueFiles).To(Equal(append(defaultValueFiles,
					"/test1.yaml",
					"/test2.yaml")))
			})
			It("Returns a default set of values and extravaluefiles with prefix", func() {
				valueFiles := newApplicationValueFiles(pattern, "myprefix")
				Expect(valueFiles).To(Equal(append(prefixArray(defaultValueFiles, "myprefix"),
					"myprefix/test1.yaml",
					"myprefix/test2.yaml")))
			})
		})
	})

	Describe("Argo Helm Functions", func() {
		var goal, actual []string
		var goalHelm, actualHelm []argoapi.HelmParameter
		var goalSourceHelm *argoapi.ApplicationSourceHelm
		BeforeEach(func() {
			goal = defaultValueFiles
			actual = append(defaultValueFiles, "/values-excess.yaml")
			goalHelm = []argoapi.HelmParameter{
				{
					Name:  "foo",
					Value: "foovalue",
				},
				{
					Name:        "bar",
					Value:       "barvalue",
					ForceString: true,
				},
				{
					Name:        "baz",
					Value:       "bazvalue",
					ForceString: false,
				},
				{
					Name:        "int1",
					Value:       "1",
					ForceString: false,
				},
				{
					Name:        "int2",
					Value:       "2",
					ForceString: true,
				},
			}
			actualHelm = append(goalHelm, argoapi.HelmParameter{
				Name:  "excess",
				Value: "excessvalue",
			})
			goalSourceHelm = &argoapi.ApplicationSourceHelm{
				ValueFiles: defaultValueFiles,
				Parameters: goalHelm,
			}
		})

		Context("Compare Helm Values", func() {
			It("Compare different Helm Value Files", func() {
				Expect(compareHelmValueFiles(goal, actual)).To(BeFalse())
			})
			It("Compare same Helm Value Files", func() {
				sameGoal := goal
				Expect(compareHelmValueFiles(goal, sameGoal)).To(BeTrue())
			})
		})

		Context("Compare Helm Parameters", func() {
			It("Compare different Helm Parameters", func() {
				Expect(compareHelmParameters(goalHelm, actualHelm)).To(BeFalse())
			})
			It("Compare same Helm Parameters", func() {
				sameGoalHelm := goalHelm
				Expect(compareHelmParameters(goalHelm, sameGoalHelm)).To(BeTrue())
			})
			It("Test updateHelmParameter non existing Parameter", func() {
				nonexistantParam := api.PatternParameter{
					Name:  "Nonexistant",
					Value: "nonexistantvalue",
				}
				Expect(updateHelmParameter(nonexistantParam, actualHelm)).To(BeFalse())
			})
			It("Test updateHelmParameter with existing Parameter with same value", func() {
				existantParam := api.PatternParameter{
					Name:  "foo",
					Value: "foovalue",
				}
				Expect(updateHelmParameter(existantParam, actualHelm)).To(BeTrue())
			})
			It("Test updateHelmParameter with existing Parameter with different value", func() {
				existantParam := api.PatternParameter{
					Name:  "foo",
					Value: "foovaluedifferent",
				}
				Expect(updateHelmParameter(existantParam, actualHelm)).To(BeTrue())
			})
			It("Test different compareHelmSource", func() {
				actualSourceHelm := &argoapi.ApplicationSourceHelm{
					ValueFiles: defaultValueFiles,
					Parameters: actualHelm,
				}
				Expect(compareHelmSource(goalSourceHelm, actualSourceHelm)).To(BeFalse())
			})
			It("Test same compareHelmSource", func() {
				sameSourceHelm := goalSourceHelm
				Expect(compareHelmSource(goalSourceHelm, sameSourceHelm)).To(BeTrue())
			})
		})

		Context("Application Parameters", func() {
			var appParameters []argoapi.HelmParameter
			BeforeEach(func() {
				appParameters = []argoapi.HelmParameter{
					{
						Name:        "global.pattern",
						Value:       "multicloud-gitops-test",
						ForceString: false,
					},
					{
						Name:        "global.namespace",
						Value:       "default",
						ForceString: false,
					},
					{
						Name:        "global.repoURL",
						Value:       "https://github.com/validatedpatterns/multicloud-gitops",
						ForceString: false,
					},
					{
						Name:        "global.targetRevision",
						Value:       "main",
						ForceString: false,
					},
					{
						Name:        "global.hubClusterDomain",
						Value:       "apps.hub-cluster.validatedpatterns.io",
						ForceString: false,
					},
					{
						Name:        "global.localClusterDomain",
						Value:       "apps.hub-cluster.validatedpatterns.io",
						ForceString: false,
					},
					{
						Name:        "global.clusterDomain",
						Value:       "hub-cluster.validatedpatterns.io",
						ForceString: false,
					},
					{
						Name:        "global.clusterVersion",
						Value:       "4.12",
						ForceString: false,
					},
					{
						Name:        "global.clusterPlatform",
						Value:       "AWS",
						ForceString: false,
					},
					{
						Name:        "global.localClusterName",
						Value:       "barcluster",
						ForceString: false,
					},
					{
						Name:        "global.privateRepo",
						Value:       "false",
						ForceString: false,
					},
				}
			})
			It("Test default newApplicationParameters", func() {
				Expect(newApplicationParameters(pattern)).To(Equal(append(appParameters,
					argoapi.HelmParameter{
						Name:        "global.multiSourceSupport",
						Value:       "false",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:        "global.experimentalCapabilities",
						Value:       "",
						ForceString: false,
					},
				)))
			})
			It("Test newApplicationParameters with extra parameters", func() {
				pattern.Spec.ExtraParameters = []api.PatternParameter{
					{
						Name:  "test1",
						Value: "test1value",
					},
					{
						Name:  "test2",
						Value: "test2value",
					},
				}
				Expect(newApplicationParameters(pattern)).To(Equal(append(appParameters,
					argoapi.HelmParameter{
						Name:        "global.multiSourceSupport",
						Value:       "false",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:        "global.experimentalCapabilities",
						Value:       "",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:  "test1",
						Value: "test1value",
					},
					argoapi.HelmParameter{
						Name:  "test2",
						Value: "test2value",
					})))
			})
			It("Test newApplicationParameters with multiSource", func() {
				tmpBool := true
				pattern.Spec.MultiSourceConfig.Enabled = &tmpBool
				Expect(newApplicationParameters(pattern)).To(Equal(append(appParameters,
					argoapi.HelmParameter{
						Name:        "global.multiSourceSupport",
						Value:       "true",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:  "global.multiSourceRepoUrl",
						Value: "https://charts.validatedpatterns.io/",
					},
					argoapi.HelmParameter{
						Name:  "global.multiSourceTargetRevision",
						Value: "0.0.*",
					},
					argoapi.HelmParameter{
						Name:        "global.experimentalCapabilities",
						Value:       "",
						ForceString: false,
					})))
			})
		})

		Context("Compare Sources", func() {
			var multiSourceArgoApp *argoapi.Application
			var sources []argoapi.ApplicationSource

			BeforeEach(func() {
				multiSourceArgoApp = newMultiSourceApplication(pattern)
				sources = multiSourceArgoApp.Spec.Sources
			})
			It("compareSource() function identical", func() {
				Expect(compareSource(appSource, appSource)).To(BeTrue())
			})
			It("compareSource() function differing", func() {
				appSourceChanged := appSource.DeepCopy()
				appSourceChanged.Path = "different"
				Expect(compareSource(appSource, appSourceChanged)).To(BeFalse())
			})
			It("compareSources() function with nil arg1", func() {
				Expect(compareSources(sources, nil)).To(BeFalse())
			})
			It("compareSources() function with nil arg2", func() {
				Expect(compareSources(nil, sources)).To(BeFalse())
			})
			It("compareSources() function different length", func() {
				Expect(compareSources(sources, append(sources, *appSource))).To(BeFalse())
			})
			It("compareSources() function one length 0 argument", func() {
				Expect(compareSources(sources, []argoapi.ApplicationSource{})).To(BeFalse())
			})
			It("compareSources() function identical", func() {
				Expect(compareSources(sources, sources)).To(BeTrue())
			})

		})
	})
})

var _ = Describe("RemoveApplication", func() {
	var (
		argocdclient *argoclient.Clientset
		name         string
		namespace    string
	)

	BeforeEach(func() {
		argocdclient = argoclient.NewSimpleClientset()
		name = "test-application"
		namespace = "default"
	})

	Context("when the application exists", func() {
		BeforeEach(func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
			_, err := argocdclient.ArgoprojV1alpha1().Applications(namespace).Create(context.Background(), app, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete the application successfully", func() {
			err := removeApplication(argocdclient, name, namespace)
			Expect(err).ToNot(HaveOccurred())

			_, err = argocdclient.ArgoprojV1alpha1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())

		})
	})

	Context("when the application does not exist", func() {
		It("should return a not found error", func() {
			err := removeApplication(argocdclient, name, namespace)
			Expect(err).To(HaveOccurred())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("when there is an error deleting the application", func() {
		BeforeEach(func() {
			argocdclient.PrependReactor("delete", "applications", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("delete error")
			})
		})

		It("should return the error", func() {
			err := removeApplication(argocdclient, name, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("delete error"))
		})
	})
})

var _ = Describe("NewApplicationValues", func() {
	Context("when there are no extra parameters", func() {
		It("should return the header with no parameters", func() {
			pattern := &api.Pattern{
				Spec: api.PatternSpec{
					ExtraParameters: []api.PatternParameter{},
				},
			}

			result := newApplicationValues(pattern)
			Expect(result).To(Equal("extraParametersNested:\n"))
		})
	})

	Context("when there is one extra parameter", func() {
		It("should return the parameter correctly formatted", func() {
			pattern := &api.Pattern{
				Spec: api.PatternSpec{
					ExtraParameters: []api.PatternParameter{
						{Name: "param1", Value: "value1"},
					},
				},
			}

			result := newApplicationValues(pattern)
			expected := "extraParametersNested:\n  param1: value1\n"
			Expect(result).To(Equal(expected))
		})
	})

	Context("when there are multiple extra parameters", func() {
		It("should return the parameters correctly formatted", func() {
			pattern := &api.Pattern{
				Spec: api.PatternSpec{
					ExtraParameters: []api.PatternParameter{
						{Name: "param1", Value: "value1"},
						{Name: "param2", Value: "value2"},
					},
				},
			}

			result := newApplicationValues(pattern)
			expected := "extraParametersNested:\n  param1: value1\n  param2: value2\n"
			Expect(result).To(Equal(expected))
		})
	})

	Context("when the parameter names and values contain special characters", func() {
		It("should return the parameters correctly formatted", func() {
			pattern := &api.Pattern{
				Spec: api.PatternSpec{
					ExtraParameters: []api.PatternParameter{
						{Name: "param-1", Value: "value-1"},
						{Name: "param_2", Value: "value_2"},
					},
				},
			}

			result := newApplicationValues(pattern)
			expected := "extraParametersNested:\n  param-1: value-1\n  param_2: value_2\n"
			Expect(result).To(Equal(expected))
		})
	})
})

var _ = Describe("CompareHelmValueFile", func() {
	var (
		goal   string
		actual []string
	)

	Context("when the goal value is in the actual slice", func() {
		BeforeEach(func() {
			goal = "value1"
			actual = []string{"value1", "value2", "value3"}
		})

		It("should return true", func() {
			result := compareHelmValueFile(goal, actual)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the goal value is not in the actual slice", func() {
		BeforeEach(func() {
			goal = "value4"
			actual = []string{"value1", "value2", "value3"}
		})

		It("should return false and log the appropriate message", func() {
			logBuffer := new(bytes.Buffer)
			log.SetOutput(logBuffer)
			defer log.SetOutput(os.Stderr)

			result := compareHelmValueFile(goal, actual)
			Expect(result).To(BeFalse())
			Expect(logBuffer.String()).To(ContainSubstring("Values file \"value4\" not found"))
		})
	})

	Context("when the actual slice is empty", func() {
		BeforeEach(func() {
			goal = "value1"
			actual = []string{}
		})

		It("should return false and log the appropriate message", func() {
			logBuffer := new(bytes.Buffer)
			log.SetOutput(logBuffer)
			defer log.SetOutput(os.Stderr)

			result := compareHelmValueFile(goal, actual)
			Expect(result).To(BeFalse())
			Expect(logBuffer.String()).To(ContainSubstring("Values file \"value1\" not found"))
		})
	})

	Context("when the goal value is empty", func() {
		BeforeEach(func() {
			goal = ""
			actual = []string{"value1", "value2", "value3"}
		})

		It("should return false and log the appropriate message", func() {
			logBuffer := new(bytes.Buffer)
			log.SetOutput(logBuffer)
			defer log.SetOutput(os.Stderr)

			result := compareHelmValueFile(goal, actual)
			Expect(result).To(BeFalse())
			Expect(logBuffer.String()).To(ContainSubstring("Values file \"\" not found"))
		})
	})

	Context("when both the goal value and the actual slice are empty", func() {
		BeforeEach(func() {
			goal = ""
			actual = []string{}
		})

		It("should return false and log the appropriate message", func() {
			logBuffer := new(bytes.Buffer)
			log.SetOutput(logBuffer)
			defer log.SetOutput(os.Stderr)

			result := compareHelmValueFile(goal, actual)
			Expect(result).To(BeFalse())
			Expect(logBuffer.String()).To(ContainSubstring("Values file \"\" not found"))
		})
	})
})

var _ = Describe("NewArgoCD", func() {
	var (
		name      string
		namespace string
		argoCD    *argooperator.ArgoCD
	)

	BeforeEach(func() {
		name = "test-argocd"
		namespace = "test-namespace"
		argoCD = newArgoCD(name, namespace)
	})

	Context("when creating a new ArgoCD object", func() {
		It("should have the correct metadata", func() {
			Expect(argoCD.Name).To(Equal(name))
			Expect(argoCD.Namespace).To(Equal(namespace))
			Expect(argoCD.Kind).To(Equal("ArgoCD"))
			Expect(argoCD.APIVersion).To(Equal("argoproj.io/v1beta1"))
		})

		It("should have the correct spec values", func() {
			spec := argoCD.Spec

			Expect(spec.ApplicationSet.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse("2")))
			Expect(spec.ApplicationSet.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
			Expect(spec.ApplicationSet.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse("250m")))
			Expect(spec.ApplicationSet.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse("512Mi")))

			Expect(spec.Controller.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse("2")))
			Expect(spec.Controller.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			Expect(spec.Controller.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse("250m")))
			Expect(spec.Controller.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))

			Expect(spec.Grafana.Enabled).To(BeFalse())
			Expect(spec.Monitoring.Enabled).To(BeFalse())
			Expect(spec.Notifications.Enabled).To(BeFalse())
			Expect(spec.Prometheus.Enabled).To(BeFalse())
			Expect(spec.Server.Route.Enabled).To(BeTrue())

			Expect(spec.RBAC.Policy).ToNot(BeNil())
			Expect(spec.RBAC.Scopes).ToNot(BeNil())
		})

		It("should have the correct init containers", func() {
			Expect(argoCD.Spec.Repo.InitContainers).To(HaveLen(1))
			initContainer := argoCD.Spec.Repo.InitContainers[0]
			Expect(initContainer.Name).To(Equal("fetch-ca"))
			Expect(initContainer.Image).To(Equal("registry.redhat.io/ansible-automation-platform-24/ee-supported-rhel9:latest"))
		})

		It("should have the correct volumes", func() {
			Expect(argoCD.Spec.Repo.Volumes).To(HaveLen(3))
			Expect(argoCD.Spec.Repo.Volumes[0].Name).To(Equal("kube-root-ca"))
			Expect(argoCD.Spec.Repo.Volumes[1].Name).To(Equal("trusted-ca-bundle"))
			Expect(argoCD.Spec.Repo.Volumes[2].Name).To(Equal("ca-bundles"))
		})

		It("should have the correct volume mounts", func() {
			Expect(argoCD.Spec.Repo.VolumeMounts).To(HaveLen(1))
			Expect(argoCD.Spec.Repo.VolumeMounts[0].Name).To(Equal("ca-bundles"))
			Expect(argoCD.Spec.Repo.VolumeMounts[0].MountPath).To(Equal("/etc/pki/tls/certs"))
		})

		It("should have the correct server route TLS configuration", func() {
			Expect(argoCD.Spec.Server.Route.TLS).ToNot(BeNil())
			Expect(argoCD.Spec.Server.Route.TLS.Termination).To(Equal(routev1.TLSTerminationReencrypt))
			Expect(argoCD.Spec.Server.Route.TLS.InsecureEdgeTerminationPolicy).To(Equal(routev1.InsecureEdgeTerminationPolicyRedirect))
		})
	})
})

var _ = Describe("haveArgo", func() {
	var (
		dynamicClient dynamic.Interface
		kubeClient    *fake.Clientset

		gvr       schema.GroupVersionResource
		name      string
		namespace string
	)

	BeforeEach(func() {
		gvr = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1beta1", Resource: "argocds"}
		kubeClient = fake.NewSimpleClientset()
		dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			gvr: "ArgoCDList",
		})
		name = "test-argocd"
		namespace = "test-namespace"
	})

	Context("when the ArgoCD instance exists", func() {
		BeforeEach(func() {
			argoCD := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1beta1",
					"kind":       "ArgoCD",
					"metadata": map[string]interface{}{
						"name":      name,
						"namespace": namespace,
					},
				},
			}
			_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(context.Background(), argoCD, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return true", func() {
			result := haveArgo(dynamicClient, name, namespace)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the ArgoCD instance does not exist", func() {
		It("should return false", func() {
			result := haveArgo(dynamicClient, name, namespace)
			Expect(result).To(BeFalse())
		})
	})

	Context("when there is an error retrieving the ArgoCD instance", func() {
		BeforeEach(func() {
			kubeClient.PrependReactor("get", "argocds", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("get error")
			})
		})

		It("should return false", func() {
			result := haveArgo(dynamicClient, name, namespace)
			Expect(result).To(BeFalse())
		})
	})
})
