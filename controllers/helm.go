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
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

func getChartValues(name string) (error, map[string]interface{}) {

	if err, actionConfig := getConfiguration(); err != nil {
		return err, nil
	} else {

		client := action.NewGetValues(actionConfig)

		vals, err := client.Run(name)
		return err, vals
	}
}

func inputsForPattern(p api.Pattern) map[string]interface{} {
	gitMap := map[string]interface{}{
		"repoURL": p.Spec.GitConfig.TargetRepo,
	}

	if len(p.Spec.GitConfig.TargetRevision) > 0 {
		gitMap["revision"] = p.Spec.GitConfig.TargetRevision
	}

	if len(p.Spec.GitConfig.ValuesDirectoryURL) > 0 {
		gitMap["valuesDirectoryURL"] = p.Spec.GitConfig.ValuesDirectoryURL
	}

	inputs := map[string]interface{}{
		"main": map[string]interface{}{
			"git": gitMap,
			"options": map[string]interface{}{
				"syncPolicy":          p.Spec.GitOpsConfig.SyncPolicy,
				"installPlanApproval": p.Spec.GitOpsConfig.InstallPlanApproval,
				"useCSV":              p.Spec.GitOpsConfig.UseCSV,
				"bootstrap":           p.Status.NeedSubscription,
			},
			"gitops": map[string]interface{}{
				"channel": p.Spec.GitOpsConfig.OperatorChannel,
				"source":  p.Spec.GitOpsConfig.OperatorSource,
				"csv":     p.Spec.GitOpsConfig.OperatorCSV,
			},
			"clusterGroupName": p.Spec.ClusterGroupName,
		},

		"global": map[string]interface{}{
			"hubClusterDomain":   p.Status.ClusterDomain,
			"localClusterDomain": p.Status.ClusterDomain,
			"imageregistry": map[string]interface{}{
				"type": "quay",
			},
			"git": map[string]interface{}{
				"hostname": p.Spec.GitConfig.Hostname,
				// Account is the user or organization under which the pattern repo lives
				"account": p.Spec.GitConfig.Account,
			},
		},
	}
	return inputs
}

func installChart(pattern api.Pattern) (error, int) {

	err, actionConfig := getConfiguration()
	if err != nil {
		return err, -1
	}

	err, chartobj := getChartObj(pattern.Status.Path)
	if err != nil {
		return err, -1
	}

	// func (i *Install) Run(chrt *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	// vendor/helm.sh/helm/v3/pkg/release/release.go
	client := action.NewInstall(actionConfig)
	client.Namespace = pattern.Namespace
	client.ReleaseName = pattern.Name
	// client.DryRun = true - very handy!

	// install the chart here
	values := inputsForPattern(pattern)
	rel, err := client.Run(chartobj, values)
	if err != nil {
		return err, -1
	}

	log.Printf("Installed Chart %s from path: %s in namespace: %s\n", rel.Name, pattern.Status.Path, rel.Namespace)
	// this will confirm the values set during installation
	log.Println(rel.Config)
	return nil, rel.Version
}

func updateChart(pattern api.Pattern) (error, int) {

	err, actionConfig := getConfiguration()
	if err != nil {
		return err, -1
	}

	err, chartobj := getChartObj(pattern.Status.Path)
	if err != nil {
		return err, -1
	}

	client := action.NewUpgrade(actionConfig)
	client.Namespace = pattern.Namespace

	// (*release.Release, error)
	//	ctx := context.Background()
	//	_, err := client.RunWithContext(ctx, pattern.Name, chartobj, values)
	values := inputsForPattern(pattern)
	rel, err := client.Run(pattern.Name, chartobj, values)

	return err, rel.Version
}

func overwriteWithChart(pattern api.Pattern) (error, int) {

	err, actionConfig := getConfiguration()
	if err != nil {
		return err, -1
	}

	err, chartobj := getChartObj(pattern.Status.Path)
	if err != nil {
		return err, -1
	}

	// func (i *Install) Run(chrt *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	// vendor/helm.sh/helm/v3/pkg/release/release.go
	client := action.NewInstall(actionConfig)
	client.Namespace = pattern.Namespace
	client.ReleaseName = pattern.Name
	client.DryRun = true

	// install the chart here
	values := inputsForPattern(pattern)
	rel, err := client.Run(chartobj, values)
	if err != nil {
		return err, -1
	}

	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))

	for i, m := range rel.Hooks {
		fmt.Printf("Rendering hook %d\n", i)
		fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)

		//OutputDir := "/tmp/..."
		//err = writeToFile(OutputDir, m.Path, m.Manifest, fileWritten[m.Path])
		//if err != nil {
		//	return err
		//}
		//fileWritten[m.Path] = true
	}

	log.Printf("Installed Chart %s from path: %s in namespace: %s\n", rel.Name, pattern.Status.Path, rel.Namespace)
	fmt.Printf("%s", manifests.String())

	// this will confirm the values set during installation
	log.Println(rel.Config)
	return nil, rel.Version
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

func isPatternDeployed(name string) bool {

	err, actionConfig := getConfiguration()
	if err != nil {
		return false
	}
	if _, err := actionConfig.Releases.Deployed(name); err != nil {
		return false
	}

	return true
}

func coalesceChartValues(pattern api.Pattern) (error, map[string]interface{}) {
	calculated := inputsForPattern(pattern)

	err, chartobj := getChartObj(pattern.Status.Path)
	if err != nil {
		return err, nil
	}

	vals, err := chartutil.CoalesceValues(chartobj, calculated)
	return err, vals
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

func getChartObj(path string) (error, *chart.Chart) {

	// load chart from the path
	chartobj, err := loader.Load(fmt.Sprintf("%s/common/install", path))
	return err, chartobj
}

func getConfiguration() (error, *action.Configuration) {
	settings := cli.New()

	actionConfig := new(action.Configuration)

	// configmaps, secrets, memory, or sql
	// The default is secrets
	//
	// sql requires HELM_DRIVER_SQL_CONNECTION_STRING
	// See helm.sh/helm/v3/pkg/action/action.go
	driver := os.Getenv("HELM_DRIVER")

	//	if err := actionConfig.Init(kube.GetConfig(kubeconfigPath, "", releaseNamespace), releaseNamespace, driver, func(format string, v ...interface{}) {
	//		_ = fmt.Sprintf(format, v)
	//	}); err != nil {
	//		panic(err)
	//	}

	// settings.Namespace() == where we are running
	// helm client uses 'default'
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
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
