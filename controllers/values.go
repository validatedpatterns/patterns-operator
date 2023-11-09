package controllers

import (
	"os"

	"gopkg.in/yaml.v2"
)

func mergeHelmValues(files ...string) (map[string]interface{}, error) {
	mergedValues := make(map[string]interface{})

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		var values map[string]interface{}
		if err := yaml.Unmarshal(content, &values); err != nil {
			return nil, err
		}

		mergedValues = mergeMaps(mergedValues, values)
	}

	return mergedValues, nil
}

func mergeMaps(map1, map2 map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	for k, v := range map1 {
		merged[k] = v
	}

	for k, v := range map2 {
		if existing, ok := merged[k]; ok {
			switch existingValue := existing.(type) {
			case map[string]interface{}:
				if newValue, ok := v.(map[string]interface{}); ok {
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
