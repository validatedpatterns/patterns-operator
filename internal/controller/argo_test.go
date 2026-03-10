package controllers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	argooperator "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	routev1 "github.com/openshift/api/route/v1"

	argoapi "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoclient "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
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

const (
	argoNS   = "test-namespace"
	argoName = "test-argocd"
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
		Context("multiSource with targetPath set", func() {
			It("Returns an argo application with path in values source", func() {
				format.MaxDepth = 100
				format.MaxLength = 0
				pattern.Spec.GitConfig.TargetPath = "envs/dev"
				tmpTrue := true
				pattern.Spec.MultiSourceConfig.Enabled = &tmpTrue
				multiSourceArgoApp = argoApp.DeepCopy()
				multiSourceArgoApp.Spec.Source = nil
				multiSourceArgoApp.Spec.Sources = []argoapi.ApplicationSource{
					{
						RepoURL:        pattern.Spec.GitConfig.TargetRepo,
						TargetRevision: pattern.Spec.GitConfig.TargetRevision,
						Path:           "envs/dev",
						Ref:            "patternref",
					},
					{
						RepoURL:        pattern.Spec.MultiSourceConfig.HelmRepoUrl,
						Chart:          "clustergroup",
						Path:           "",
						TargetRevision: pattern.Spec.MultiSourceConfig.ClusterGroupChartVersion,
						Helm:           commonApplicationSourceHelm(pattern, "$patternref"),
					},
				}
				Expect(newMultiSourceApplication(pattern)).To(Equal(multiSourceArgoApp))
				pattern.Spec.GitConfig.TargetPath = ""
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

		Context("With targetPath set", func() {
			BeforeEach(func() {
				pattern.Spec.GitConfig.TargetPath = "envs/dev"
			})
			AfterEach(func() {
				pattern.Spec.GitConfig.TargetPath = ""
			})
			It("Returns values with targetPath prefix", func() {
				valueFiles := newApplicationValueFiles(pattern, "")
				Expect(valueFiles).To(Equal(prefixArray(defaultValueFiles, "envs/dev")))
			})
			It("Returns values with combined prefix and targetPath", func() {
				valueFiles := newApplicationValueFiles(pattern, "$patternref")
				Expect(valueFiles).To(Equal(prefixArray(defaultValueFiles, "$patternref/envs/dev")))
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
			It("Compare Helm Value Files with different order", func() {
				sortedGoal := make([]string, len(goal))
				reversedGoal := make([]string, len(goal))
				_ = copy(sortedGoal, goal)
				_ = copy(reversedGoal, goal)
				slices.Sort(sortedGoal)
				slices.Reverse(reversedGoal)
				Expect(compareHelmValueFiles(sortedGoal, reversedGoal)).To(BeFalse())
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
			It("Compare Helm Parameters with different order", func() {
				sortedGoalHelm := make([]argoapi.HelmParameter, len(goalHelm))
				reversedGoalHelm := make([]argoapi.HelmParameter, len(goalHelm))
				_ = copy(sortedGoalHelm, goalHelm)
				_ = copy(reversedGoalHelm, goalHelm)
				slices.SortFunc(sortedGoalHelm, func(a, b argoapi.HelmParameter) int {
					return strings.Compare(a.Name, b.Name)
				})
				slices.SortFunc(reversedGoalHelm, func(a, b argoapi.HelmParameter) int {
					return strings.Compare(a.Name, b.Name) * -1
				})
				Expect(compareHelmParameters(sortedGoalHelm, reversedGoalHelm)).To(BeFalse())
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
						Name:  "global.originURL",
						Value: "",
					},
					{
						Name:        "global.targetRevision",
						Value:       "main",
						ForceString: false,
					},
					{
						Name:  "global.targetPath",
						Value: "",
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
						Name:        "global.multiSourceRepoUrl",
						Value:       "https://charts.validatedpatterns.io/",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:        "global.experimentalCapabilities",
						Value:       "",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:        "global.multiSourceTargetRevision",
						Value:       "0.0.*",
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
						Name:        "global.multiSourceRepoUrl",
						Value:       "https://charts.validatedpatterns.io/",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:        "global.experimentalCapabilities",
						Value:       "",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:        "global.multiSourceTargetRevision",
						Value:       "0.0.*",
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
						Name:        "global.experimentalCapabilities",
						Value:       "",
						ForceString: false,
					},
					argoapi.HelmParameter{
						Name:  "global.multiSourceTargetRevision",
						Value: "0.0.*",
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

		Context("Compare SyncPolicy", func() {
			var multiSourceArgoApp *argoapi.Application
			var syncPolicy *argoapi.SyncPolicy

			BeforeEach(func() {
				multiSourceArgoApp = newMultiSourceApplication(pattern)
				syncPolicy = multiSourceArgoApp.Spec.SyncPolicy
			})
			It("compareSyncPolicy() function identical", func() {
				Expect(compareSyncPolicy(syncPolicy, syncPolicy)).To(BeTrue())
			})
			It("compareSyncPolicy() function differing", func() {
				syncPolicyChanged := &argoapi.SyncPolicy{}
				Expect(compareSyncPolicy(syncPolicy, syncPolicyChanged)).To(BeFalse())
			})
			It("compareSyncPolicy() function with nil arg1", func() {
				Expect(compareSyncPolicy(syncPolicy, nil)).To(BeFalse())
			})
			It("compareSyncPolicy() function with nil arg2", func() {
				Expect(compareSyncPolicy(nil, syncPolicy)).To(BeFalse())
			})

		})

		Context("Compare AutomatedSyncPolicy", func() {
			var multiSourceArgoApp *argoapi.Application
			var automatedSyncPolicy *argoapi.SyncPolicyAutomated

			BeforeEach(func() {
				multiSourceArgoApp = newMultiSourceApplication(pattern)
				automatedSyncPolicy = multiSourceArgoApp.Spec.SyncPolicy.Automated
			})
			It("compareAutomatedSyncPolicy() function identical", func() {
				Expect(compareAutomatedSyncPolicy(automatedSyncPolicy, automatedSyncPolicy)).To(BeTrue())
			})
			It("should return false and log the appropriate message", func() {
				automatedSyncPolicyChanged := automatedSyncPolicy.DeepCopy()
				automatedSyncPolicyChanged.Prune = true
				logBuffer := new(bytes.Buffer)
				log.SetOutput(logBuffer)
				defer log.SetOutput(os.Stderr)

				result := compareAutomatedSyncPolicy(automatedSyncPolicy, automatedSyncPolicyChanged)
				Expect(result).To(BeFalse())
				Expect(logBuffer.String()).To(ContainSubstring("SyncPolicy Prune changed true -> false"))
			})
			It("should return false and log the appropriate message", func() {
				automatedSyncPolicyChanged := automatedSyncPolicy.DeepCopy()
				automatedSyncPolicyChanged.AllowEmpty = true
				logBuffer := new(bytes.Buffer)
				log.SetOutput(logBuffer)
				defer log.SetOutput(os.Stderr)

				result := compareAutomatedSyncPolicy(automatedSyncPolicy, automatedSyncPolicyChanged)
				Expect(result).To(BeFalse())
				Expect(logBuffer.String()).To(ContainSubstring("SyncPolicy AllowEmpty changed true -> false"))
			})
			It("should return false and log the appropriate message", func() {
				automatedSyncPolicyChanged := automatedSyncPolicy.DeepCopy()
				automatedSyncPolicyChanged.SelfHeal = true
				logBuffer := new(bytes.Buffer)
				log.SetOutput(logBuffer)
				defer log.SetOutput(os.Stderr)

				result := compareAutomatedSyncPolicy(automatedSyncPolicy, automatedSyncPolicyChanged)
				Expect(result).To(BeFalse())
				Expect(logBuffer.String()).To(ContainSubstring("SyncPolicy SelfHeal changed true -> false"))
			})
			It("compareAutomatedSyncPolicy() function with nil arg1", func() {
				Expect(compareAutomatedSyncPolicy(automatedSyncPolicy, nil)).To(BeFalse())
			})
			It("compareAutomatedSyncPolicy() function with nil arg2", func() {
				Expect(compareAutomatedSyncPolicy(nil, automatedSyncPolicy)).To(BeFalse())
			})

		})

		Context("Compare SyncOptions", func() {
			var multiSourceArgoApp *argoapi.Application
			var syncOptions argoapi.SyncOptions

			BeforeEach(func() {
				multiSourceArgoApp = newMultiSourceApplication(pattern)
				syncOptions = multiSourceArgoApp.Spec.SyncPolicy.SyncOptions
			})
			It("compareSyncOptions() function identical", func() {
				Expect(compareSyncOptions(syncOptions, syncOptions)).To(BeTrue())
			})
			It("compareSyncOptions() function differing", func() {
				syncOptionsChanged := append(syncOptions, "key=value")
				Expect(compareSyncOptions(syncOptions, syncOptionsChanged)).To(BeFalse())
			})
			It("Compare SyncOptions with different order", func() {
				syncOptions1 := []string{"opt1=value1", "opt2=value2"}
				syncOptions2 := []string{"opt2=value2", "opt1=value1"}
				Expect(compareSyncOptions(syncOptions1, syncOptions2)).To(BeFalse())
			})
			It("compareSyncOptions() function with nil arg1", func() {
				Expect(compareSyncOptions(syncOptions, nil)).To(BeFalse())
			})
			It("compareSyncOptions() function with nil arg2", func() {
				Expect(compareSyncOptions(nil, syncOptions)).To(BeFalse())
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
			Expect(spec.Controller.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse("8Gi")))
			Expect(spec.Controller.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse("250m")))
			Expect(spec.Controller.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))

			// Expect(spec.Grafana.Enabled).To(BeFalse()) // spec.Grafana is deprecated
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
			Expect(initContainer.Image).To(Equal("registry.redhat.io/ubi9/ubi-minimal:latest"))
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
		name = argoName
		namespace = argoNS
	})

	Context("when the ArgoCD instance exists", func() {
		BeforeEach(func() {
			argoCD := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "argoproj.io/v1beta1",
					"kind":       "ArgoCD",
					"metadata": map[string]any{
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

var _ = Describe("CreateOrUpdateArgoCD", func() {
	var (
		dynamicClient dynamic.Interface
		gvr           schema.GroupVersionResource
		name          string
		namespace     string
	)

	BeforeEach(func() {
		gvr = schema.GroupVersionResource{Group: ArgoCDGroup, Version: ArgoCDVersion, Resource: ArgoCDResource}
		dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			gvr: "ArgoCDList",
		})
		name = argoName
		namespace = argoNS
	})

	Context("when the ArgoCD instance does not exist", func() {
		It("should create a new ArgoCD instance", func() {
			err := createOrUpdateArgoCD(dynamicClient, nil, name, namespace)
			Expect(err).ToNot(HaveOccurred())

			argoCD, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(argoCD.GetName()).To(Equal(name))
			Expect(argoCD.GetNamespace()).To(Equal(namespace))
		})
	})

	Context("when the ArgoCD instance exists", func() {
		BeforeEach(func() {
			argoCD := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "argoproj.io/v1beta1",
					"kind":       "ArgoCD",
					"metadata": map[string]any{
						"name":            name,
						"namespace":       namespace,
						"resourceVersion": "1",
					},
				},
			}
			_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), argoCD, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should update the existing ArgoCD instance", func() {
			err := createOrUpdateArgoCD(dynamicClient, nil, name, namespace)
			Expect(err).ToNot(HaveOccurred())

			argoCD, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(argoCD.GetResourceVersion()).To(Equal("1")) // Ensure it has been updated
		})
	})
})

var _ = Describe("CompareApplication", func() {
	var pattern *api.Pattern
	BeforeEach(func() {
		tmpFalse := false
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pattern", Namespace: defaultNamespace},
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
	})

	Context("when both applications are nil", func() {
		It("should return true", func() {
			Expect(compareApplication(nil, nil)).To(BeTrue())
		})
	})

	Context("when one application is nil and the other is not", func() {
		It("should return false when goal is nil", func() {
			app := newArgoApplication(pattern)
			Expect(compareApplication(nil, app)).To(BeFalse())
		})
		It("should return false when actual is nil", func() {
			app := newArgoApplication(pattern)
			Expect(compareApplication(app, nil)).To(BeFalse())
		})
	})

	Context("when both applications are identical", func() {
		It("should return true", func() {
			app := newArgoApplication(pattern)
			Expect(compareApplication(app, app)).To(BeTrue())
		})
	})

	Context("when applications have different sources", func() {
		It("should return false", func() {
			app1 := newArgoApplication(pattern)
			app2 := app1.DeepCopy()
			app2.Spec.Source.RepoURL = "https://different.repo/url"
			Expect(compareApplication(app1, app2)).To(BeFalse())
		})
	})

	Context("when applications have different sync policies", func() {
		It("should return false", func() {
			app1 := newArgoApplication(pattern)
			app2 := app1.DeepCopy()
			app2.Spec.SyncPolicy = nil
			Expect(compareApplication(app1, app2)).To(BeFalse())
		})
	})
})

var _ = Describe("GetApplication", func() {
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

		It("should return the application", func() {
			app, err := getApplication(argocdclient, name, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(app).ToNot(BeNil())
			Expect(app.Name).To(Equal(name))
		})
	})

	Context("when the application does not exist", func() {
		It("should return an error", func() {
			app, err := getApplication(argocdclient, "nonexistent", namespace)
			Expect(err).To(HaveOccurred())
			Expect(app).To(BeNil())
		})
	})
})

var _ = Describe("CreateApplication", func() {
	var (
		argocdclient *argoclient.Clientset
		namespace    string
	)

	BeforeEach(func() {
		argocdclient = argoclient.NewSimpleClientset()
		namespace = "default"
	})

	Context("when creating an application", func() {
		It("should create successfully", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-app",
					Namespace: namespace,
				},
			}
			err := createApplication(argocdclient, app, namespace)
			Expect(err).ToNot(HaveOccurred())

			created, err := argocdclient.ArgoprojV1alpha1().Applications(namespace).Get(context.Background(), "new-app", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(created.Name).To(Equal("new-app"))
		})
	})

	Context("when creation fails", func() {
		BeforeEach(func() {
			argocdclient.PrependReactor("create", "applications", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("create error")
			})
		})

		It("should return the error", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-app",
					Namespace: namespace,
				},
			}
			err := createApplication(argocdclient, app, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("create error"))
		})
	})
})

var _ = Describe("UpdateApplication", func() {
	var (
		argocdclient *argoclient.Clientset
		namespace    string
	)

	BeforeEach(func() {
		argocdclient = argoclient.NewSimpleClientset()
		namespace = "default"
	})

	Context("when current is nil", func() {
		It("should return an error", func() {
			target := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
			}
			changed, err := updateApplication(argocdclient, target, nil, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("current application was nil"))
			Expect(changed).To(BeFalse())
		})
	})

	Context("when target is nil", func() {
		It("should return an error", func() {
			current := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
			}
			changed, err := updateApplication(argocdclient, nil, current, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("target application was nil"))
			Expect(changed).To(BeFalse())
		})
	})

	Context("when applications are identical", func() {
		It("should not update and return false", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
				Spec: argoapi.ApplicationSpec{
					Source: &argoapi.ApplicationSource{
						RepoURL: "https://example.com/repo",
					},
				},
			}
			_, err := argocdclient.ArgoprojV1alpha1().Applications(namespace).Create(context.Background(), app, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			changed, err := updateApplication(argocdclient, app, app.DeepCopy(), namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
		})
	})

	Context("when applications differ", func() {
		It("should update and return true", func() {
			current := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
				Spec: argoapi.ApplicationSpec{
					Source: &argoapi.ApplicationSource{
						RepoURL: "https://example.com/repo-old",
					},
				},
			}
			_, err := argocdclient.ArgoprojV1alpha1().Applications(namespace).Create(context.Background(), current, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			target := current.DeepCopy()
			target.Spec.Source.RepoURL = "https://example.com/repo-new"

			changed, err := updateApplication(argocdclient, target, current, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
		})
	})
})

var _ = Describe("SyncApplication", func() {
	var (
		argocdclient *argoclient.Clientset
		namespace    string
	)

	BeforeEach(func() {
		argocdclient = argoclient.NewSimpleClientset()
		namespace = "default"
	})

	Context("when sync is already in progress with same options", func() {
		It("should return nil", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
				Operation: &argoapi.Operation{
					Sync: &argoapi.SyncOperation{
						Prune:       true,
						SyncOptions: []string{"Force=true"},
					},
				},
			}
			err := syncApplication(argocdclient, app, true)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when no sync is in progress", func() {
		It("should set sync operation and update", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
			}
			_, err := argocdclient.ArgoprojV1alpha1().Applications(namespace).Create(context.Background(), app, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			err = syncApplication(argocdclient, app, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(app.Operation).ToNot(BeNil())
			Expect(app.Operation.Sync.Prune).To(BeTrue())
			Expect(app.Operation.Sync.SyncOptions).To(ContainElement("Force=true"))
		})
	})

	Context("when sync without prune", func() {
		It("should set prune to false", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
			}
			_, err := argocdclient.ArgoprojV1alpha1().Applications(namespace).Create(context.Background(), app, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			err = syncApplication(argocdclient, app, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(app.Operation.Sync.Prune).To(BeFalse())
		})
	})

	Context("when update fails", func() {
		BeforeEach(func() {
			argocdclient.PrependReactor("update", "applications", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("update error")
			})
		})

		It("should return the error", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
			}
			err := syncApplication(argocdclient, app, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to sync application"))
		})
	})
})

var _ = Describe("GetChildApplications", func() {
	var (
		argocdclient *argoclient.Clientset
		namespace    string
	)

	BeforeEach(func() {
		argocdclient = argoclient.NewSimpleClientset()
		namespace = "default"
	})

	Context("when there are child applications", func() {
		It("should return them", func() {
			parentApp := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "parent-app", Namespace: namespace},
			}

			childApp := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "child-app",
					Namespace: namespace,
					Labels:    map[string]string{"app.kubernetes.io/instance": "parent-app"},
				},
			}
			_, err := argocdclient.ArgoprojV1alpha1().Applications(namespace).Create(context.Background(), childApp, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			children, err := getChildApplications(argocdclient, parentApp)
			Expect(err).ToNot(HaveOccurred())
			Expect(children).To(HaveLen(1))
			Expect(children[0].Name).To(Equal("child-app"))
		})
	})

	Context("when there are no child applications", func() {
		It("should return empty list", func() {
			parentApp := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "parent-app", Namespace: namespace},
			}

			children, err := getChildApplications(argocdclient, parentApp)
			Expect(err).ToNot(HaveOccurred())
			Expect(children).To(BeEmpty())
		})
	})
})

