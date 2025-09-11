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
	"log"
	"os"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argooperator "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	argoapi "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
)

// Which ArgoCD objects we're creating
const (
	ArgoCDGroup    = "argoproj.io"
	ArgoCDVersion  = "v1beta1"
	ArgoCDResource = "argocds"
)

func newArgoCD(name, namespace string) *argooperator.ArgoCD {
	argoPolicy := `g, system:cluster-admins, role:admin
g, cluster-admins, role:admin
g, admin, role:admin`
	defaultPolicy := "role:readonly"
	argoScopes := "[groups,email]"
	trueBool := true
	initVolumes := []v1.Volume{
		{
			Name: "kube-root-ca",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "kube-root-ca.crt",
					},
				},
			},
		},
		{
			Name: "trusted-ca-bundle",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "trusted-ca-bundle",
					},
					Optional: &trueBool,
				},
			},
		},
		{
			Name: "ca-bundles",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}
	initVolumeMounts := []v1.VolumeMount{
		{
			Name:      "ca-bundles",
			MountPath: "/etc/pki/tls/certs",
		},
	}

	initContainers := []v1.Container{
		{
			Name:  "fetch-ca",
			Image: "registry.redhat.io/ubi9/ubi-minimal:latest",
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "kube-root-ca",
					MountPath: "/var/run/kube-root-ca", // ca.crt field
				},
				{
					Name:      "trusted-ca-bundle",
					MountPath: "/var/run/trusted-ca", // ca-bundle.crt field
				},
				{
					Name:      "ca-bundles",
					MountPath: "/tmp/ca-bundles",
				},
			},
			Command: []string{
				"bash",
				"-c",
				"cat /var/run/kube-root-ca/ca.crt /var/run/trusted-ca/ca-bundle.crt > /tmp/ca-bundles/ca-bundle.crt || true",
			},
		},
	}

	s := argooperator.ArgoCD{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ArgoCD",
			APIVersion: "argoproj.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Finalizers: []string{"argoproj.io/finalizer"},
		},
		Spec: argooperator.ArgoCDSpec{
			ApplicationSet: &argooperator.ArgoCDApplicationSet{
				Resources: &v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("250m"),
						v1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				WebhookServer: argooperator.WebhookServerSpec{
					Ingress: argooperator.ArgoCDIngressSpec{
						Enabled: false,
					},
					Route: argooperator.ArgoCDRouteSpec{
						Enabled: false,
					},
				},
			},

			Controller: argooperator.ArgoCDApplicationControllerSpec{
				Resources: &v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("2Gi"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("250m"),
						v1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
			Grafana: argooperator.ArgoCDGrafanaSpec{
				Enabled: false,
				Ingress: argooperator.ArgoCDIngressSpec{
					Enabled: false,
				},
				Route: argooperator.ArgoCDRouteSpec{
					Enabled: false,
				},
				Resources: &v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("250m"),
						v1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			HA: argooperator.ArgoCDHASpec{
				Enabled: false,
				Resources: &v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("250m"),
						v1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			Monitoring: argooperator.ArgoCDMonitoringSpec{
				Enabled: false,
			},
			Notifications: argooperator.ArgoCDNotifications{
				Enabled: false,
			},
			Prometheus: argooperator.ArgoCDPrometheusSpec{
				Enabled: false,
				Ingress: argooperator.ArgoCDIngressSpec{
					Enabled: false,
				},
				Route: argooperator.ArgoCDRouteSpec{
					Enabled: false,
				},
			},
			RBAC: argooperator.ArgoCDRBACSpec{
				DefaultPolicy: &defaultPolicy,
				Policy:        &argoPolicy,
				Scopes:        &argoScopes,
			},
			Redis: argooperator.ArgoCDRedisSpec{
				Resources: &v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("250m"),
						v1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			Repo: argooperator.ArgoCDRepoSpec{
				Resources: &v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("1"),
						v1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("250m"),
						v1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
				InitContainers: initContainers,
				VolumeMounts:   initVolumeMounts,
				Volumes:        initVolumes,
			},
			ResourceExclusions: `- apiGroups:
  - tekton.dev
  clusters:
  - '*'
  kinds:
  - TaskRun
  - PipelineRun`,
			Server: argooperator.ArgoCDServerSpec{
				Autoscale: argooperator.ArgoCDServerAutoscaleSpec{
					Enabled: false,
				},
				GRPC: argooperator.ArgoCDServerGRPCSpec{
					Ingress: argooperator.ArgoCDIngressSpec{
						Enabled: false,
					},
				},
				Ingress: argooperator.ArgoCDIngressSpec{
					Enabled: false,
				},
				Resources: &v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("125m"),
						v1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				Route: argooperator.ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationReencrypt,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
				Service: argooperator.ArgoCDServerServiceSpec{
					Type: "",
				},
			},
			SSO: &argooperator.ArgoCDSSOSpec{
				Dex: &argooperator.ArgoCDDexSpec{
					OpenShiftOAuth: true,
					Resources: &v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("500m"),
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("250m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
				Provider: argooperator.SSOProviderTypeDex,
			},
		},
	}
	return &s
}

func haveArgo(cl client.Client, name, namespace string) (bool, error) {
	// Using an unstructured object.
	unstructuredArgo := &unstructured.Unstructured{}
	unstructuredArgo.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   ArgoCDGroup,
		Kind:    ArgoCDResource,
		Version: ArgoCDVersion,
	})
	err := cl.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, unstructuredArgo)

	if err != nil {
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func createOrUpdateArgoCD(cl client.Client, name, namespace string) error {
	argo := newArgoCD(name, namespace)
	var err error

	foundArgo, err := haveArgo(cl, name, namespace)
	if err != nil {
		return fmt.Errorf("cannot find a sufficiently recent argocd crd version: %v", err)
	}

	if !foundArgo {
		// create it
		obj, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(argo)
		newArgo := &unstructured.Unstructured{Object: obj}

		// Using a unstructured object.

		newArgo.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   ArgoCDGroup,
			Kind:    ArgoCDResource,
			Version: ArgoCDVersion,
		})

		err = cl.Create(context.Background(), newArgo)
	} else { // update it
		oldArgo, _ := getArgoCD(cl, name, namespace)
		argo.SetResourceVersion(oldArgo.GetResourceVersion())
		obj, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(argo)
		newArgo := &unstructured.Unstructured{Object: obj}
		newArgo.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   ArgoCDGroup,
			Kind:    ArgoCDResource,
			Version: ArgoCDVersion,
		})
		err = cl.Update(context.Background(), newArgo)
	}
	return err
}

