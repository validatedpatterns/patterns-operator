package controllers

import (
	"os"
	"strings"
)

// OperatorNamespace is detected at runtime via DetectOperatorNamespace().
// It defaults to "openshift-operators" for backward compatibility.
var OperatorNamespace string

// DetectOperatorNamespace determines the namespace the operator is running in.
func DetectOperatorNamespace() string {
	if ns := os.Getenv("OPERATOR_NAMESPACE"); ns != "" {
		return ns
	}
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	return LegacyOperatorNamespace
}

// Below are the default constants that we will
// use throughout the patterns operator code
const (
	LegacyOperatorNamespace = "openshift-operators"
	// Default Operator Config Map Name
	OperatorConfigMap = "patterns-operator-config"
	// Default Application Namespace
	ApplicationNamespace = "openshift-gitops"
	// ClusterWide Argo Name
	ClusterWideArgoName = "openshift-gitops"
)

// GitOps Subscription
const (
	GitOpsDefaultSubscriptionNamespace  = "openshift-gitops-operator"
	GitOpsLegacySubscriptionNamespace   = LegacyOperatorNamespace
	GitOpsDefaultChannel                = "gitops-1.18"
	GitOpsDefaultPackageName            = "openshift-gitops-operator"
	GitOpsDefaultCatalogSource          = "redhat-operators"
	GitOpsDefaultCatalogSourceNamespace = "openshift-marketplace"
	GitOpsDefaultApprovalPlan           = "Automatic"
	// Dangerous. Force a specific version to be installed. Default: ""
	GitOpsDefaultCSV = ""
)

// Gitea chart defaults
const (
	// URL to the Validated Patterns Helm chart repo
	GiteaHelmRepoUrl = "https://charts.validatedpatterns.io/"
	// Repo name for the Validated Patterns Helm repo
	GiteaRepoName = "helm-charts"
	// Gitea chart name in the Validated Patterns repo
	GiteaChartName = "gitea"
	// Release name used by the Helm SDK
	GiteaReleaseName = "gitea"
	// Namespace for the Gitea resources
	GiteaNamespace = "vp-gitea"
	// Our gitea-chart default version (we stay on the latest 0.0.X version)
	GiteaDefaultChartVersion = "0.0.*"
	// Default Gitea Admin user
	GiteaAdminUser = "gitea_admin"
	// Gitea Admin Secrets name
	GiteaAdminSecretName = "gitea-admin-secret" //nolint:gosec
	// GiteaServer default name
	GiteaServerDefaultName = "vp-gitea-instance"
	// Gitea Route Name
	GiteaRouteName = "gitea-route"
	// Gitea Argo Application Name
	GiteaApplicationName = "gitea-in-cluster"
	// Gitea Default Random Password Length
	GiteaDefaultPasswordLen = 15
)

// Experimental Capabilities that can be enabled
// Currently none

var DefaultPatternOperatorConfig = map[string]string{
	"gitops.catalogSource":       GitOpsDefaultCatalogSource,
	"gitops.channel":             GitOpsDefaultChannel,
	"gitops.sourceNamespace":     GitOpsDefaultCatalogSourceNamespace,
	"gitops.installApprovalPlan": GitOpsDefaultApprovalPlan,
	"gitops.csv":                 GitOpsDefaultCSV,
	"gitea.chartName":            GiteaChartName,
	"gitea.helmRepoUrl":          GiteaHelmRepoUrl,
	"gitea.chartVersion":         GiteaDefaultChartVersion,
	"analytics.enabled":          "true",
}

type GitOpsConfig map[string]string

var PatternsOperatorConfig GitOpsConfig

func (g GitOpsConfig) getValueWithDefault(k string) string {
	if v, present := g[k]; present {
		return v
	}
	if defaultValue, present := DefaultPatternOperatorConfig[k]; present {
		return defaultValue
	}
	return ""
}
