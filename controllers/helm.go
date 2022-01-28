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

func installChart(releaseName string, releaseNamespace string) {
	chartPath := "/mypath"

	settings := cli.New()

	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	if err := actionConfig.Init(settings.RESTClientGetter(), releaseNamespace,
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
	vals := map[string]interface{}{
		"redis": map[string]interface{}{
			"sentinel": map[string]interface{}{
				"masterName": "BigMaster",
				"pass":       "random",
				"addr":       "localhost",
				"port":       "26379",
			},
		},
	}

	// load chart from the path
	chart, err := loader.Load(chartPath)
	if err != nil {
		panic(err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = releaseNamespace
	client.ReleaseName = releaseName
	// client.DryRun = true - very handy!

	// install the chart here
	rel, err := client.Run(chart, vals)
	if err != nil {
		panic(err)
	}

	log.Printf("Installed Chart from path: %s in namespace: %s\n", rel.Name, rel.Namespace)
	// this will confirm the values set during installation
	log.Println(rel.Config)
}

// https://stackoverflow.com/questions/45692719/samples-on-kubernetes-helm-golang-client
func listCharts() {

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
	}
}