func getArgoCD(cl client.Client, name, namespace string) (*argooperator.ArgoCD, error) {
	argo := &argooperator.ArgoCD{}

	// Using an unstructured object.
	unstructuredArgo := &unstructured.Unstructured{}
	unstructuredArgo.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   ArgoCDGroup,
		Kind:    ArgoCDResource,
		Version: ArgoCDVersion,
	})
	err := cl.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, unstructuredArgo)

	if err != nil {
		return nil, err
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredArgo.UnstructuredContent(), argo)
	return argo, err
}

func newApplicationParameters(p *api.Pattern) []argoapi.HelmParameter {
	parameters := []argoapi.HelmParameter{
		{
			Name:  "global.pattern",
			Value: p.Name,
		},
		{
			Name:  "global.namespace",
			Value: p.Namespace,
		},
		{
			Name:  "global.repoURL",
			Value: p.Spec.GitConfig.TargetRepo,
		},
		{
			Name:  "global.originURL",
			Value: p.Spec.GitConfig.OriginRepo,
		},
		{
			Name:  "global.targetRevision",
			Value: p.Spec.GitConfig.TargetRevision,
		},
		{
			Name:  "global.hubClusterDomain",
			Value: p.Status.AppClusterDomain,
		},
		{
			Name:  "global.localClusterDomain",
			Value: p.Status.AppClusterDomain,
		},
		{
			Name:  "global.clusterDomain",
			Value: p.Status.ClusterDomain,
		},
		{
			Name:  "global.clusterVersion",
			Value: p.Status.ClusterVersion,
		},
		{
			Name:  "global.clusterPlatform",
			Value: p.Status.ClusterPlatform,
		},
		{
			Name:  "global.localClusterName",
			Value: p.Status.ClusterName,
		},
		{
			Name:  "global.privateRepo",
			Value: strconv.FormatBool(p.Spec.GitConfig.TokenSecret != ""),
		},
		{
			Name:  "global.multiSourceSupport",
			Value: strconv.FormatBool(*p.Spec.MultiSourceConfig.Enabled),
		},
		{
			Name:  "global.multiSourceRepoUrl",
			Value: p.Spec.MultiSourceConfig.HelmRepoUrl,
		},

		{
			Name:  "global.experimentalCapabilities",
			Value: p.Spec.ExperimentalCapabilities,
		},
	}
	parameters = append(parameters, argoapi.HelmParameter{
		Name:  "global.multiSourceTargetRevision",
		Value: getClusterGroupChartVersion(p),
	})
	for _, extra := range p.Spec.ExtraParameters {
		if !updateHelmParameter(extra, parameters) {
			log.Printf("Parameter %q = %q added", extra.Name, extra.Value)
			parameters = append(parameters, argoapi.HelmParameter{
				Name:  extra.Name,
				Value: extra.Value,
			})
		}
	}
	if !p.DeletionTimestamp.IsZero() {
		parameters = append(parameters, argoapi.HelmParameter{
			Name:        "global.deletePattern",
			Value:       "1",
			ForceString: true,
		})
	}
	return parameters
}

