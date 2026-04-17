package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PatternsOperatorConfig getValueWithDefault", func() {
	Context("when the key exists in the config", func() {
		It("should return the config value", func() {
			config := PatternsOperatorConfig{
				"gitops.channel": "custom-channel",
			}
			Expect(config.getValueWithDefault("gitops.channel")).To(Equal("custom-channel"))
		})
	})

	Context("when the key does not exist in config but exists in defaults", func() {
		It("should return the default value for gitops.channel", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("gitops.channel")).To(Equal(GitOpsDefaultChannel))
		})

		It("should return the default value for gitops.catalogSource", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("gitops.catalogSource")).To(Equal(GitOpsDefaultCatalogSource))
		})

		It("should return the default value for gitops.sourceNamespace", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("gitops.sourceNamespace")).To(Equal(GitOpsDefaultCatalogSourceNamespace))
		})

		It("should return the default value for gitops.installApprovalPlan", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("gitops.installApprovalPlan")).To(Equal(GitOpsDefaultApprovalPlan))
		})

		It("should return the default value for gitea.chartName", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("gitea.chartName")).To(Equal(GiteaChartName))
		})

		It("should return the default value for gitea.helmRepoUrl", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("gitea.helmRepoUrl")).To(Equal(GiteaHelmRepoUrl))
		})

		It("should return the default value for gitea.chartVersion", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("gitea.chartVersion")).To(Equal(GiteaDefaultChartVersion))
		})

		It("should return the default value for analytics.enabled", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("analytics.enabled")).To(Equal("true"))
		})
	})

	Context("when the key does not exist in config or defaults", func() {
		It("should return an empty string", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getValueWithDefault("nonexistent.key")).To(Equal(""))
		})
	})

	Context("when config overrides a default value", func() {
		It("should return the overridden value, not the default", func() {
			config := PatternsOperatorConfig{
				"gitops.channel": "gitops-1.99",
			}
			Expect(config.getValueWithDefault("gitops.channel")).To(Equal("gitops-1.99"))
		})
	})

	Context("when config is nil", func() {
		It("should return the default value", func() {
			var config PatternsOperatorConfig
			Expect(config.getValueWithDefault("gitops.channel")).To(Equal(GitOpsDefaultChannel))
		})
	})
})

var _ = Describe("DefaultPatternsOperatorConfig", func() {
	It("should contain all expected keys", func() {
		expectedKeys := []string{
			"gitops.catalogSource",
			"gitops.channel",
			"gitops.sourceNamespace",
			"gitops.installApprovalPlan",
			"gitea.chartName",
			"gitea.helmRepoUrl",
			"gitea.chartVersion",
			"analytics.enabled",
		}
		for _, key := range expectedKeys {
			Expect(DefaultPatternsOperatorConfig).To(HaveKey(key))
		}
	})

	It("should have correct default values", func() {
		Expect(DefaultPatternsOperatorConfig["gitops.catalogSource"]).To(Equal("redhat-operators"))
		Expect(DefaultPatternsOperatorConfig["gitops.sourceNamespace"]).To(Equal("openshift-marketplace"))
		Expect(DefaultPatternsOperatorConfig["gitops.installApprovalPlan"]).To(Equal("Automatic"))
		Expect(DefaultPatternsOperatorConfig["analytics.enabled"]).To(Equal("true"))
	})
})
