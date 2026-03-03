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

	It("should render template with empty values", func() {
		simpleTemplate := "static text"
		values := map[string]any{}
		valueFiles := []string{}

		rendered, err := helmTpl(simpleTemplate, valueFiles, values)
		Expect(err).ToNot(HaveOccurred())
		Expect(rendered).To(Equal("static text"))
	})
})

var _ = Describe("MergeHelmValues", func() {
	var td string

	BeforeEach(func() {
		var err error
		td, err = os.MkdirTemp("", "vp-merge-test")
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() {
		os.RemoveAll(td)
	})

	Context("with no files", func() {
		It("should return empty map", func() {
			result, err := mergeHelmValues()
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Context("with non-existent files", func() {
		It("should skip missing files", func() {
			result, err := mergeHelmValues("/nonexistent/file.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Context("with valid values files", func() {
		It("should merge values from multiple files", func() {
			file1 := filepath.Join(td, "values1.yaml")
			file2 := filepath.Join(td, "values2.yaml")
			err := os.WriteFile(file1, []byte("key1: value1\nshared: from-file1\n"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(file2, []byte("key2: value2\nshared: from-file2\n"), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := mergeHelmValues(file1, file2)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("key1", "value1"))
			Expect(result).To(HaveKeyWithValue("key2", "value2"))
			// Later files take precedence (CoalesceTables with dst precedence)
			Expect(result).To(HaveKey("shared"))
		})
	})

	Context("with invalid YAML file", func() {
		It("should return an error", func() {
			file := filepath.Join(td, "invalid.yaml")
			err := os.WriteFile(file, []byte("{{invalid yaml}}"), 0644)
			Expect(err).ToNot(HaveOccurred())

			_, err = mergeHelmValues(file)
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("GetClusterGroupValue", func() {
	Context("when clusterGroup key exists", func() {
		It("should return the value for the requested key", func() {
			values := map[string]any{
				"clusterGroup": map[string]any{
					"name": "test-group",
				},
			}
			result := getClusterGroupValue("name", values)
			Expect(result).To(Equal("test-group"))
		})
	})

	Context("when clusterGroup key does not exist", func() {
		It("should return nil", func() {
			values := map[string]any{
				"other": "value",
			}
			result := getClusterGroupValue("name", values)
			Expect(result).To(BeNil())
		})
	})

	Context("when requested key does not exist in clusterGroup", func() {
		It("should return nil", func() {
			values := map[string]any{
				"clusterGroup": map[string]any{
					"name": "test-group",
				},
			}
			result := getClusterGroupValue("nonexistent", values)
			Expect(result).To(BeNil())
		})
	})
})

var _ = Describe("CountApplicationsAndSets", func() {
	Context("with nil input", func() {
		It("should return 0, 0", func() {
			apps, appsets := countApplicationsAndSets(nil)
			Expect(apps).To(Equal(0))
			Expect(appsets).To(Equal(0))
		})
	})

	Context("with non-map input", func() {
		It("should return 0, 0", func() {
			apps, appsets := countApplicationsAndSets("not a map")
			Expect(apps).To(Equal(0))
			Expect(appsets).To(Equal(0))
		})
	})

	Context("with applications only", func() {
		It("should count applications correctly", func() {
			input := map[string]any{
				"app1": map[string]any{"name": "app1"},
				"app2": map[string]any{"name": "app2"},
			}
			apps, appsets := countApplicationsAndSets(input)
			Expect(apps).To(Equal(2))
			Expect(appsets).To(Equal(0))
		})
	})

	Context("with applicationSets only", func() {
		It("should count applicationSets correctly", func() {
			input := map[string]any{
				"appset1": map[string]any{"generators": []any{"gen1"}},
				"appset2": map[string]any{"generatorFile": "file.yaml"},
			}
			apps, appsets := countApplicationsAndSets(input)
			Expect(apps).To(Equal(0))
			Expect(appsets).To(Equal(2))
		})
	})

	Context("with mixed applications and applicationSets", func() {
		It("should count both correctly", func() {
			input := map[string]any{
				"app1":    map[string]any{"name": "app1"},
				"appset1": map[string]any{"generators": []any{"gen1"}},
				"app2":    map[string]any{"chart": "chart1"},
			}
			apps, appsets := countApplicationsAndSets(input)
			Expect(apps).To(Equal(2))
			Expect(appsets).To(Equal(1))
		})
	})

	Context("with applicationSet using different keys", func() {
		It("should detect useGeneratorValues key", func() {
			input := map[string]any{
				"appset": map[string]any{"useGeneratorValues": true},
			}
			apps, appsets := countApplicationsAndSets(input)
			Expect(apps).To(Equal(0))
			Expect(appsets).To(Equal(1))
		})

		It("should detect destinationServer key", func() {
			input := map[string]any{
				"appset": map[string]any{"destinationServer": "server"},
			}
			apps, appsets := countApplicationsAndSets(input)
			Expect(apps).To(Equal(0))
			Expect(appsets).To(Equal(1))
		})

		It("should detect destinationNamespace key", func() {
			input := map[string]any{
				"appset": map[string]any{"destinationNamespace": "ns"},
			}
			apps, appsets := countApplicationsAndSets(input)
			Expect(apps).To(Equal(0))
			Expect(appsets).To(Equal(1))
		})
	})

	Context("with non-map sub-entries", func() {
		It("should skip non-map values", func() {
			input := map[string]any{
				"app1":   map[string]any{"name": "app1"},
				"scalar": "not-a-map",
			}
			apps, appsets := countApplicationsAndSets(input)
			Expect(apps).To(Equal(1))
			Expect(appsets).To(Equal(0))
		})
	})
})

var _ = Describe("GitOpsConfig getValueWithDefault", func() {
	Context("when value exists in config", func() {
		It("should return the configured value", func() {
			config := GitOpsConfig{"key1": "value1"}
			Expect(config.getValueWithDefault("key1")).To(Equal("value1"))
		})
	})

	Context("when value does not exist in config but has a default", func() {
		It("should return the default value", func() {
			config := GitOpsConfig{}
			Expect(config.getValueWithDefault("gitops.channel")).To(Equal(GitOpsDefaultChannel))
		})
	})

	Context("when value does not exist anywhere", func() {
		It("should return empty string", func() {
			config := GitOpsConfig{}
			Expect(config.getValueWithDefault("nonexistent.key")).To(Equal(""))
		})
	})

	Context("when config overrides a default", func() {
		It("should return the config value not the default", func() {
			config := GitOpsConfig{"gitops.channel": "custom-channel"}
			Expect(config.getValueWithDefault("gitops.channel")).To(Equal("custom-channel"))
		})
	})

	Context("when config is nil", func() {
		It("should return the default value", func() {
			var config GitOpsConfig
			Expect(config.getValueWithDefault("gitops.channel")).To(Equal(GitOpsDefaultChannel))
		})
	})
})