func convertArgoHelmParametersToMap(params []argoapi.HelmParameter) map[string]any {
	result := make(map[string]any)

	for _, p := range params {
		keys := strings.Split(p.Name, ".")
		lastKeyIndex := len(keys) - 1

		currentMap := result
		for i, key := range keys {
			if i == lastKeyIndex {
				currentMap[key] = p.Value
			} else {
				if _, ok := currentMap[key]; !ok {
					currentMap[key] = make(map[string]any)
				}
				currentMap = currentMap[key].(map[string]any)
			}
		}
	}
	return result
}

func newApplicationValueFiles(p *api.Pattern, prefix string) []string {
	files := []string{
		fmt.Sprintf("%s/values-global.yaml", prefix),
		fmt.Sprintf("%s/values-%s.yaml", prefix, p.Spec.ClusterGroupName),
		fmt.Sprintf("%s/values-%s.yaml", prefix, p.Status.ClusterPlatform),
		fmt.Sprintf("%s/values-%s-%s.yaml", prefix, p.Status.ClusterPlatform, p.Status.ClusterVersion),
		fmt.Sprintf("%s/values-%s-%s.yaml", prefix, p.Status.ClusterPlatform, p.Spec.ClusterGroupName),
		fmt.Sprintf("%s/values-%s-%s.yaml", prefix, p.Status.ClusterVersion, p.Spec.ClusterGroupName),
		fmt.Sprintf("%s/values-%s.yaml", prefix, p.Status.ClusterName),
	}

	for _, extra := range p.Spec.ExtraValueFiles {
		extraValueFile := fmt.Sprintf("%s/%s", prefix, strings.TrimPrefix(extra, "/"))
		log.Printf("Values file %q added", extraValueFile)
		files = append(files, extraValueFile)
	}
	return files
}

func newApplicationValues(p *api.Pattern) string {
	s := "extraParametersNested:\n"
	for _, extra := range p.Spec.ExtraParameters {
		line := fmt.Sprintf("  %s: %s\n", extra.Name, extra.Value)
		s += line
	}
	return s
}

