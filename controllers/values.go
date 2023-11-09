package controllers

import (
	"os"

	"gopkg.in/yaml.v3"
)

func mergeHelmValues(files ...string) (map[string]any, error) {
	mergedValues := make(map[string]any)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
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
