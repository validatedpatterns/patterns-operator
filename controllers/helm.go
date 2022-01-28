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
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"

	"helm.sh/helm/v3/pkg/cli"
	//	"helm.sh/helm/v3/pkg/release"

	// listCharts
	//"k8s.io/client-go/kubernetes"
	"k8s.io/helm/pkg/helm"
	// "k8s.io/helm/pkg/helm/portforwarder"
)

type Values map[string]interface{}

type HelmChart struct {
	Name       string
	Namespace  string
	Version    int32
	Path       string
	Parameters Values
}

func installChart(chart HelmChart) (error, int) {
	settings := cli.New()

	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	if err := actionConfig.Init(settings.RESTClientGetter(), chart.Namespace,
		os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}

	//	if err := actionConfig.Init(kube.GetConfig(kubeconfigPath, "", releaseNamespace), releaseNamespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
	//		_ = fmt.Sprintf(format, v)
	//	}); err != nil {
	//		panic(err)
	//	}

	// define values
	//	chart.Parameters := map[string]interface{}{
	//		"redis": map[string]interface{}{
	//			"sentinel": map[string]interface{}{
	//				"masterName": "BigMaster",
	//				"pass":       "random",
	//				"addr":       "localhost",
	//				"port":       "26379",
	//			},
	//		},
	//	}

	// load chart from the path
	chartobj, err := loader.Load(chart.Path)
	if err != nil {
		panic(err)
	}

	// func (i *Install) Run(chrt *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	// vendor/helm.sh/helm/v3/pkg/release/release.go
	client := action.NewInstall(actionConfig)
	client.Namespace = chart.Namespace
	client.ReleaseName = chart.Name
	// client.DryRun = true - very handy!

	// install the chart here
	rel, err := client.Run(chartobj, chart.Parameters)
	if err != nil {
		return err, -1
	}

	log.Printf("Installed Chart %s from path: %s in namespace: %s\n", rel.Name, chart.Path, rel.Namespace)
	// this will confirm the values set during installation
	log.Println(rel.Config)
	return nil, rel.Version
}

// https://stackoverflow.com/questions/45692719/samples-on-kubernetes-helm-golang-client
func installedCharts() []HelmChart {
	// ./vendor/k8s.io/helm/pkg/proto/hapi/release/release.pb.go

	// type Release struct {
	//         // Name is the name of the release
	//         Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	//         // Info provides information about a release
	//         Info *Info `protobuf:"bytes,2,opt,name=info,proto3" json:"info,omitempty"`
	//         // Chart is the chart that was released.
	//         Chart *chart.Chart `protobuf:"bytes,3,opt,name=chart,proto3" json:"chart,omitempty"`
	//         // Config is the set of extra Values added to the chart.
	//         // These values override the default values inside of the chart.
	//         Config *chart.Config `protobuf:"bytes,4,opt,name=config,proto3" json:"config,omitempty"`
	//         // Manifest is the string representation of the rendered template.
	//         Manifest string `protobuf:"bytes,5,opt,name=manifest,proto3" json:"manifest,omitempty"`
	//         // Hooks are all of the hooks declared for this release.
	//         Hooks []*Hook `protobuf:"bytes,6,rep,name=hooks,proto3" json:"hooks,omitempty"`
	//         // Version is an int32 which represents the version of the release.
	//         Version int32 `protobuf:"varint,7,opt,name=version,proto3" json:"version,omitempty"`
	//         // Namespace is the kubernetes namespace of the release.
	//         Namespace            string   `protobuf:"bytes,8,opt,name=namespace,proto3" json:"namespace,omitempty"`
	//         XXX_NoUnkeyedLiteral struct{} `json:"-"`
	//         XXX_unrecognized     []byte   `json:"-"`
	//         XXX_sizecache        int32    `json:"-"`
	// }

	var charts []HelmChart
	// omit getting kubeConfig, see: https://github.com/kubernetes/client-go/tree/master/examples

	// get kubernetes client
	// client, _ := kubernetes.NewForConfig(kubeConfig)

	// port forward tiller
	// tillerTunnel, _ := portforwarder.New("kube-system", client, config)
	host := "tiller-deploy.kube-system.svc:44134"

	// new helm client
	helmClient := helm.NewClient(helm.Host(host))

	// list/print releases
	resp, _ := helmClient.ListReleases()
	for _, release := range resp.Releases {
		log.Println(release.GetName())
		charts = append(charts, HelmChart{
			Name:      release.GetName(),
			Namespace: release.GetNamespace(),
			Version:   release.GetVersion(),
		})
	}
	return charts
}