// Fetches the clusterGroup.sharedValueFiles values from a checked out git repo
//  1. We get all the valueFiles from the pattern
//  2. We parse them and merge them in order
//  3. Then for each element of the sharedValueFiles list we template it via the helm
//     libraries. E.g. a string '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'
//     will be converted to '/overrides/values-AWS.yaml'
//  4. We return the list of templated strings back as an array
func getSharedValueFiles(p *api.Pattern, prefix string) ([]string, error) {
	gitDir := p.Status.LocalCheckoutPath
	if _, err := os.Stat(gitDir); err != nil {
		return nil, fmt.Errorf("%s path does not exist", gitDir)
	}

	valueFiles := newApplicationValueFiles(p, gitDir)

	helmValues, err := mergeHelmValues(valueFiles...)
	if err != nil {
		return nil, fmt.Errorf("could not fetch value files: %s", err)
	}
	sharedValueFiles := getClusterGroupValue("sharedValueFiles", helmValues)
	if sharedValueFiles == nil {
		return nil, nil
	}

	// Check if s is of type []interface{}
	val, ok := sharedValueFiles.([]any)
	if !ok {
		return nil, fmt.Errorf("could not make a list out of sharedValueFiles: %v", sharedValueFiles)
	}

	// Convert each element of slice to a string
	stringSlice := make([]string, len(val))
	for i, v := range val {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("type assertion failed at index %d: Not a string", i)
		}
		valueMap := convertArgoHelmParametersToMap(newApplicationParameters(p))
		templatedString, err := helmTpl(str, valueFiles, valueMap)

		// we only log an error, but try to keep going
		if err != nil {
			log.Printf("Failed to render templated string %s: %v", str, err)
			continue
		}
		if strings.HasPrefix(templatedString, "/") {
			stringSlice[i] = fmt.Sprintf("%s%s", prefix, templatedString)
		} else {
			stringSlice[i] = fmt.Sprintf("%s/%s", prefix, templatedString)
		}
	}

	return stringSlice, nil
}

func commonSyncPolicy(p *api.Pattern) *argoapi.SyncPolicy {
	var syncPolicy *argoapi.SyncPolicy
	if !p.DeletionTimestamp.IsZero() {
		syncPolicy = &argoapi.SyncPolicy{
			// Automated will keep an application synced to the target revision
			Automated: &argoapi.SyncPolicyAutomated{
				Prune: true,
			},
			// Options allow you to specify whole app sync-SyncOptions
			SyncOptions: []string{"Prune=true"},
		}
	} else if !p.Spec.GitOpsConfig.ManualSync {
		// SyncPolicy controls when and how a sync will be performed
		syncPolicy = &argoapi.SyncPolicy{
			// Automated will keep an application synced to the target revision
			Automated: &argoapi.SyncPolicyAutomated{},
			// Options allow you to specify whole app sync-options
			SyncOptions: []string{},
			// Retry controls failed sync retry behavior
			// Retry *RetryStrategy `json:"retry,omitempty" protobuf:"bytes,3,opt,name=retry"`
		}
	}
	return syncPolicy
}

func commonApplicationSpec(p *api.Pattern, sources []argoapi.ApplicationSource) *argoapi.ApplicationSpec {
	spec := &argoapi.ApplicationSpec{
		Destination: argoapi.ApplicationDestination{
			Name:      "in-cluster",
			Namespace: p.Namespace,
		},
		// Project is a reference to the project this application belongs to.
		// The empty string means that application belongs to the 'default' project.
		Project: "default",

		// IgnoreDifferences is a list of resources and their fields which should be ignored during comparison
		// IgnoreDifferences []ResourceIgnoreDifferences `json:"ignoreDifferences,omitempty" protobuf:"bytes,5,name=ignoreDifferences"`
		// Info contains a list of information (URLs, email addresses, and plain text) that relates to the application
		// Info []Info `json:"info,omitempty" protobuf:"bytes,6,name=info"`
		// RevisionHistoryLimit limits the number of items kept in the
		// application's revision history, which is used for informational
		// purposes as well as for rollbacks to previous versions.
		// This should only be changed in exceptional circumstances.
		// Setting to zero will store no history. This will reduce storage used.
		// Increasing will increase the space used to store the history, so we do not recommend increasing it.
		// Default is 10.
		// RevisionHistoryLimit *int64 `json:"revisionHistoryLimit,omitempty" protobuf:"bytes,7,name=revisionHistoryLimit"`
	}
	if len(sources) == 1 {
		spec.Source = &sources[0]
	} else {
		spec.Sources = sources
	}
	return spec
}

