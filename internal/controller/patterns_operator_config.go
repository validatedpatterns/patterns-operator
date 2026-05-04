package controllers

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// Creates the patterns operator configmap
// This will include configuration parameters that
// will allow operator configuration operatorConfigMap corev1.ConfigMap
func CreatePatternsOperatorConfigMap(ctx context.Context, cl client.Client) (*corev1.ConfigMap, error) {
	configMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      OperatorConfigMap,
			Namespace: DetectOperatorNamespace(),
		},
	}
	if err := cl.Create(ctx, &configMap, &client.CreateOptions{}); err != nil {
		return nil, err
	}
	return &configMap, nil
}

func GetPatternsOperatorConfigMap(ctx context.Context, cl client.Client) (*corev1.ConfigMap, error) {
	configMap := corev1.ConfigMap{}

	err := cl.Get(ctx, client.ObjectKey{Namespace: DetectOperatorNamespace(), Name: OperatorConfigMap}, &configMap, &client.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}
	return &configMap, nil
}