var _ = Describe("NewArgoGiteaApplication", func() {
	var pattern *api.Pattern
	BeforeEach(func() {
		tmpFalse := false
		PatternsOperatorConfig = DefaultPatternOperatorConfig
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pattern", Namespace: defaultNamespace},
			TypeMeta:   metav1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
			Spec: api.PatternSpec{
				ClusterGroupName: "foogroup",
				GitConfig: api.GitConfig{
					TargetRepo:     "https://github.com/validatedpatterns/multicloud-gitops",
					TargetRevision: "main",
				},
				MultiSourceConfig: api.MultiSourceConfig{
					Enabled: &tmpFalse,
				},
				GitOpsConfig: &api.GitOpsConfig{
					ManualSync: false,
				},
			},
			Status: api.PatternStatus{
				AppClusterDomain: "apps.hub-cluster.validatedpatterns.io",
				ClusterDomain:    "hub-cluster.validatedpatterns.io",
			},
		}
	})

	It("should create a gitea application with correct properties", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app).ToNot(BeNil())
		Expect(app.Name).To(Equal(GiteaApplicationName))
		Expect(app.Namespace).To(Equal(getClusterWideArgoNamespace()))
		Expect(app.Labels["validatedpatterns.io/pattern"]).To(Equal("test-pattern"))
		Expect(app.Spec.Destination.Name).To(Equal("in-cluster"))
		Expect(app.Spec.Destination.Namespace).To(Equal(GiteaNamespace))
		Expect(app.Spec.Project).To(Equal("default"))
		Expect(app.Spec.Source).ToNot(BeNil())
		Expect(controllerutil.ContainsFinalizer(app, argoapi.ForegroundPropagationPolicyFinalizer)).To(BeTrue())

		// Check helm parameters
		Expect(app.Spec.Source.Helm).ToNot(BeNil())
		Expect(app.Spec.Source.Helm.Parameters).To(HaveLen(3))

		foundAdminSecret := false
		for _, p := range app.Spec.Source.Helm.Parameters {
			if p.Name == "gitea.admin.existingSecret" {
				foundAdminSecret = true
				Expect(p.Value).To(Equal(GiteaAdminSecretName))
			}
		}
		Expect(foundAdminSecret).To(BeTrue())
	})
})

