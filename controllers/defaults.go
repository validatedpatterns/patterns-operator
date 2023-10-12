package controllers

import (
	"fmt"
)

// Below are the default constants that we will
// use throughout the patterns operator code
const (
	// Default Operator Namespace
	OperatorNamespace = "openshift-operators"
	// Default Operator Config Map Name
	OperatorConfigMap = "patterns-operator-config"
	// Default Subscription Namespace
	SubscriptionNamespace = "openshift-operators"
	// Default Application Namespace
	ApplicationNamespace = "openshift-gitops"
)

// GitOps Subscription
const (
	GitOpsDefaultChannel                = "gitops-1.8"
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

var DefaultPatternOperatorConfig = map[string]string{
	"gitops.catalogSource":       "redhat-operators",
	"gitops.name":                "openshift-gitops-operator",
	"gitops.channel":             "gitops-1.8",
	"gitops.sourceNamespace":     "openshift-marketplace",
	"gitops.installApprovalPlan": "Automatic",
	"gitops.ManualSync":          "false",
}

type GitOpsConfig map[string]string

var PatternsOperatorConfig GitOpsConfig

func (g GitOpsConfig) getValueWithDefault(k string) (string, error) {
	if v, present := g[k]; present {
		return v, nil
	}
	if defaultValue, present := DefaultPatternOperatorConfig[k]; present {
		return defaultValue, nil
	}
	return "", fmt.Errorf("could not find neither an existing value nor a default for %s", k)
}