func commonApplicationSourceHelm(p *api.Pattern, prefix string) *argoapi.ApplicationSourceHelm {
	valueFiles := newApplicationValueFiles(p, prefix)
	sharedValueFiles, err := getSharedValueFiles(p, prefix)
	if err != nil {
		fmt.Printf("Could not fetch sharedValueFiles: %s", err)
	}
	valueFiles = append(valueFiles, sharedValueFiles...)

	return &argoapi.ApplicationSourceHelm{
		ValueFiles: valueFiles,

		// Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation
		Parameters: newApplicationParameters(p),

		// This is to be able to pass down the extraParams to the single applications
		Values: newApplicationValues(p),
		// ReleaseName is the Helm release name to use. If omitted it will use the application name
		// ReleaseName string `json:"releaseName,omitempty" protobuf:"bytes,3,opt,name=releaseName"`
		// Values specifies Helm values to be passed to helm template, typically defined as a block
		// Values string `json:"values,omitempty" protobuf:"bytes,4,opt,name=values"`
		// FileParameters are file parameters to the helm template
		// FileParameters []HelmFileParameter `json:"fileParameters,omitempty" protobuf:"bytes,5,opt,name=fileParameters"`
		// Version is the Helm version to use for templating (either "2" or "3")
		// Version string `json:"version,omitempty" protobuf:"bytes,6,opt,name=version"`
		// PassCredentials pass credentials to all domains (Helm's --pass-credentials)
		// PassCredentials bool `json:"passCredentials,omitempty" protobuf:"bytes,7,opt,name=passCredentials"`
		// IgnoreMissingValueFiles prevents helm template from failing when valueFiles do not exist locally by not appending them to helm template --values
		// Only applies to local files
		IgnoreMissingValueFiles: true,
		// SkipCrds skips custom resource definition installation step (Helm's --skip-crds)
		// SkipCrds bool `json:"skipCrds,omitempty" protobuf:"bytes,9,opt,name=skipCrds"`
	}
}

func newArgoOperatorApplication(p *api.Pattern, spec *argoapi.ApplicationSpec) *argoapi.Application {
	labels := make(map[string]string)
	labels["validatedpatterns.io/pattern"] = p.Name
	app := argoapi.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationName(p),
			Namespace: getClusterWideArgoNamespace(),
			Labels:    labels,
		},
		Spec: *spec,
	}
	controllerutil.AddFinalizer(&app, argoapi.ForegroundPropagationPolicyFinalizer)
	return &app
}

func newSourceApplication(p *api.Pattern) *argoapi.Application {
	// Argo uses...
	// r := regexp.MustCompile("(/|:)")
	// root := filepath.Join(os.TempDir(), r.ReplaceAllString(NormalizeGitURL(rawRepoURL), "_"))
	// Source is a reference to the location of the application's manifests or chart
	source := argoapi.ApplicationSource{
		RepoURL:        p.Spec.GitConfig.TargetRepo,
		Path:           "common/clustergroup",
		TargetRevision: p.Spec.GitConfig.TargetRevision,
		Helm:           commonApplicationSourceHelm(p, ""),
	}
	spec := commonApplicationSpec(p, []argoapi.ApplicationSource{source})

	spec.SyncPolicy = commonSyncPolicy(p)
	return newArgoOperatorApplication(p, spec)
}

func newMultiSourceApplication(p *api.Pattern) *argoapi.Application {
	sources := []argoapi.ApplicationSource{}
	var baseSource *argoapi.ApplicationSource

	valuesSource := &argoapi.ApplicationSource{
		RepoURL:        p.Spec.GitConfig.TargetRepo,
		TargetRevision: p.Spec.GitConfig.TargetRevision,
		Ref:            "patternref",
	}
	sources = append(sources, *valuesSource)

	// If we do not specify a custom repo for the clustergroup chart, let's use the default
	// clustergroup chart from the helm repo url. Otherwise use the git repo that was given
	if p.Spec.MultiSourceConfig.ClusterGroupGitRepoUrl == "" {
		// If the user set the clustergroupchart version use that

		baseSource = &argoapi.ApplicationSource{
			RepoURL:        p.Spec.MultiSourceConfig.HelmRepoUrl,
			Chart:          "clustergroup",
			TargetRevision: getClusterGroupChartVersion(p),
			Helm:           commonApplicationSourceHelm(p, "$patternref"),
		}
	} else {
		baseSource = &argoapi.ApplicationSource{
			RepoURL:        p.Spec.MultiSourceConfig.ClusterGroupGitRepoUrl,
			Path:           ".",
			TargetRevision: p.Spec.MultiSourceConfig.ClusterGroupChartGitRevision,
			Helm:           commonApplicationSourceHelm(p, "$patternref"),
		}
	}
	sources = append(sources, *baseSource)

	spec := commonApplicationSpec(p, sources)
	spec.SyncPolicy = commonSyncPolicy(p)
	return newArgoOperatorApplication(p, spec)
}