var _ = Describe("CommonSyncPolicy", func() {
	var pattern *api.Pattern
	BeforeEach(func() {
		tmpFalse := false
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pattern", Namespace: defaultNamespace},
			Spec: api.PatternSpec{
				MultiSourceConfig: api.MultiSourceConfig{
					Enabled: &tmpFalse,
				},
				GitOpsConfig: &api.GitOpsConfig{
					ManualSync: false,
				},
			},
		}
	})

	Context("when pattern is not being deleted and manualSync is false", func() {
		It("should return automated sync policy", func() {
			policy := commonSyncPolicy(pattern)
			Expect(policy).ToNot(BeNil())
			Expect(policy.Automated).ToNot(BeNil())
			Expect(policy.Automated.Prune).To(BeFalse())
		})
	})

	Context("when pattern is not being deleted and manualSync is true", func() {
		It("should return nil sync policy", func() {
			pattern.Spec.GitOpsConfig.ManualSync = true
			policy := commonSyncPolicy(pattern)
			Expect(policy).To(BeNil())
		})
	})

	Context("when pattern is being deleted", func() {
		It("should return sync policy with prune enabled", func() {
			now := metav1.Now()
			pattern.DeletionTimestamp = &now
			policy := commonSyncPolicy(pattern)
			Expect(policy).ToNot(BeNil())
			Expect(policy.Automated).ToNot(BeNil())
			Expect(policy.Automated.Prune).To(BeTrue())
			Expect(policy.SyncOptions).To(ContainElement("Prune=true"))
		})
	})
})

