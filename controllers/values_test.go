package controllers

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Helm Values", func() {
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

	Describe("getClusterGroupValue", func() {
		var values map[string]any

		BeforeEach(func() {
			values = make(map[string]any)
		})

		Context("when clusterGroup key exists and has the desired key", func() {
			It("should return the correct value", func() {
				values["clusterGroup"] = map[string]any{"desiredKey": "desiredValue"}
				result := getClusterGroupValue("desiredKey", values)
				Expect(result).To(Equal("desiredValue"))
			})
		})

		Context("when clusterGroup key exists but does not have the desired key", func() {
			It("should return nil", func() {
				values["clusterGroup"] = map[string]any{"otherKey": "otherValue"}
				result := getClusterGroupValue("desiredKey", values)
				Expect(result).To(BeNil())
			})
		})

		Context("when clusterGroup key does not exist", func() {
			It("should return nil", func() {
				result := getClusterGroupValue("desiredKey", values)
				Expect(result).To(BeNil())
			})
		})

		Context("when clusterGroup is not a map", func() {
			It("should panic", func() {
				values["clusterGroup"] = "notAMap"
				Expect(func() { getClusterGroupValue("desiredKey", values) }).To(Panic())
			})
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
