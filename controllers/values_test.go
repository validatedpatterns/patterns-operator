package controllers

import (
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helm Values", func() {
	Describe("mergeMaps", func() {
		It("should merge two maps", func() {
			map1 := map[string]any{"key1": "value1", "key2": "value2"}
			map2 := map[string]any{"key2": "newvalue2", "key3": "value3"}

			merged := mergeMaps(map1, map2)

			Expect(merged).To(HaveLen(3))
			Expect(merged).To(HaveKeyWithValue("key1", "value1"))
			Expect(merged).To(HaveKeyWithValue("key2", "newvalue2"))
			Expect(merged).To(HaveKeyWithValue("key3", "value3"))
		})

		It("should handle nested maps", func() {
			map1 := map[string]any{
				"key1": "value1",
				"key2": map[string]any{"nestedKey1": "nestedValue1"},
			}
			map2 := map[string]any{
				"key2": map[string]any{"nestedKey2": "nestedValue2"},
				"key3": "value3",
			}

			merged := mergeMaps(map1, map2)

			Expect(merged).To(HaveLen(3))
			Expect(merged).To(HaveKeyWithValue("key1", "value1"))

			nestedMap := merged["key2"].(map[string]any)
			Expect(nestedMap).To(HaveKeyWithValue("nestedKey1", "nestedValue1"))
			Expect(nestedMap).To(HaveKeyWithValue("nestedKey2", "nestedValue2"))

			Expect(merged).To(HaveKeyWithValue("key3", "value3"))
		})
	})

	Describe("mergeHelmValues", func() {
		It("should merge Helm flat values", func() {
			filePaths := []string{
				createTempValueFile("values1.yaml", map[string]any{"key1": "value1"}),
				createTempValueFile("values2.yaml", map[string]any{"key2": "value2"}),
				createTempValueFile("values3.yaml", map[string]any{"key3": "value3"}),
			}

			mergedValues, err := mergeHelmValues(filePaths...)
			Expect(err).NotTo(HaveOccurred())

			Expect(mergedValues).To(HaveLen(3))
			Expect(mergedValues).To(HaveKeyWithValue("key1", "value1"))
			Expect(mergedValues).To(HaveKeyWithValue("key2", "value2"))
			Expect(mergedValues).To(HaveKeyWithValue("key3", "value3"))
		})

		It("should merge Helm nested values", func() {
			filePaths := []string{
				createTempValueFile("values4.yaml", map[string]any{"key1": map[string]any{"value1": "nested1"}}),
				createTempValueFile("values5.yaml", map[string]any{"key2": "value2", "key3": "value3"}),
				createTempValueFile("values6.yaml", map[string]any{"key4": "value4"}),
				createTempValueFile("values7.yaml", map[string]any{"key2": "overridden"}),
			}

			mergedValues, err := mergeHelmValues(filePaths...)
			Expect(err).NotTo(HaveOccurred())

			Expect(mergedValues).To(HaveLen(4))
			Expect(mergedValues).To(HaveKeyWithValue("key2", "overridden"))
			Expect(mergedValues).To(HaveKeyWithValue("key3", "value3"))
			Expect(mergedValues).To(HaveKeyWithValue("key1", map[string]any{"value1": "nested1"}))
		})
	})
})

func createTempValueFile(name string, content any) string {
	filePath := filepath.Join(tempDir, name)
	data, err := yaml.Marshal(content)
	Expect(err).NotTo(HaveOccurred())

	err = os.WriteFile(filePath, data, 0600)
	Expect(err).NotTo(HaveOccurred())

	return filePath
}
