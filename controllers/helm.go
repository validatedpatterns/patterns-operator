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
	"errors"
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

type HelmChart struct {
	Name       string
	Namespace  string
	Version    int
	Path       string
	Parameters chartutil.Values
}

func getConfiguration() (error, *action.Configuration) {
	settings := cli.New()

	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	driver := os.Getenv("HELM_DRIVER")
	if len(driver) == 0 {
		// configmaps, secrets, memory, or sql
		// sql requires HELM_DRIVER_SQL_CONNECTION_STRING
		// See helm.sh/helm/v3/pkg/action/action.go
		driver = "configmap"
	}

	//	if err := actionConfig.Init(kube.GetConfig(kubeconfigPath, "", releaseNamespace), releaseNamespace, driver, func(format string, v ...interface{}) {
	//		_ = fmt.Sprintf(format, v)
	//	}); err != nil {
	//		panic(err)
	//	}

	// settings.Namespace() == where we are running
	// helm client uses 'default'
	if err := actionConfig.Init(settings.RESTClientGetter(), "default", driver, log.Printf); err != nil {
		log.Printf("Bad config: %+v", err)
		return err, nil
	}

	if err := actionConfig.KubeClient.IsReachable(); err != nil {
		log.Printf("not reachable: %+v", err)
		return err, nil
	}

	return nil, actionConfig
}

func getChartObj(c HelmChart) (error, *chart.Chart) {

	// load chart from the path
	chartobj, err := loader.Load(c.Path)
	return err, chartobj
}

func uninstallChart(name string) error {
	err, actionConfig := getConfiguration()
	if err != nil {
		return err
	}

	// func (i *Install) Run(chrt *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	// vendor/helm.sh/helm/v3/pkg/release/release.go
	client := action.NewUninstall(actionConfig)

	// install the chart here
	res, err := client.Run(name)
	if err != nil {
		return err
	}

	log.Printf("Removed Chart %s: %s\n", name, res.Info)
	// this will confirm the values set during installation

	return nil
}

func installChart(c HelmChart) (error, int) {

	err, actionConfig := getConfiguration()
	if err != nil {
		return err, -1
	}

	err, chartobj := getChartObj(c)
	if err != nil {
		return err, -1
	}

	// func (i *Install) Run(chrt *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	// vendor/helm.sh/helm/v3/pkg/release/release.go
	client := action.NewInstall(actionConfig)
	client.Namespace = c.Namespace
	client.ReleaseName = c.Name
	// client.DryRun = true - very handy!

	// install the chart here
	rel, err := client.Run(chartobj, c.Parameters)
	if err != nil {
		return err, -1
	}

	log.Printf("Installed Chart %s from path: %s in namespace: %s\n", rel.Name, c.Path, rel.Namespace)
	// this will confirm the values set during installation
	log.Println(rel.Config)
	return nil, rel.Version
}

func getChartValues(name string) (error, map[string]interface{}) {

	if err, actionConfig := getConfiguration(); err != nil {
		return err, nil
	} else {

		client := action.NewGetValues(actionConfig)

		vals, err := client.Run(name)
		return err, vals
	}
}

func updateChart(c HelmChart) (error, int) {

	err, actionConfig := getConfiguration()
	if err != nil {
		return err, -1
	}

	err, chartobj := getChartObj(c)
	if err != nil {
		return err, -1
	}

	client := action.NewUpgrade(actionConfig)
	client.Namespace = c.Namespace

	// (*release.Release, error)
	//	ctx := context.Background()
	//	_, err := client.RunWithContext(ctx, c.Name, chartobj, c.Parameters)
	rel, err := client.Run(c.Name, chartobj, c.Parameters)
	return err, rel.Version
}

func chartForPattern(pattern api.Pattern) *HelmChart {
	c := HelmChart{
		Name:      pattern.Name,
		Namespace: pattern.Namespace,
		Path:      fmt.Sprintf("%s/common/install", pattern.Status.Path),
	}

	err, actionConfig := getConfiguration()
	if err != nil {
		return nil
	}

	err, chartobj := getChartObj(c)
	if err != nil {
		log.Printf("Bad chart: %s\n", err.Error())
		return nil
	}

	rel, err := lastRelease(*actionConfig, pattern.Name, chartobj)
	if err == nil && rel != nil && rel.Chart != nil {
		c.Version = rel.Version
		c.Parameters = rel.Chart.Values
		return &c
	} else if err != nil {
		log.Printf("Chart not installed: %s\n", err.Error())
	} else {
		log.Printf("Chart not installed\n")
	}
	return nil
}

func coalesceChartValues(pattern api.Pattern, c HelmChart) (error, map[string]interface{}) {
	calculated := inputsForPattern(pattern, false)

	err, chartobj := getChartObj(c)
	if err != nil {
		return err, nil
	}

	vals, err := chartutil.CoalesceValues(chartobj, calculated)
	return err, vals
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
	err, cfg := getConfiguration()
	deployed, err := cfg.Releases.ListDeployed()
	if err != nil {
		log.Printf("Could not list deployed charts: %s\n", err.Error())
		return charts
	}

	for _, release := range deployed {
		log.Println(release.Name)
		charts = append(charts, HelmChart{
			Name:      release.Name,
			Namespace: release.Namespace,
			Version:   release.Version,
		})
	}
	return charts
}

func lastRelease(cfg action.Configuration, name string, chart *chart.Chart) (*release.Release, error) {
	if chart == nil {
		return nil, fmt.Errorf("errMissingChart")
	}

	// finds the last non-deleted release with the given name
	lastRelease, err := cfg.Releases.Last(name)
	if err != nil {
		// to keep existing behavior of returning the "%q has no deployed releases" error when an existing release does not exist
		log.Printf("Error obtaining chart: %s\n", err.Error())
		return nil, nil
	}

	// Concurrent `helm upgrade`s will either fail here with `errPending` or when creating the release with "already exists". This should act as a pessimistic lock.
	if lastRelease.Info.Status.IsPending() {
		log.Println("Chart is in a pending state - this is bad")
		//return nil, fmt.Errorf("errPending")
		return lastRelease, nil

	}

	var currentRelease *release.Release
	if lastRelease.Info.Status == release.StatusDeployed {
		// no need to retrieve the last deployed release from storage as the last release is deployed
		currentRelease = lastRelease
	} else {
		// finds the deployed release with the given name
		currentRelease, err = cfg.Releases.Deployed(name)
		if err != nil {
			if errors.Is(err, driver.ErrNoDeployedReleases) &&
				(lastRelease.Info.Status == release.StatusFailed || lastRelease.Info.Status == release.StatusSuperseded) {
				currentRelease = lastRelease
			} else {
				return nil, err
			}
		}
	}
	return currentRelease, nil
}
