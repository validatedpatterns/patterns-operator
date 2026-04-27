package controllers

import (
	"strings"
)

type PatternsOperatorConfig map[string]string

var DefaultPatternsOperatorConfig = PatternsOperatorConfig{
	"gitops.catalogSource":                 GitOpsDefaultCatalogSource,
	"gitops.channel":                       GitOpsDefaultChannel,
	"gitops.sourceNamespace":               GitOpsDefaultCatalogSourceNamespace,
	"gitops.installApprovalPlan":           GitOpsDefaultApprovalPlan,
	"gitops.csv":                           GitOpsDefaultCSV,
	"gitops.additionalArgoAdmins":          "",
	"gitops.applicationHealthCheckEnabled": "false",
	"gitea.chartName":                      GiteaChartName,
	"gitea.helmRepoUrl":                    GiteaHelmRepoUrl,
	"gitea.chartVersion":                   GiteaDefaultChartVersion,
	"catalog.image":                        "",
}

func (g PatternsOperatorConfig) getStringValue(k string) string {
	if v, present := g[k]; present {
		return v
	} else {
		return DefaultPatternsOperatorConfig[k]
	}
}

func (g PatternsOperatorConfig) getBoolValue(k string) bool {
	if v, present := g[k]; present {
		return strings.EqualFold(v, "true")
	} else {
		return strings.EqualFold(DefaultPatternsOperatorConfig[k], "true")
	}
}
