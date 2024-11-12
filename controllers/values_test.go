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

var _ = Describe("CountApplicationsAndSets", func() {
	var (
		input map[string]any
	)

	BeforeEach(func() {
		input = make(map[string]any)
	})

	Context("when input is empty", func() {
		It("returns zero for both counts", func() {
			appCount, appSetsCount := countApplicationsAndSets(input)
			Expect(appCount).To(Equal(0))
			Expect(appSetsCount).To(Equal(0))
		})
	})

	Context("when input contains only applications", func() {
		BeforeEach(func() {
			input["app1"] = map[string]any{"someKey": "someValue"}
			input["app2"] = map[string]any{"anotherKey": "anotherValue"}
		})

		It("returns the correct number of applications and zero application sets", func() {
			appCount, appSetsCount := countApplicationsAndSets(input)
			Expect(appCount).To(Equal(2))
			Expect(appSetsCount).To(Equal(0))
		})
	})

	Context("when input contains only application sets", func() {
		BeforeEach(func() {
			input["appSet1"] = map[string]any{"generators": "someValue"}
			input["appSet2"] = map[string]any{"destinationServer": "someServer"}
		})

		It("returns zero for applications and the correct number for application sets", func() {
			appCount, appSetsCount := countApplicationsAndSets(input)
			Expect(appCount).To(Equal(0))
			Expect(appSetsCount).To(Equal(2))
		})
	})

	Context("when input contains a mix of applications and application sets", func() {
		BeforeEach(func() {
			input["app1"] = map[string]any{"someKey": "someValue"}
			input["appSet1"] = map[string]any{"generators": "someValue"}
			input["app2"] = map[string]any{"someKey2": "someValue2"}
			input["app3"] = map[string]any{"someKey3": "someValue3"}
			input["appSet2"] = map[string]any{"generators": "someValue"}
			input["app4"] = map[string]any{"someKey4": "someValue4"}
		})

		It("returns the correct counts for both applications and application sets", func() {
			appCount, appSetsCount := countApplicationsAndSets(input)
			Expect(appCount).To(Equal(4))
			Expect(appSetsCount).To(Equal(2))
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

var _ = Describe("helmTpl", func() {
	const (
		templateString = "Hello, {{ .Values.name }}!"
	)

	It("should successfully render a template with inline values", func() {
		values := map[string]any{"name": "World"}
		valueFiles := []string{}

		rendered, err := helmTpl(templateString, valueFiles, values)

		Expect(err).ToNot(HaveOccurred())
		Expect(rendered).To(Equal("Hello, World!"))
	})

	It("should successfully render a template with values from a values file", func() {
		values := map[string]any{}
		valueFiles := []string{"testdata/values.yaml"}

		// Create a temporary values file
		valuesFileContent := []byte("name: WorldFromFile")
		_ = os.MkdirAll("testdata", os.ModePerm)
		defer os.RemoveAll("testdata")
		_ = os.WriteFile("testdata/values.yaml", valuesFileContent, 0600) //nolint:gosec

		rendered, err := helmTpl(templateString, valueFiles, values)
		Expect(err).ToNot(HaveOccurred())
		Expect(rendered).To(Equal("Hello, WorldFromFile!"))
	})

	It("should handle missing values files gracefully", func() {
		values := map[string]any{"name": "World"}
		valueFiles := []string{"testdata/missing_values.yaml"}

		rendered, err := helmTpl(templateString, valueFiles, values)
		Expect(err).ToNot(HaveOccurred())
		Expect(rendered).To(Equal("Hello, World!"))
	})

	It("should handle errors while preparing render values", func() {
		values := map[string]any{"name": make(chan int)} // invalid value type
		valueFiles := []string{}

		rendered, err := helmTpl(templateString, valueFiles, values)

		Expect(err).To(HaveOccurred())
		Expect(rendered).To(BeEmpty())
	})
})