func getClusterGroupChartVersion(p *api.Pattern) string {
	var clusterGroupChartVersion string
	if p.Spec.MultiSourceConfig.ClusterGroupChartVersion != "" {
		clusterGroupChartVersion = p.Spec.MultiSourceConfig.ClusterGroupChartVersion
	} else { // if the user has not specified anything, then let's detect if common is slimmed
		if IsCommonSlimmed(p.Status.LocalCheckoutPath) {
			clusterGroupChartVersion = "0.9.*"
		} else {
			clusterGroupChartVersion = "0.8.*"
		}
	}
	return clusterGroupChartVersion
}

func newArgoApplication(p *api.Pattern) *argoapi.Application {
	// -- ArgoCD Application
	var targetApp *argoapi.Application

	if *p.Spec.MultiSourceConfig.Enabled {
		targetApp = newMultiSourceApplication(p)
	} else {
		targetApp = newSourceApplication(p)
	}

	return targetApp
}

func newArgoGiteaApplication(p *api.Pattern) *argoapi.Application {
	consoleHref := fmt.Sprintf("https://%s-%s.%s", GiteaRouteName, GiteaNamespace, p.Status.AppClusterDomain)
	parameters := []argoapi.HelmParameter{
		{
			Name:  "gitea.admin.existingSecret",
			Value: GiteaAdminSecretName,
		},
		{
			Name:  "gitea.console.href",
			Value: consoleHref,
		},
		{
			Name:  "gitea.config.server.ROOT_URL",
			Value: consoleHref,
		},
	}
	spec := &argoapi.ApplicationSpec{
		Destination: argoapi.ApplicationDestination{
			Name:      "in-cluster",
			Namespace: GiteaNamespace,
		},
		Project: "default",
		Source: &argoapi.ApplicationSource{
			RepoURL:        PatternsOperatorConfig.getValueWithDefault("gitea.helmRepoUrl"),
			TargetRevision: PatternsOperatorConfig.getValueWithDefault("gitea.chartVersion"),
			Chart:          PatternsOperatorConfig.getValueWithDefault("gitea.chartName"),
			Helm: &argoapi.ApplicationSourceHelm{
				Parameters: parameters,
			},
		},
		SyncPolicy: commonSyncPolicy(p),
	}
	labels := make(map[string]string)
	labels["validatedpatterns.io/pattern"] = p.Name
	app := argoapi.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GiteaApplicationName,
			Namespace: getClusterWideArgoNamespace(),
			Labels:    labels,
		},
		Spec: *spec,
	}
	controllerutil.AddFinalizer(&app, argoapi.ForegroundPropagationPolicyFinalizer)
	return &app
}

func countVPApplications(p *api.Pattern) (appCount, appSetsCount int, err error) {
	gitDir := p.Status.LocalCheckoutPath
	if _, err := os.Stat(gitDir); err != nil {
		return -1, -1, fmt.Errorf("%s path does not exist", gitDir)
	}
	valueFiles := newApplicationValueFiles(p, gitDir)
	helmValues, helmErr := mergeHelmValues(valueFiles...)
	if helmErr != nil {
		return -2, -2, fmt.Errorf("error reading value file: %s", helmErr)
	}

	applicationDict := getClusterGroupValue("applications", helmValues)
	if applicationDict == nil {
		return 0, 0, nil
	}
	apps, appsets := countApplicationsAndSets(applicationDict)
	return apps, appsets, nil
}

func applicationName(p *api.Pattern) string {
	return fmt.Sprintf("%s-%s", p.Name, p.Spec.ClusterGroupName)
}

func getApplication(cl client.Client, name, namespace string) (*argoapi.Application, error) {
	app := &argoapi.Application{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, app)
	if err != nil {
		return nil, err
	}
	return app, nil
}

func createApplication(cl client.Client, app *argoapi.Application, _ string) error {
	err := cl.Create(context.Background(), app)
	yamlOutput, _ := objectYaml(app)
	log.Printf("Created: %s\n", yamlOutput)
	return err
}

