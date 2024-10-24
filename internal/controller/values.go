package controllers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

func mergeHelmValues(files ...string) (map[string]any, error) {
	mergedValues := make(map[string]any)

	for _, file := range files {
		var values map[string]any
		values, err := chartutil.ReadValuesFile(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Skip this file as it does not exist
				continue
			}
			// For all other errors, return the error
			return nil, err
		}
		// Contrary to intuition the dst argument (values) takes precedence
		mergedValues = chartutil.CoalesceTables(values, mergedValues)
	}

	return mergedValues, nil
}

func getClusterGroupValue(key string, values map[string]any) any {
	clusterGroup, hasClusterGroup := values["clusterGroup"]
	if !hasClusterGroup {
		return nil
	}

	clusterGroupMap := clusterGroup.(map[string]any)
	v, hasKey := clusterGroupMap[key]
	if hasKey {
		return v
	}
	return nil
}

func helmTpl(templateString string, valueFiles []string, values map[string]any) (string, error) {
	// Create a fake chart with the template.
	fakeChart := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: "v2",
			Name:       "fake",
			Version:    "0.1.0",
		},
		Templates: []*chart.File{
			{
				Name: "templates/template.tpl",
				Data: []byte(templateString),
			},
		},
	}

	// Load and merge values from the specified value files. Note that the ordering is a bit
	// unexpected. The first values added are the more specific ones that will win
	mergedValues := make(map[string]any)
	// Contrary to intuition the dst argument (values) takes precedence
	mergedValues = chartutil.CoalesceTables(values, mergedValues)
	for _, fileName := range valueFiles {
		fname := filepath.Clean(fileName)
		// If the file does not exist we simply skip it
		if _, err := os.Stat(fname); os.IsNotExist(err) {
			continue
		}
		fileValues, err := chartutil.ReadValuesFile(fname)
		if err != nil {
			return "", fmt.Errorf("error reading values file %s: %w", fileName, err)
		}
		// Contrary to intuition the dst argument (values) takes precedence
		mergedValues = chartutil.CoalesceTables(fileValues, mergedValues)
	}

	// Merge with the additional values provided.
	mergedValues = chartutil.CoalesceTables(mergedValues, values)

	// Render the template.
	options := chartutil.ReleaseOptions{
		Name:      "fake-release",
		Namespace: "default",
		IsInstall: true,
		IsUpgrade: false,
	}
	valuesToRender, err := chartutil.ToRenderValues(fakeChart, mergedValues, options, chartutil.DefaultCapabilities)
	if err != nil {
		return "", fmt.Errorf("error preparing render values: %w", err)
	}

	renderedTemplates, err := engine.Render(fakeChart, valuesToRender)
	if err != nil {
		return "", fmt.Errorf("error rendering template: %w", err)
	}

	// Extract the rendered template.
	rendered, ok := renderedTemplates["fake/templates/template.tpl"]
	if !ok {
		return "", fmt.Errorf("rendered template not found")
	}

	return rendered, nil
}

func countApplicationsAndSets(a any) (appCount, appSetsCount int) {
	applicationCount := 0
	applicationSetsCount := 0
	applicationSetsKeys := []string{"generators", "generatorFile", "useGeneratorValues", "destinationServer", "destinationNamespace"}

	m, ok := a.(map[string]any)
	if !ok {
		return 0, 0
	}
	for _, v := range m {
		foundApplicationSet := false
		subMap, ok := v.(map[string]any)
		if !ok {
			// If it's not a map, skip it
			continue
		}
		// ApplicationSets have one of these subkeys in the application
		for _, key := range applicationSetsKeys {
			if _, exists := subMap[key]; exists {
				foundApplicationSet = true
				break
			}
		}
		if foundApplicationSet {
			applicationSetsCount++
		} else {
			applicationCount++
		}
	}
	return applicationCount, applicationSetsCount
}
