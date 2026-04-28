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
	"slices"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argooperator "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	argoapi "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoclient "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
)

// Which ArgoCD objects we're creating
const (
	ArgoCDGroup    = "argoproj.io"
	ArgoCDVersion  = "v1beta1"
	ArgoCDResource = "argocds"
)

// ConsoleLink constants
const (
	ConsoleLinkGroup    = "console.openshift.io"
	ConsoleLinkVersion  = "v1"
	ConsoleLinkResource = "consolelinks"
)

func newArgoCD(name, namespace string, patternsOperatorConfig PatternsOperatorConfig) *argooperator.ArgoCD {
	argoPolicies := []string{
		"g, system:cluster-admins, role:admin",
		"g, cluster-admins, role:admin",
		"g, admin, role:admin",
	}
	for argoAdmin := range strings.SplitSeq(patternsOperatorConfig.getValueWithDefault("gitops.additionalArgoAdmins"), ",") {
		argoAdmin = strings.TrimSpace(argoAdmin)
		if argoAdmin != "" {
			argoPolicies = append(argoPolicies, "g, "+argoAdmin+", role:admin")
		}
	}
	argoPolicy := strings.Join(argoPolicies, "\n")
	defaultPolicy := "role:readonly"
	argoScopes := "[groups,email]"

	resourceHealthChecks := []argooperator.ResourceHealthCheck{
		{
			// We can drop this custom Subscription healthcheck once https://www.github.com/argoproj/argo-cd/issues/25921 is fixed
			Group: "operators.coreos.com",
			Kind:  "Subscription",
			Check: `local health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local numDegraded = 0
    local numPending = 0
    local msg = ""

    -- Check if this is a manual approval scenario where InstallPlanPending is expected
    -- and the operator is already installed (upgrade pending, not initial install)
    local isManualApprovalPending = false
    if obj.spec ~= nil and obj.spec.installPlanApproval == "Manual" then
      for _, condition in pairs(obj.status.conditions) do
        if condition.type == "InstallPlanPending" and condition.status == "True" and condition.reason == "RequiresApproval" then
          -- Only treat as expected healthy state if the operator is already installed
          -- (installedCSV is present), meaning this is an upgrade pending approval
          if obj.status.installedCSV ~= nil then
            isManualApprovalPending = true
          end
          break
        end
      end
    end

    for i, condition in pairs(obj.status.conditions) do
      -- Skip InstallPlanPending condition when manual approval is pending (expected behavior)
      if isManualApprovalPending and condition.type == "InstallPlanPending" then
        -- Do not include in message or count as pending
      else
        msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. "\n"
        if condition.type == "InstallPlanPending" and condition.status == "True" then
          numPending = numPending + 1
        elseif (condition.type == "InstallPlanMissing" and condition.reason ~= "ReferencedInstallPlanNotFound") then
          numDegraded = numDegraded + 1
        elseif (condition.type == "CatalogSourcesUnhealthy" or condition.type == "InstallPlanFailed" or condition.type == "ResolutionFailed") and condition.status == "True" then
          numDegraded = numDegraded + 1
        end
      end
    end

    -- Available states: undef/nil, UpgradeAvailable, UpgradePending, UpgradeFailed, AtLatestKnown
    -- Source: https://github.com/openshift/operator-framework-olm/blob/5e2c73b7663d0122c9dc3e59ea39e515a31e2719/staging/api/pkg/operators/v1alpha1/subscription_types.go#L17-L23
    if obj.status.state == nil  then
      numPending = numPending + 1
      msg = msg .. ".status.state not yet known\n"
    elseif obj.status.state == "" or obj.status.state == "UpgradeAvailable" then
      numPending = numPending + 1
      msg = msg .. ".status.state is '" .. obj.status.state .. "'\n"
    elseif obj.status.state == "UpgradePending" then
      -- UpgradePending with manual approval is expected behavior, treat as healthy
      if isManualApprovalPending then
        msg = msg .. ".status.state is 'AtLatestKnown'\n"
      else
        numPending = numPending + 1
        msg = msg .. ".status.state is '" .. obj.status.state .. "'\n"
      end
    elseif obj.status.state == "UpgradeFailed" then
      numDegraded = numDegraded + 1
      msg = msg .. ".status.state is '" .. obj.status.state .. "'\n"
    else
      -- Last possiblity of .status.state: AtLatestKnown
      msg =  msg .. ".status.state is '" .. obj.status.state .. "'\n"
    end
 
    if numDegraded == 0 and numPending == 0 then
      health_status.status = "Healthy"
      health_status.message = msg
      return health_status
    elseif numPending > 0 and numDegraded == 0 then
      health_status.status = "Progressing"
      health_status.message = msg
      return health_status
    else
      health_status.status = "Degraded"
      health_status.message = msg
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "An install plan for a subscription is pending installation"
return health_status`,
		},
	}
	if strings.EqualFold(patternsOperatorConfig.getValueWithDefault("gitops.applicationHealthCheckEnabled"), "true") {
		// As of ArgoCD 1.8 the Application health check was dropped (see https://github.com/argoproj/argo-cd/issues/3781),
		// but in app-of-apps pattern this is needed in order to implement children apps dependencies via sync-waves
		resourceHealthChecks = append(resourceHealthChecks, argooperator.ResourceHealthCheck{
			Group: "argoproj.io",
			Kind:  "Application",
			Check: `local health_status = {}
health_status.status = "Progressing"
health_status.message = ""
if obj.status ~= nil then
  if obj.status.health ~= nil then
    -- we consider the Application Healthy only when the health status is Healthy AND it's synced
    if obj.status.health.status == "Healthy" and (obj.status.sync and obj.status.sync.status or nil) == "Synced" then
      health_status.status = "Healthy"
      health_status.message = (obj.status.health.message or "Application is healthy and synced")
      return health_status
    end
	-- We consider the Application Degraded only when the Sync failed for 'retry.limit' times
    if obj.status.operationState ~= nil then
      local retryLimit = (obj.status.operationState.operation and obj.status.operationState.operation.retry and obj.status.operationState.operation.retry.limit or nil)
      local retryCount = (obj.status.operationState.retryCount or nil)
      if retryLimit == retryCount and obj.status.operationState.phase ~= "Succeeded" then
        health_status.status = "Degraded"
        health_status.message = "Retry limit reached and sync didn't succeed"
      end
    end
  end
end
return health_status`,
		})
	}

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
						v1.ResourceMemory: resource.MustParse("8Gi"),
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
			// We can drop this custom Subscription healthcheck once https://www.github.com/argoproj/argo-cd/issues/25921 is fixed
			ResourceHealthChecks:   resourceHealthChecks,
			ResourceTrackingMethod: "annotation",
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

func haveArgo(client dynamic.Interface, name, namespace string) bool {
	gvr := schema.GroupVersionResource{Group: ArgoCDGroup, Version: ArgoCDVersion, Resource: ArgoCDResource}
	_, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	return err == nil
}

func createOrUpdateArgoCD(client dynamic.Interface, fullClient kubernetes.Interface, name, namespace string, patternsOperatorConfig PatternsOperatorConfig) error {
	argo := newArgoCD(name, namespace, patternsOperatorConfig)
	gvr := schema.GroupVersionResource{Group: ArgoCDGroup, Version: ArgoCDVersion, Resource: ArgoCDResource}

	var err error
	// we skip this check if fullClient is explicitly nil for simpler testing
	if fullClient != nil {
		err = checkAPIVersion(fullClient, ArgoCDGroup, ArgoCDVersion)
		if err != nil {
			return fmt.Errorf("cannot find a sufficiently recent argocd crd version: %v", err)
		}
	}

	if !haveArgo(client, name, namespace) {
		// create it
		obj, errConvert := runtime.DefaultUnstructuredConverter.ToUnstructured(argo)
		if errConvert != nil {
			return fmt.Errorf("failed to convert ArgoCD to unstructured for create: %v", errConvert)
		}
		newArgo := &unstructured.Unstructured{Object: obj}
		_, err = client.Resource(gvr).Namespace(namespace).Create(context.TODO(), newArgo, metav1.CreateOptions{})
	} else { // update it
		oldArgo, errGet := getArgoCDFunc(client, name, namespace)
		if errGet != nil {
			return fmt.Errorf("failed to get existing ArgoCD %s/%s: %v", namespace, name, errGet)
		}
		argo.SetResourceVersion(oldArgo.GetResourceVersion())
		obj, errConvert := runtime.DefaultUnstructuredConverter.ToUnstructured(argo)
		if errConvert != nil {
			return fmt.Errorf("failed to convert ArgoCD to unstructured for update: %v", errConvert)
		}
		newArgo := &unstructured.Unstructured{Object: obj}

		_, err = client.Resource(gvr).Namespace(namespace).Update(context.TODO(), newArgo, metav1.UpdateOptions{})
	}
	return err
}

// argocdIconBase64 is the ArgoCD logo used in the OpenShift console application menu
const argocdIconBase64 = "data:image/png;base64,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" //nolint:lll

func consoleLinkGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: ConsoleLinkGroup, Version: ConsoleLinkVersion, Resource: ConsoleLinkResource}
}