func updateApplication(cl client.Client, target, current *argoapi.Application, _ string) (bool, error) {
	if current == nil {
		return false, fmt.Errorf("current application was nil")
	} else if target == nil {
		return false, fmt.Errorf("target application was nil")
	}
	if current.Spec.Sources == nil {
		if compareSource(target.Spec.Source, current.Spec.Source) {
			return false, nil
		}
	} else {
		if compareSources(target.Spec.Sources, current.Spec.Sources) {
			return false, nil
		}
	}

	spec := current.Spec.DeepCopy()
	target.Spec.DeepCopyInto(spec)
	current.Spec = *spec
	if err := cl.Update(context.Background(), current); err != nil {
		return false, fmt.Errorf("could not update application: %w", err)
	}
	return true, nil
}

func removeApplication(cl client.Client, app *argoapi.Application) error {
	return cl.Delete(context.Background(), app)
}

func compareSource(goal, actual *argoapi.ApplicationSource) bool {
	if goal == nil || actual == nil {
		return false
	}
	if goal.RepoURL != actual.RepoURL {
		log.Printf("RepoURL changed %s -> %s\n", actual.RepoURL, goal.RepoURL)
		return false
	}

	if goal.TargetRevision != actual.TargetRevision {
		log.Printf("TargetRevision changed %s -> %s\n", actual.TargetRevision, goal.TargetRevision)
		return false
	}

	if goal.Path != actual.Path {
		log.Printf("Path changed %s -> %s\n", actual.Path, goal.Path)
		return false
	}

	// if both .Helm structs are nil, we compared everything already and we can just
	// return true here without invoking compareHelmSource()
	if goal.Helm == nil && actual.Helm == nil {
		return true
	}
	// but if one .Helm struct is nil and the other one is not then we can safely return false
	if goal.Helm == nil || actual.Helm == nil {
		return false
	}

	return compareHelmSource(goal.Helm, actual.Helm)
}

func compareSources(goal, actual argoapi.ApplicationSources) bool {
	if actual == nil || goal == nil {
		return false
	}
	if len(actual) != len(goal) {
		return false
	}
	if len(actual) == 0 || len(goal) == 0 {
		return false
	}
	for i := range actual {
		// avoids memory aliasing (the iteration variable is reused, so v changes but &v is always the same)
		value := actual[i]
		if !compareSource(&value, &goal[i]) {
			return false
		}
	}
	return true
}

func compareHelmSource(goal, actual *argoapi.ApplicationSourceHelm) bool {
	if !compareHelmValueFiles(goal.ValueFiles, actual.ValueFiles) {
		return false
	}
	if !compareHelmParameters(goal.Parameters, actual.Parameters) {
		return false
	}
	return true
}

func compareHelmParameter(goal argoapi.HelmParameter, actual []argoapi.HelmParameter) bool {
	for _, param := range actual {
		if goal.Name == param.Name {
			if goal.Value == param.Value {
				return true
			}
			log.Printf("Parameter %q changed: %q -> %q", goal.Name, param.Value, goal.Value)
			return false
		}
	}
	log.Printf("Parameter %q not found", goal.Name)
	return false
}

func compareHelmParameters(goal, actual []argoapi.HelmParameter) bool {
	if len(goal) != len(actual) {
		return false
	}

	for _, gP := range goal {
		if !compareHelmParameter(gP, actual) {
			return false
		}
	}
	return true
}

func compareHelmValueFile(goal string, actual []string) bool {
	for _, value := range actual {
		if goal == value {
			return true
		}
	}
	log.Printf("Values file %q not found", goal)
	return false
}

func compareHelmValueFiles(goal, actual []string) bool {
	if len(goal) != len(actual) {
		return false
	}
	for _, gV := range goal {
		if !compareHelmValueFile(gV, actual) {
			return false
		}
	}
	return true
}

func updateHelmParameter(goal api.PatternParameter, actual []argoapi.HelmParameter) bool {
	for _, param := range actual {
		if goal.Name == param.Name {
			if goal.Value == param.Value {
				return true
			}
			log.Printf("Parameter %q updated: %q -> %q", goal.Name, param.Value, goal.Value)
			param.Value = goal.Value
			return true
		}
	}
	return false
}