var _ = Describe("NewApplicationParameters with deletion", func() {
	var pattern *api.Pattern
	BeforeEach(func() {
		tmpFalse := false
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pattern", Namespace: defaultNamespace},
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
			},
			Status: api.PatternStatus{
				ClusterPlatform:  "AWS",
				ClusterVersion:   "4.12",
				ClusterName:      "barcluster",
				AppClusterDomain: "apps.hub-cluster.validatedpatterns.io",
				ClusterDomain:    "hub-cluster.validatedpatterns.io",
			},
		}
	})

	Context("when pattern is being deleted with DeleteSpokeChildApps phase", func() {
		It("should include deletePattern parameter with phase value", func() {
			now := metav1.Now()
			pattern.DeletionTimestamp = &now
			pattern.Status.DeletionPhase = api.DeleteSpokeChildApps
			params := newApplicationParameters(pattern)
			foundDelete := false
			for _, p := range params {
				if p.Name == "global.deletePattern" {
					foundDelete = true
					Expect(p.Value).To(Equal(string(api.DeleteSpokeChildApps)))
					Expect(p.ForceString).To(BeTrue())
				}
			}
			Expect(foundDelete).To(BeTrue())
		})
	})

	Context("when pattern is being deleted with DeleteHubChildApps phase", func() {
		It("should include deletePattern=DeleteChildApps", func() {
			now := metav1.Now()
			pattern.DeletionTimestamp = &now
			pattern.Status.DeletionPhase = api.DeleteHubChildApps
			params := newApplicationParameters(pattern)
			foundDelete := false
			for _, p := range params {
				if p.Name == "global.deletePattern" {
					foundDelete = true
					Expect(p.Value).To(Equal("DeleteChildApps"))
					Expect(p.ForceString).To(BeTrue())
				}
			}
			Expect(foundDelete).To(BeTrue())
		})
	})
})

