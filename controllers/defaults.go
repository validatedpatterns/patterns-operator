package controllers

// Below are the default constants that we will
// use throughout the patterns operator code
const (
	// Default Operator Namespace
	OperatorNamespace = "openshift-operators"
	// Default Operator Config Map Name
	OperatorConfigMap = "patterns-operator-config"
	// Default Subscription Namespace
	SubscriptionNamespace = "openshift-operators"
	// Default Application Namespace (i.e. when EnableArgoInitContainers is false)
	ApplicationNamespace = "openshift-gitops"
	// ClusterWide Argo Name
	ClusterWideArgoName = "openshift-gitops"
	// Application Namespace of cluster-wide argo (i.e. when EnableArgoInitContainers is true)
	VPApplicationNamespace = "validated-patterns"
)

// GitOps Subscription
const (
	GitOpsDefaultChannel                = "gitops-1.11"
	GitOpsDefaultPackageName            = "openshift-gitops-operator"
	GitOpsDefaultCatalogSource          = "redhat-operators"
	GitOpsDefaultCatalogSourceNamespace = "openshift-marketplace"
	GitOpsDefaultApprovalPlan           = "Automatic"
)

// GitOps Configuration
const (
	// Require manual intervention before Argo will sync new content. Default: False
	GitOpsDefaultManualSync = "false"
	// Require manual confirmation before installing and upgrading operators. Default: False
	GitOpsDefaultManualApproval = "false"
	// Dangerous. Force a specific version to be installed. Default: False
	GitOpsDefaultUseCSV = "false"
)

// Experimental Capabilities that can be enabled
const (
	VPInitContainers = "initcontainers"
)

var DefaultPatternOperatorConfig = map[string]string{
	"gitops.catalogSource":       GitOpsDefaultCatalogSource,
	"gitops.name":                GitOpsDefaultPackageName,
	"gitops.channel":             GitOpsDefaultChannel,
	"gitops.sourceNamespace":     GitOpsDefaultCatalogSourceNamespace,
	"gitops.installApprovalPlan": GitOpsDefaultApprovalPlan,
	"gitops.ManualSync":          GitOpsDefaultManualSync,
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
