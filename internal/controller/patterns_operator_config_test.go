package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PatternsOperatorConfig get values", func() {
	Context("when the key exists in the config", func() {
		It("should return the config value", func() {
			config := PatternsOperatorConfig{
				"gitops.channel": "custom-channel",
			}
			Expect(config.getStringValue("gitops.channel")).To(Equal("custom-channel"))
		})
	})

	Context("when the key does not exist in config but exists in defaults", func() {
		It("should return the default value for gitops.channel", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitops.channel")).To(Equal(GitOpsDefaultChannel))
		})

		It("should return the default value for gitops.catalogSource", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitops.catalogSource")).To(Equal(GitOpsDefaultCatalogSource))
		})

		It("should return the default value for gitops.sourceNamespace", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitops.sourceNamespace")).To(Equal(GitOpsDefaultCatalogSourceNamespace))
		})

		It("should return the default value for gitops.installApprovalPlan", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitops.installApprovalPlan")).To(Equal(GitOpsDefaultApprovalPlan))
		})

		It("should return the default value for gitops.csv", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitops.csv")).To(Equal(""))
		})

		It("should return the default value for gitops.additionalArgoAdmins", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitops.additionalArgoAdmins")).To(Equal(""))
		})

		It("should return the default value for gitops.applicationHealthCheckEnabled", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getBoolValue("gitops.applicationHealthCheckEnabled")).To(BeFalse())
		})

		It("should return the default value for gitea.chartName", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitea.chartName")).To(Equal(GiteaChartName))
		})

		It("should return the default value for gitea.helmRepoUrl", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitea.helmRepoUrl")).To(Equal(GiteaHelmRepoUrl))
		})

		It("should return the default value for gitea.chartVersion", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("gitea.chartVersion")).To(Equal(GiteaDefaultChartVersion))
		})

		It("should return the default value for catalog.image", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("catalog.image")).To(Equal(""))
		})
	})

	Context("when the key does not exist in config or defaults", func() {
		It("should return an empty string for string parameters", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getStringValue("nonexistent.key")).To(Equal(""))
		})
		It("should return false for boolean parameters", func() {
			config := PatternsOperatorConfig{}
			Expect(config.getBoolValue("nonexistent.key")).To(BeFalse())
		})
	})

	Context("when config overrides a default value", func() {
		It("should return the overridden value, not the default", func() {
			config := PatternsOperatorConfig{
				"gitops.channel": "gitops-1.99",
			}
			Expect(config.getStringValue("gitops.channel")).To(Equal("gitops-1.99"))
		})
	})

	Context("when config is nil", func() {
		It("should return the default value", func() {
			var config PatternsOperatorConfig
			Expect(config.getStringValue("gitops.channel")).To(Equal(GitOpsDefaultChannel))
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
			"gitops.csv",
			"gitops.additionalArgoAdmins",
			"gitops.applicationHealthCheckEnabled",
			"gitea.chartName",
			"gitea.helmRepoUrl",
			"gitea.chartVersion",
			"catalog.image",
		}
		for _, key := range expectedKeys {
			Expect(DefaultPatternsOperatorConfig).To(HaveKey(key))
		}
	})
})