var _ = Describe("CompareSource edge cases", func() {
	Context("when both sources have nil Helm", func() {
		It("should return true for otherwise identical sources", func() {
			source := &argoapi.ApplicationSource{
				RepoURL:        "https://example.com/repo",
				TargetRevision: "main",
				Path:           "path",
			}
			Expect(compareSource(source, source)).To(BeTrue())
		})
	})

	Context("when only one source has Helm set", func() {
		It("should return false when goal has Helm but actual does not", func() {
			goal := &argoapi.ApplicationSource{
				RepoURL:        "https://example.com/repo",
				TargetRevision: "main",
				Path:           "path",
				Helm:           &argoapi.ApplicationSourceHelm{},
			}
			actual := &argoapi.ApplicationSource{
				RepoURL:        "https://example.com/repo",
				TargetRevision: "main",
				Path:           "path",
			}
			Expect(compareSource(goal, actual)).To(BeFalse())
		})

		It("should return false when actual has Helm but goal does not", func() {
			goal := &argoapi.ApplicationSource{
				RepoURL:        "https://example.com/repo",
				TargetRevision: "main",
				Path:           "path",
			}
			actual := &argoapi.ApplicationSource{
				RepoURL:        "https://example.com/repo",
				TargetRevision: "main",
				Path:           "path",
				Helm:           &argoapi.ApplicationSourceHelm{},
			}
			Expect(compareSource(goal, actual)).To(BeFalse())
		})
	})

	Context("when RepoURL differs", func() {
		It("should return false", func() {
			goal := &argoapi.ApplicationSource{RepoURL: "https://a.com"}
			actual := &argoapi.ApplicationSource{RepoURL: "https://b.com"}
			Expect(compareSource(goal, actual)).To(BeFalse())
		})
	})

	Context("when TargetRevision differs", func() {
		It("should return false", func() {
			goal := &argoapi.ApplicationSource{RepoURL: "https://a.com", TargetRevision: "v1"}
			actual := &argoapi.ApplicationSource{RepoURL: "https://a.com", TargetRevision: "v2"}
			Expect(compareSource(goal, actual)).To(BeFalse())
		})
	})
})

var _ = Describe("CompareHelmParameters edge cases", func() {
	Context("when both are nil", func() {
		It("should return true", func() {
			Expect(compareHelmParameters(nil, nil)).To(BeTrue())
		})
	})

	Context("when one is nil", func() {
		It("should return false when goal is nil", func() {
			params := []argoapi.HelmParameter{{Name: "key", Value: "val"}}
			Expect(compareHelmParameters(nil, params)).To(BeFalse())
		})
		It("should return false when actual is nil", func() {
			params := []argoapi.HelmParameter{{Name: "key", Value: "val"}}
			Expect(compareHelmParameters(params, nil)).To(BeFalse())
		})
	})

	Context("when ForceString differs", func() {
		It("should return false", func() {
			goal := []argoapi.HelmParameter{{Name: "key", Value: "val", ForceString: true}}
			actual := []argoapi.HelmParameter{{Name: "key", Value: "val", ForceString: false}}
			Expect(compareHelmParameters(goal, actual)).To(BeFalse())
		})
	})

	Context("when values differ", func() {
		It("should return false", func() {
			goal := []argoapi.HelmParameter{{Name: "key", Value: "val1"}}
			actual := []argoapi.HelmParameter{{Name: "key", Value: "val2"}}
			Expect(compareHelmParameters(goal, actual)).To(BeFalse())
		})
	})
})

var _ = Describe("GetClusterGroupChartVersion", func() {
	var pattern *api.Pattern
	BeforeEach(func() {
		tmpFalse := false
		pattern = &api.Pattern{
			Spec: api.PatternSpec{
				MultiSourceConfig: api.MultiSourceConfig{
					Enabled: &tmpFalse,
				},
			},
		}
	})

	Context("when ClusterGroupChartVersion is explicitly set", func() {
		It("should return the explicit version", func() {
			pattern.Spec.MultiSourceConfig.ClusterGroupChartVersion = "1.2.3"
			Expect(getClusterGroupChartVersion(pattern)).To(Equal("1.2.3"))
		})
	})

	Context("when ClusterGroupChartVersion is not set and common is slimmed", func() {
		It("should return 0.9.*", func() {
			// No operator-install directory means it's slimmed
			pattern.Status.LocalCheckoutPath = "/nonexistent/path"
			Expect(getClusterGroupChartVersion(pattern)).To(Equal("0.9.*"))
		})
	})

	Context("when ClusterGroupChartVersion is not set and common is not slimmed", func() {
		It("should return 0.8.*", func() {
			td := createTempDir("vp-version-test")
			defer cleanupTempDir(td)

			// Create common/operator-install to indicate non-slimmed
			err := os.MkdirAll(filepath.Join(td, "common", "operator-install"), 0755)
			Expect(err).ToNot(HaveOccurred())

			pattern.Status.LocalCheckoutPath = td
			Expect(getClusterGroupChartVersion(pattern)).To(Equal("0.8.*"))
		})
	})
})