func createOrUpdateConsoleLink(client dynamic.Interface, argoName, argoNamespace, appClusterDomain string) error {
	linkName := argoName + "-gitops-link"
	href := fmt.Sprintf("https://%s-server-%s.%s", argoName, argoNamespace, appClusterDomain)

	consoleLinkObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": ConsoleLinkGroup + "/" + ConsoleLinkVersion,
			"kind":       "ConsoleLink",
			"metadata": map[string]any{
				"name": linkName,
			},
			"spec": map[string]any{
				"applicationMenu": map[string]any{
					"section":  "OpenShift GitOps",
					"imageURL": argocdIconBase64,
				},
				"href":     href,
				"location": "ApplicationMenu",
				"text":     "Argo CD VP",
			},
		},
	}

	gvr := consoleLinkGVR()
	existing, err := client.Resource(gvr).Get(context.TODO(), linkName, metav1.GetOptions{})
	if err != nil {
		// Does not exist, create it
		_, err = client.Resource(gvr).Create(context.TODO(), consoleLinkObj, metav1.CreateOptions{})
		return err
	}
	// Update: carry over resourceVersion
	consoleLinkObj.SetResourceVersion(existing.GetResourceVersion())
	_, err = client.Resource(gvr).Update(context.TODO(), consoleLinkObj, metav1.UpdateOptions{})
	return err
}

