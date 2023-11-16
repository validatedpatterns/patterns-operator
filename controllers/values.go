package controllers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

func mergeHelmValues(files ...string) (map[string]any, error) {
	mergedValues := make(map[string]any)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Skip this file as it does not exist
				continue
			}
			// For all other errors, return the error
			return nil, err
		}

		var values map[string]any
		if err := yaml.Unmarshal(content, &values); err != nil {
			return nil, err
		}

		mergedValues = mergeMaps(mergedValues, values)
	}

	return mergedValues, nil
}

func mergeMaps(map1, map2 map[string]any) map[string]any {
	merged := make(map[string]any)

	for k, v := range map1 {
		merged[k] = v
	}

	for k, v := range map2 {
		if existing, ok := merged[k]; ok {
			switch existingValue := existing.(type) {
			case map[string]any:
				if newValue, ok := v.(map[string]any); ok {
					merged[k] = mergeMaps(existingValue, newValue)
				} else {
					// If types are not compatible, overwrite with the new value
					merged[k] = v
				}
			default:
				// If not a map, overwrite with the new value
				merged[k] = v
			}
		} else {
			// If key does not exist in the first map, add it
			merged[k] = v
		}
	}

	return merged
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

func helmTpl(templateString string, valueFiles []string, values map[string]interface{}) (string, error) {
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

	// Load and merge values from the specified value files.
	mergedValues := make(map[string]interface{})
	for _, fileName := range valueFiles {
		fileValues, err := chartutil.ReadValuesFile(filepath.Clean(fileName))
		if err != nil {
			return "", fmt.Errorf("error reading values file %s: %w", fileName, err)
		}
		mergedValues = chartutil.CoalesceTables(mergedValues, fileValues)
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