var _ = Describe("GetSharedValueFiles", func() {
	var pattern *api.Pattern
	var td string
	BeforeEach(func() {
		td = createTempDir("vp-shared-test")
		tmpFalse := false
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pattern", Namespace: defaultNamespace},
			Spec: api.PatternSpec{
				ClusterGroupName: "foogroup",
				GitConfig: api.GitConfig{
					TargetRepo:     "https://github.com/validatedpatterns/test",
					TargetRevision: "main",
				},
				MultiSourceConfig: api.MultiSourceConfig{
					Enabled: &tmpFalse,
				},
			},
			Status: api.PatternStatus{
				ClusterPlatform:   "AWS",
				ClusterVersion:    "4.12",
				ClusterName:       "barcluster",
				AppClusterDomain:  "apps.hub.example.com",
				ClusterDomain:     "hub.example.com",
				LocalCheckoutPath: td,
			},
		}
	})
	AfterEach(func() {
		cleanupTempDir(td)
	})

	Context("when path does not exist", func() {
		It("should return an error", func() {
			pattern.Status.LocalCheckoutPath = "/nonexistent/path"
			_, err := getSharedValueFiles(pattern, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path does not exist"))
		})
	})

	Context("when there are no sharedValueFiles in the values", func() {
		It("should return nil", func() {
			// Create empty values file
			err := os.WriteFile(filepath.Join(td, "values-global.yaml"), []byte("key: value\n"), 0600)
			Expect(err).ToNot(HaveOccurred())

			result, err := getSharedValueFiles(pattern, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Context("when sharedValueFiles has entries", func() {
		It("should return the value file paths", func() {
			yamlContent := `clusterGroup:
  sharedValueFiles:
    - /values-shared.yaml
`
			err := os.WriteFile(filepath.Join(td, "values-global.yaml"), []byte(yamlContent), 0600)
			Expect(err).ToNot(HaveOccurred())

			result, err := getSharedValueFiles(pattern, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0]).To(ContainSubstring("values-shared.yaml"))
		})
	})
})

var _ = Describe("CountVPApplications", func() {
	var pattern *api.Pattern
	var td string
	BeforeEach(func() {
		td = createTempDir("vp-count-test")
		tmpFalse := false
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pattern", Namespace: defaultNamespace},
			Spec: api.PatternSpec{
				ClusterGroupName: "foogroup",
				GitConfig: api.GitConfig{
					TargetRepo:     "https://github.com/validatedpatterns/test",
					TargetRevision: "main",
				},
				MultiSourceConfig: api.MultiSourceConfig{
					Enabled: &tmpFalse,
				},
			},
			Status: api.PatternStatus{
				ClusterPlatform:   "AWS",
				ClusterVersion:    "4.12",
				ClusterName:       "barcluster",
				AppClusterDomain:  "apps.hub.example.com",
				ClusterDomain:     "hub.example.com",
				LocalCheckoutPath: td,
			},
		}
	})
	AfterEach(func() {
		cleanupTempDir(td)
	})

	Context("when path does not exist", func() {
		It("should return error", func() {
			pattern.Status.LocalCheckoutPath = "/nonexistent/path"
			apps, appsets, err := countVPApplications(pattern)
			Expect(err).To(HaveOccurred())
			Expect(apps).To(Equal(-1))
			Expect(appsets).To(Equal(-1))
		})
	})

	Context("when there are no applications defined", func() {
		It("should return 0, 0", func() {
			err := os.WriteFile(filepath.Join(td, "values-global.yaml"), []byte("key: value\n"), 0600)
			Expect(err).ToNot(HaveOccurred())

			apps, appsets, err := countVPApplications(pattern)
			Expect(err).ToNot(HaveOccurred())
			Expect(apps).To(Equal(0))
			Expect(appsets).To(Equal(0))
		})
	})

	Context("when there are applications defined", func() {
		It("should count them", func() {
			yamlContent := `clusterGroup:
  applications:
    vault:
      name: vault
    golang-external-secrets:
      name: golang-external-secrets
    acm:
      generators:
        - generator1
`
			err := os.WriteFile(filepath.Join(td, "values-global.yaml"), []byte(yamlContent), 0600)
			Expect(err).ToNot(HaveOccurred())

			apps, appsets, err := countVPApplications(pattern)
			Expect(err).ToNot(HaveOccurred())
			Expect(apps).To(Equal(2))
			Expect(appsets).To(Equal(1))
		})
	})
})

var _ = Describe("ConvertArgoHelmParametersToMap", func() {
	Context("when the parameters list is empty", func() {
		It("should return an empty map", func() {
			params := []argoapi.HelmParameter{}
			result := convertArgoHelmParametersToMap(params)
			Expect(result).To(BeEmpty())
		})
	})

	Context("when the parameters list has single level keys", func() {
		It("should return a map with the correct key-value pairs", func() {
			params := []argoapi.HelmParameter{
				{Name: "key1", Value: "value1"},
				{Name: "key2", Value: "value2"},
			}
			result := convertArgoHelmParametersToMap(params)
			Expect(result).To(HaveKeyWithValue("key1", "value1"))
			Expect(result).To(HaveKeyWithValue("key2", "value2"))
		})
	})

	Context("when the parameters list has nested keys", func() {
		It("should return a map with the correct nested structure", func() {
			params := []argoapi.HelmParameter{
				{Name: "key1.subkey1", Value: "value1"},
				{Name: "key1.subkey2", Value: "value2"},
				{Name: "key2.subkey1.subsubkey1", Value: "value3"},
			}
			result := convertArgoHelmParametersToMap(params)
			Expect(result).To(HaveKey("key1"))
			Expect(result["key1"]).To(HaveKeyWithValue("subkey1", "value1"))
			Expect(result["key1"]).To(HaveKeyWithValue("subkey2", "value2"))
			Expect(result).To(HaveKey("key2"))
			Expect(result["key2"]).To(HaveKey("subkey1"))
			Expect(result["key2"].(map[string]any)["subkey1"]).To(HaveKeyWithValue("subsubkey1", "value3"))
		})
	})

	Context("when the parameters list has mixed nested and non-nested keys", func() {
		It("should return a map with the correct structure", func() {
			params := []argoapi.HelmParameter{
				{Name: "key1", Value: "value1"},
				{Name: "key2.subkey1", Value: "value2"},
				{Name: "key2.subkey2.subsubkey1", Value: "value3"},
			}
			result := convertArgoHelmParametersToMap(params)
			Expect(result).To(HaveKeyWithValue("key1", "value1"))
			Expect(result).To(HaveKey("key2"))
			Expect(result["key2"]).To(HaveKeyWithValue("subkey1", "value2"))
			Expect(result["key2"]).To(HaveKey("subkey2"))
			Expect(result["key2"].(map[string]any)["subkey2"]).To(HaveKeyWithValue("subsubkey1", "value3"))
		})
	})
})

var _ = Describe("newApplicationValues", func() {
	It("should return extraParametersNested YAML with all extra parameters", func() {
		pattern := &api.Pattern{
			Spec: api.PatternSpec{
				ExtraParameters: []api.PatternParameter{
					{Name: "global.extraParam1", Value: "extraValue1"},
					{Name: "global.extraParam2", Value: "extraValue2"},
				},
			},
		}
		result := newApplicationValues(pattern)
		Expect(result).To(ContainSubstring("extraParametersNested:"))
		Expect(result).To(ContainSubstring("global.extraParam1: extraValue1"))
		Expect(result).To(ContainSubstring("global.extraParam2: extraValue2"))
	})

	It("should return only the header when no extra parameters exist", func() {
		pattern := &api.Pattern{
			Spec: api.PatternSpec{
				ExtraParameters: []api.PatternParameter{},
			},
		}
		result := newApplicationValues(pattern)
		Expect(result).To(Equal("extraParametersNested:\n"))
	})

	It("should handle a single extra parameter", func() {
		pattern := &api.Pattern{
			Spec: api.PatternSpec{
				ExtraParameters: []api.PatternParameter{
					{Name: "key", Value: "value"},
				},
			},
		}
		result := newApplicationValues(pattern)
		Expect(result).To(Equal("extraParametersNested:\n  key: value\n"))
	})
})

var _ = Describe("removeApplication", func() {
	Context("when the application exists", func() {
		It("should delete the application without error", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "openshift-gitops",
				},
			}
			argoClient := argoclient.NewSimpleClientset(app)

			err := removeApplication(argoClient, "test-app", "openshift-gitops")
			Expect(err).ToNot(HaveOccurred())

			// Verify the application is gone
			_, err = argoClient.ArgoprojV1alpha1().Applications("openshift-gitops").Get(
				context.Background(), "test-app", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("when the application does not exist", func() {
		It("should return an error", func() {
			argoClient := argoclient.NewSimpleClientset()
			err := removeApplication(argoClient, "nonexistent", "openshift-gitops")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("newArgoGiteaApplication", func() {
	var pattern *api.Pattern

	BeforeEach(func() {
		tmpFalse := false
		pattern = &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pattern",
				Namespace: "default",
			},
			Spec: api.PatternSpec{
				ClusterGroupName: "hub",
				GitConfig: api.GitConfig{
					TargetRepo:     "https://github.com/test/repo",
					TargetRevision: "main",
				},
				GitOpsConfig: &api.GitOpsConfig{
					ManualSync: false,
				},
				MultiSourceConfig: api.MultiSourceConfig{
					Enabled: &tmpFalse,
				},
			},
			Status: api.PatternStatus{
				AppClusterDomain: "apps.example.com",
				ClusterPlatform:  "AWS",
				ClusterVersion:   "4.14.0",
			},
		}
		PatternsOperatorConfig = GitOpsConfig{}
	})

	It("should create the Gitea application with correct name", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app.Name).To(Equal(GiteaApplicationName))
		Expect(app.Namespace).To(Equal(getClusterWideArgoNamespace()))
	})

	It("should set the pattern label", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app.Labels).To(HaveKeyWithValue("validatedpatterns.io/pattern", "test-pattern"))
	})

	It("should set the destination namespace to GiteaNamespace", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app.Spec.Destination.Namespace).To(Equal(GiteaNamespace))
	})

	It("should set destination to in-cluster", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app.Spec.Destination.Name).To(Equal("in-cluster"))
	})

	It("should set the project to default", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app.Spec.Project).To(Equal("default"))
	})

	It("should include helm parameters for gitea admin secret and console href", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app.Spec.Source).ToNot(BeNil())
		Expect(app.Spec.Source.Helm).ToNot(BeNil())

		params := app.Spec.Source.Helm.Parameters
		paramMap := make(map[string]string)
		for _, p := range params {
			paramMap[p.Name] = p.Value
		}
		Expect(paramMap).To(HaveKeyWithValue("gitea.admin.existingSecret", GiteaAdminSecretName))
		Expect(paramMap).To(HaveKey("gitea.console.href"))
		Expect(paramMap["gitea.console.href"]).To(ContainSubstring("apps.example.com"))
		Expect(paramMap).To(HaveKey("gitea.config.server.ROOT_URL"))
	})

	It("should have the foreground propagation finalizer", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(controllerutil.ContainsFinalizer(app, argoapi.ForegroundPropagationPolicyFinalizer)).To(BeTrue())
	})

	It("should set a sync policy when not manual sync", func() {
		app := newArgoGiteaApplication(pattern)
		Expect(app.Spec.SyncPolicy).ToNot(BeNil())
		Expect(app.Spec.SyncPolicy.Automated).ToNot(BeNil())
	})

	It("should have nil sync policy when manual sync is enabled", func() {
		pattern.Spec.GitOpsConfig.ManualSync = true
		app := newArgoGiteaApplication(pattern)
		Expect(app.Spec.SyncPolicy).To(BeNil())
	})
})