func removeConsoleLink(client dynamic.Interface, argoName string) error {
	linkName := argoName + "-gitops-link"
	gvr := consoleLinkGVR()
	err := client.Resource(gvr).Delete(context.TODO(), linkName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("Failed to delete ConsoleLink %s (may not exist): %v", linkName, err)
		return err
	}
	return nil
}

var getArgoCDFunc = getArgoCD

func getArgoCD(client dynamic.Interface, name, namespace string) (*argooperator.ArgoCD, error) {
	gvr := schema.GroupVersionResource{Group: ArgoCDGroup, Version: ArgoCDVersion, Resource: ArgoCDResource}
	argo := &argooperator.ArgoCD{}
	unstructuredArgo, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
			Name:  "global.targetPath",
			Value: p.Spec.GitConfig.TargetPath,
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
	_, gitOpsSubNamespace := DetectGitOpsSubscription()
	parameters = append(parameters, argoapi.HelmParameter{
		Name:  "global.gitOpsSubNamespace",
		Value: gitOpsSubNamespace,
	}, argoapi.HelmParameter{
		Name:  "global.vpArgoNamespace",
		Value: getClusterWideArgoNamespace(),
	}, argoapi.HelmParameter{
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
		// Determine deletePattern value based on deletion phase

		// Phase 1: Delete child applications from spoke clusters: DeleteSpokeChildApps
		// Phase 2: Delete app of apps from spoke: DeleteSpoke
		// Phase 3: Delete applications from hub: DeleteHubChildApps
		// Phase 4: Delete app of apps from hub: DeleteHub

		deletePatternValue := p.Status.DeletionPhase // default to the phase on the pattern object

		// If we need to clean up child apps from the hub, we change it (clustergroup chart app creation logic)
		if p.Status.DeletionPhase == api.DeleteHubChildApps {
			deletePatternValue = "DeleteChildApps"
		}
		parameters = append(parameters, argoapi.HelmParameter{
			Name:        "global.deletePattern",
			Value:       string(deletePatternValue),
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
	basePath := p.Spec.GitConfig.TargetPath
	if basePath != "" {
		if prefix != "" {
			prefix = fmt.Sprintf("%s/%s", prefix, basePath)
		} else {
			prefix = basePath
		}
	}
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
			Automated: &argoapi.SyncPolicyAutomated{
				SelfHeal: true,
			},
			// Options allow you to specify whole app sync-options
			SyncOptions: []string{},
			// Retry controls failed sync retry behavior
			Retry: &argoapi.RetryStrategy{
				Limit: 20,
			},
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
		log.Printf("Could not fetch sharedValueFiles: %s", err)
	} else {
		valueFiles = append(valueFiles, sharedValueFiles...)
	}

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
		Path:           p.Spec.GitConfig.TargetPath,
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

func newArgoGiteaApplication(p *api.Pattern, patternsOperatorConfig PatternsOperatorConfig) *argoapi.Application {
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
			RepoURL:        patternsOperatorConfig.getValueWithDefault("gitea.helmRepoUrl"),
			TargetRevision: patternsOperatorConfig.getValueWithDefault("gitea.chartVersion"),
			Chart:          patternsOperatorConfig.getValueWithDefault("gitea.chartName"),
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

func getApplication(client argoclient.Interface, name, namespace string) (*argoapi.Application, error) {
	if app, err := client.ArgoprojV1alpha1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	} else {
		return app, nil
	}
}

func createApplication(client argoclient.Interface, app *argoapi.Application, namespace string) error {
	saved, err := client.ArgoprojV1alpha1().Applications(namespace).Create(context.Background(), app, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	yamlOutput, _ := objectYaml(saved)
	log.Printf("Created: %s\n", yamlOutput)
	return nil
}

func updateApplication(client argoclient.Interface, target, current *argoapi.Application, namespace string) (bool, error) {
	if current == nil {
		return false, fmt.Errorf("current application was nil")
	} else if target == nil {
		return false, fmt.Errorf("target application was nil")
	}
	if compareApplication(current, target) {
		return false, nil
	}

	spec := current.Spec.DeepCopy()
	target.Spec.DeepCopyInto(spec)
	current.Spec = *spec

	_, err := client.ArgoprojV1alpha1().Applications(namespace).Update(context.Background(), current, metav1.UpdateOptions{})
	return true, err
}

func removeApplication(client argoclient.Interface, name, namespace string) error {
	return client.ArgoprojV1alpha1().Applications(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func compareApplication(goal, actual *argoapi.Application) bool {
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
		return false
	}
	if !compareSource(goal.Spec.Source, actual.Spec.Source) {
		return false
	}
	if !compareSources(goal.Spec.Sources, actual.Spec.Sources) {
		return false
	}
	if !compareSyncPolicy(goal.Spec.SyncPolicy, actual.Spec.SyncPolicy) {
		return false
	}
	return true
}

func compareSyncPolicy(goal, actual *argoapi.SyncPolicy) bool {
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
		return false
	}
	if !compareAutomatedSyncPolicy(goal.Automated, actual.Automated) {
		return false
	}
	if !compareSyncOptions(goal.SyncOptions, actual.SyncOptions) {
		return false
	}
	if !compareRetryStrategy(goal.Retry, actual.Retry) {
		return false
	}
	return true
}

func compareRetryStrategy(goal, actual *argoapi.RetryStrategy) bool {
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
		return false
	}
	if goal.Limit != actual.Limit {
		log.Printf("RetryStrategy Limit changed %d -> %d\n", actual.Limit, goal.Limit)
		return false
	}
	return true
}

func compareAutomatedSyncPolicy(goal, actual *argoapi.SyncPolicyAutomated) bool {
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
		return false
	}
	if goal.Prune != actual.Prune {
		log.Printf("SyncPolicy Prune changed %t -> %t\n", actual.Prune, goal.Prune)
		return false
	}
	if goal.AllowEmpty != actual.AllowEmpty {
		log.Printf("SyncPolicy AllowEmpty changed %t -> %t\n", actual.AllowEmpty, goal.AllowEmpty)
		return false
	}
	if goal.SelfHeal != actual.SelfHeal {
		log.Printf("SyncPolicy SelfHeal changed %t -> %t\n", actual.SelfHeal, goal.SelfHeal)
		return false
	}
	return true
}

func compareSyncOptions(goal, actual argoapi.SyncOptions) bool {
	if len(goal) == 0 && len(actual) == 0 {
		return true
	}
	if len(goal) != len(actual) {
		return false
	}
	for i, gS := range goal {
		if gS != actual[i] {
			log.Printf("SyncOption at position %d changed: %s -> %s\n", i, actual[i], gS)
			return false
		}
	}
	return true
}

func compareSource(goal, actual *argoapi.ApplicationSource) bool {
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
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
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
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

func compareHelmParameters(goal, actual []argoapi.HelmParameter) bool {
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
		return false
	}
	if len(goal) != len(actual) {
		return false
	}
	for i, gP := range goal {
		if gP.Name != actual[i].Name {
			log.Printf("Helm parameter at position %d changed: %s -> %s\n", i, actual[i].Name, gP.Name)
			return false
		}
		if gP.Value != actual[i].Value {
			log.Printf("Helm parameter %s changed: %s -> %s\n", actual[i].Name, actual[i].Value, gP.Value)
			return false
		}
		if gP.ForceString != actual[i].ForceString {
			log.Printf("ForceString for Helm parameter %s changed: %t -> %t\n", actual[i].Name, actual[i].ForceString, gP.ForceString)
			return false
		}
	}
	return true
}

func compareHelmValueFiles(goal, actual []string) bool {
	if goal == nil && actual == nil {
		return true
	}
	if (goal == nil) != (actual == nil) {
		return false
	}
	if len(goal) != len(actual) {
		return false
	}
	for i, gV := range goal {
		if gV != actual[i] {
			log.Printf("ValueFile at position %d changed: %s -> %s\n", i, actual[i], gV)
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

// syncApplication syncs the application with prune and force options if such a sync is not already in progress.
// Returns nil if a sync is already in progress, error otherwise
func syncApplication(client argoclient.Interface, app *argoapi.Application, withPrune bool) error {
	if app.Operation != nil && app.Operation.Sync != nil && app.Operation.Sync.Prune == withPrune && slices.Contains(app.Operation.Sync.SyncOptions, "Force=true") {
		return nil
	}

	app.Operation = &argoapi.Operation{
		Sync: &argoapi.SyncOperation{
			Prune:       withPrune,
			SyncOptions: []string{"Force=true"},
		},
	}

	_, err := client.ArgoprojV1alpha1().Applications(app.Namespace).Update(context.Background(), app, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to sync application %q with 'prune: %t': %w", app.Name, withPrune, err)
	}

	return nil
}

// returns the child applications owned by the app-of-apps parentApp
func getChildApplications(client argoclient.Interface, parentApp *argoapi.Application) ([]argoapi.Application, error) {
	appList, err := client.ArgoprojV1alpha1().
		Applications("").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list child applications of %s: %w", parentApp.Name, err)
	}

	var result []argoapi.Application

	// Build expected tracking prefix
	// Example:
	// multicloud-gitops-hub:argoproj.io/Application:multicloud-gitops-hub/
	expectedPrefix := fmt.Sprintf(
		"%s:argoproj.io/Application:%s/",
		parentApp.Name,
		parentApp.Name,
	)

	for _, app := range appList.Items { //nolint:gocritic // rangeValCopy: each iteration copies 992 bytes
		if trackingID, ok := app.Annotations["argocd.argoproj.io/tracking-id"]; ok {
			if strings.HasPrefix(trackingID, expectedPrefix) {
				result = append(result, app)
			}
		}
	}
	return result, nil
}