var _ = Describe("newArgoCD", func() {
	It("should create an ArgoCD with the correct name and namespace", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Name).To(Equal("test-argo"))
		Expect(argo.Namespace).To(Equal("test-ns"))
	})

	It("should have the argoproj.io/finalizer", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Finalizers).To(ContainElement("argoproj.io/finalizer"))
	})

	It("should have HA disabled", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.HA.Enabled).To(BeFalse())
	})

	It("should have monitoring disabled", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.Monitoring.Enabled).To(BeFalse())
	})

	It("should have notifications disabled", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.Notifications.Enabled).To(BeFalse())
	})

	It("should have SSO configured with Dex provider", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.SSO).ToNot(BeNil())
		Expect(argo.Spec.SSO.Provider).To(Equal(argooperator.SSOProviderTypeDex))
		Expect(argo.Spec.SSO.Dex).ToNot(BeNil())
		Expect(argo.Spec.SSO.Dex.OpenShiftOAuth).To(BeTrue())
	})

	It("should have server route enabled with reencrypt TLS", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.Server.Route.Enabled).To(BeTrue())
		Expect(argo.Spec.Server.Route.TLS).ToNot(BeNil())
		Expect(argo.Spec.Server.Route.TLS.Termination).To(Equal(routev1.TLSTerminationReencrypt))
	})

	It("should have resource exclusions for tekton", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.ResourceExclusions).To(ContainSubstring("tekton.dev"))
		Expect(argo.Spec.ResourceExclusions).To(ContainSubstring("TaskRun"))
		Expect(argo.Spec.ResourceExclusions).To(ContainSubstring("PipelineRun"))
	})

	It("should have resource health checks for Subscription", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.ResourceHealthChecks).To(HaveLen(1))
		Expect(argo.Spec.ResourceHealthChecks[0].Group).To(Equal("operators.coreos.com"))
		Expect(argo.Spec.ResourceHealthChecks[0].Kind).To(Equal("Subscription"))
	})

	It("should have init containers for CA cert fetching", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.Repo.InitContainers).To(HaveLen(1))
		Expect(argo.Spec.Repo.InitContainers[0].Name).To(Equal("fetch-ca"))
	})

	It("should have correct RBAC policy", func() {
		argo := newArgoCD("test-argo", "test-ns")
		Expect(argo.Spec.RBAC.Policy).ToNot(BeNil())
		Expect(*argo.Spec.RBAC.Policy).To(ContainSubstring("cluster-admins"))
	})
})

var _ = Describe("commonSyncPolicy", func() {
	It("should return automated sync policy when not deleting and not manual", func() {
		pattern := &api.Pattern{
			Spec: api.PatternSpec{
				GitOpsConfig: &api.GitOpsConfig{ManualSync: false},
			},
		}
		policy := commonSyncPolicy(pattern)
		Expect(policy).ToNot(BeNil())
		Expect(policy.Automated).ToNot(BeNil())
		Expect(policy.Automated.Prune).To(BeFalse())
	})

	It("should return nil sync policy when manual sync is enabled", func() {
		pattern := &api.Pattern{
			Spec: api.PatternSpec{
				GitOpsConfig: &api.GitOpsConfig{ManualSync: true},
			},
		}
		policy := commonSyncPolicy(pattern)
		Expect(policy).To(BeNil())
	})
})

var _ = Describe("applicationName", func() {
	It("should return pattern name combined with cluster group name", func() {
		pattern := &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "my-pattern"},
			Spec:       api.PatternSpec{ClusterGroupName: "hub"},
		}
		Expect(applicationName(pattern)).To(Equal("my-pattern-hub"))
	})

	It("should handle different cluster group names", func() {
		pattern := &api.Pattern{
			ObjectMeta: metav1.ObjectMeta{Name: "industrial-edge"},
			Spec:       api.PatternSpec{ClusterGroupName: "factory"},
		}
		Expect(applicationName(pattern)).To(Equal("industrial-edge-factory"))
	})
})

var _ = Describe("syncApplication", func() {
	Context("when no sync is in progress", func() {
		It("should set the sync operation with prune", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "openshift-gitops",
				},
			}
			argoFakeClient := argoclient.NewSimpleClientset(app)

			err := syncApplication(argoFakeClient, app, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(app.Operation).ToNot(BeNil())
			Expect(app.Operation.Sync).ToNot(BeNil())
			Expect(app.Operation.Sync.Prune).To(BeTrue())
			Expect(app.Operation.Sync.SyncOptions).To(ContainElement("Force=true"))
		})

		It("should set the sync operation without prune", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "openshift-gitops",
				},
			}
			argoFakeClient := argoclient.NewSimpleClientset(app)

			err := syncApplication(argoFakeClient, app, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(app.Operation).ToNot(BeNil())
			Expect(app.Operation.Sync.Prune).To(BeFalse())
		})
	})

	Context("when a matching sync is already in progress", func() {
		It("should return nil without updating", func() {
			app := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "openshift-gitops",
				},
				Operation: &argoapi.Operation{
					Sync: &argoapi.SyncOperation{
						Prune:       true,
						SyncOptions: []string{"Force=true"},
					},
				},
			}
			argoFakeClient := argoclient.NewSimpleClientset(app)

			err := syncApplication(argoFakeClient, app, true)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("getChildApplications", func() {
	Context("when child applications exist", func() {
		It("should return the child applications", func() {
			parentApp := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-app",
					Namespace: "openshift-gitops",
				},
			}
			childApp := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "child-app",
					Namespace: "openshift-gitops",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "parent-app",
					},
				},
			}
			argoFakeClient := argoclient.NewSimpleClientset(parentApp, childApp)

			children, err := getChildApplications(argoFakeClient, parentApp)
			Expect(err).ToNot(HaveOccurred())
			Expect(children).To(HaveLen(1))
			Expect(children[0].Name).To(Equal("child-app"))
		})
	})

	Context("when no child applications exist", func() {
		It("should return an empty list", func() {
			parentApp := &argoapi.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-app",
					Namespace: "openshift-gitops",
				},
			}
			argoFakeClient := argoclient.NewSimpleClientset(parentApp)

			children, err := getChildApplications(argoFakeClient, parentApp)
			Expect(err).ToNot(HaveOccurred())
			Expect(children).To(BeEmpty())
		})
	})
})
