/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	//+kubebuilder:scaffold:imports
)

var testCases = []struct {
	inputURL     string
	expectedName string
	expectedFQDN string
}{
	{"https://github.com/username/repo.git", "repo", "github.com"},
	{"https://github.com/username/repo", "repo", "github.com"},
	{"https://github.com/username/repo.git/", "repo", "github.com"},
	{"https://github.com/username/repo/", "repo", "github.com"},
	{"https://gitlab.com/username/my-project.git", "my-project", "gitlab.com"},
	{"https://gitlab.com/username/my-project", "my-project", "gitlab.com"},
	{"https://bitbucket.org/username/myrepo.git", "myrepo", "bitbucket.org"},
	{"https://bitbucket.org/username/myrepo", "myrepo", "bitbucket.org"},
	{"https://example.com/username/repo.git", "repo", "example.com"},
	{"https://example.com/username/repo", "repo", "example.com"},
	{"https://example.com/username/repo.git/", "repo", "example.com"},
	{"https://example.com/username/repo/", "repo", "example.com"},
	{"git@github.com:mbaldessari/common.git", "common", "github.com"},
	{"git@github.com:mbaldessari/common.git/", "common", "github.com"},
	{"git@github.com:mbaldessari/common", "common", "github.com"},
}

var _ = Describe("ExtractRepositoryName", func() {
	Context("when the git URL is SSH format", func() {
		It("should extract the repository name correctly", func() {
			gitURL := "git@github.com:user/repo.git"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(repoName).To(Equal("repo"))
		})

		It("should handle URLs without .git suffix correctly", func() {
			gitURL := "git@github.com:user/repo"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(repoName).To(Equal("repo"))
		})

		It("should return an error for invalid SSH git URL", func() {
			gitURL := "git@github.com:user/repo:extra"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(repoName).To(BeEmpty())
		})
	})

	Context("when the git URL is HTTP/HTTPS format", func() {
		It("should extract the repository name correctly", func() {
			gitURL := "https://github.com/user/repo.git"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(repoName).To(Equal("repo"))
		})

		It("should handle URLs without .git suffix correctly", func() {
			gitURL := "https://github.com/user/repo"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(repoName).To(Equal("repo"))
		})

		It("should return an error for invalid HTTP/HTTPS git URL", func() {
			gitURL := "https//github.com@2/user://"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(repoName).To(BeEmpty())
		})

		It("should return an error for non-absolute HTTP/HTTPS URL", func() {
			gitURL := "github.com/user/repo.git"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(repoName).To(BeEmpty())
		})
	})
})

var _ = Describe("extractGitFQDNHostname", func() {
	It("should extract the fqdn name from various URL formats", func() {
		for _, testCase := range testCases {
			repoName, err := extractGitFQDNHostname(testCase.inputURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(repoName).To(Equal(testCase.expectedFQDN))
		}
	})

	It("should return an error for an invalid URL", func() {
		invalidURL := "lwn:///invalid-url"
		_, err := extractGitFQDNHostname(invalidURL)
		Expect(err).To(HaveOccurred())
	})
})
var _ = Describe("validGitRepoURL", func() {
	It("should accept a 'git@' URL", func() {
		repoURL := "git@example.com:username/repo.git"
		err := validGitRepoURL(repoURL)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return nil for 'http://' and 'https://' URLs", func() {
		httpURL := "http://example.com/username/repo.git"
		httpsURL := "https://example.com/username/repo.git"
		errHTTP := validGitRepoURL(httpURL)
		errHTTPS := validGitRepoURL(httpsURL)
		Expect(errHTTP).NotTo(HaveOccurred())
		Expect(errHTTPS).NotTo(HaveOccurred())
	})

	It("should return an error for unsupported URL formats", func() {
		unsupportedURL := "ftp://example.com/username/repo.git"
		err := validGitRepoURL(unsupportedURL)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("repository URL must be either http/https"))
		Expect(err.Error()).To(ContainSubstring(unsupportedURL))
	})
})

var _ = Describe("compareMaps", func() {
	It("should return true for two empty maps", func() {
		m1 := make(map[string][]byte)
		m2 := make(map[string][]byte)
		Expect(compareMaps(m1, m2)).To(BeTrue())
	})

	It("should return false for maps of different sizes", func() {
		m1 := map[string][]byte{"key1": []byte("value1")}
		m2 := make(map[string][]byte)
		Expect(compareMaps(m1, m2)).To(BeFalse())
	})

	It("should return true for maps with the same keys and values", func() {
		m1 := map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")}
		m2 := map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")}
		Expect(compareMaps(m1, m2)).To(BeTrue())
	})

	It("should return false for maps with the same keys but different values", func() {
		m1 := map[string][]byte{"key1": []byte("value1")}
		m2 := map[string][]byte{"key1": []byte("differentValue")}
		Expect(compareMaps(m1, m2)).To(BeFalse())
	})

	It("should return false for maps with different keys", func() {
		m1 := map[string][]byte{"key1": []byte("value1")}
		m2 := map[string][]byte{"anotherKey": []byte("value1")}
		Expect(compareMaps(m1, m2)).To(BeFalse())
	})
})

var _ = Describe("newSecret", func() {
	var myname string = "my-secret"
	var myns string = "my-namespace"
	It("should create a secret with minimal input", func() {
		name := myname
		namespace := myns
		secret := newSecret(name, namespace, map[string][]byte{}, map[string]string{})
		Expect(secret.ObjectMeta.Name).To(Equal(name))
		Expect(secret.ObjectMeta.Namespace).To(Equal(namespace))
		Expect(secret.Data).To(BeEmpty())
		Expect(secret.Labels).To(BeEmpty())
	})

	It("should create a secret with full input", func() {
		name := myname
		namespace := myns
		data := map[string][]byte{"key": []byte("value")}
		labels := map[string]string{"app": "my-app"}
		secret := newSecret(name, namespace, data, labels)
		Expect(secret.ObjectMeta.Name).To(Equal(name))
		Expect(secret.ObjectMeta.Namespace).To(Equal(namespace))
		Expect(secret.Data).To(Equal(data))
		Expect(secret.Labels).To(Equal(labels))
	})

	It("should create a secret with only labels", func() {
		name := myname
		namespace := myns
		labels := map[string]string{"app": "my-app"}
		secret := newSecret(name, namespace, map[string][]byte{}, labels)
		Expect(secret.ObjectMeta.Name).To(Equal(name))
		Expect(secret.ObjectMeta.Namespace).To(Equal(namespace))
		Expect(secret.Data).To(BeEmpty())
		Expect(secret.Labels).To(Equal(labels))
	})

	It("should create a secret with only data", func() {
		name := myname
		namespace := myns
		data := map[string][]byte{"key": []byte("value")}
		secret := newSecret(name, namespace, data, map[string]string{})
		Expect(secret.ObjectMeta.Name).To(Equal(name))
		Expect(secret.ObjectMeta.Namespace).To(Equal(namespace))
		Expect(secret.Data).To(Equal(data))
		Expect(secret.Labels).To(BeEmpty())
	})
})

var _ = Describe("hasExperimentalCapability", func() {
	It("should return false for empty capabilities string", func() {
		Expect(hasExperimentalCapability("", "cap1")).To(BeFalse())
	})

	It("should return true for a single matching capability", func() {
		Expect(hasExperimentalCapability("cap1", "cap1")).To(BeTrue())
	})

	It("should return false for a single non-matching capability", func() {
		Expect(hasExperimentalCapability("cap2", "cap1")).To(BeFalse())
	})

	It("should return true for multiple capabilities with one matching", func() {
		Expect(hasExperimentalCapability("cap1,cap2,cap3", "cap2")).To(BeTrue())
	})

	It("should return false for multiple capabilities with none matching", func() {
		Expect(hasExperimentalCapability("cap1,cap2,cap3", "cap4")).To(BeFalse())
	})

	It("should return true for capabilities string containing spaces", func() {
		Expect(hasExperimentalCapability("cap1, cap2 , cap3", "cap2")).To(BeTrue())
	})
})

var _ = Describe("GetPatternConditionByStatus", func() {
	var (
		conditions      []api.PatternCondition
		conditionStatus corev1.ConditionStatus
		expectedIndex   int
		expectedResult  *api.PatternCondition
	)

	BeforeEach(func() {
		conditions = []api.PatternCondition{
			{Type: "ConditionA", Status: corev1.ConditionFalse},
			{Type: "ConditionB", Status: corev1.ConditionTrue},
		}
		conditionStatus = corev1.ConditionTrue
	})

	Context("when conditions are nil", func() {
		BeforeEach(func() {
			conditions = nil
			expectedIndex = -1
			expectedResult = nil
		})

		It("should return -1 and nil", func() {
			index, result := getPatternConditionByStatus(conditions, conditionStatus)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(BeNil())
		})
	})

	Context("when conditions are empty", func() {
		BeforeEach(func() {
			conditions = []api.PatternCondition{}
		})

		It("should return -1 and nil", func() {
			index, result := getPatternConditionByStatus(conditions, conditionStatus)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(BeNil())
		})
	})

	Context("when condition is found", func() {
		BeforeEach(func() {
			expectedIndex = 1
			expectedResult = &api.PatternCondition{Type: "ConditionB", Status: corev1.ConditionTrue}
		})

		It("should return the index and the condition", func() {
			index, result := getPatternConditionByStatus(conditions, conditionStatus)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when condition is not found", func() {
		BeforeEach(func() {
			expectedIndex = -1
			expectedResult = nil
			conditions[1].Status = corev1.ConditionFalse // Modify to ensure condition is not found
		})

		It("should return -1 and nil", func() {
			index, result := getPatternConditionByStatus(conditions, conditionStatus)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(BeNil())
		})
	})
})

var _ = Describe("GetPatternConditionByType", func() {
	var (
		conditions     []api.PatternCondition
		conditionType  api.PatternConditionType
		expectedIndex  int
		expectedResult *api.PatternCondition
	)

	JustBeforeEach(func() {
		// This runs after all BeforeEach blocks, just before the It block
		expectedIndex = -1
		expectedResult = nil
	})

	Context("when conditions are nil", func() {
		BeforeEach(func() {
			conditions = nil
			conditionType = "ConditionType1"
		})

		It("should return -1 and nil", func() {
			index, result := getPatternConditionByType(conditions, conditionType)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(BeNil())
		})
	})

	Context("when conditions are empty", func() {
		BeforeEach(func() {
			conditions = []api.PatternCondition{}
			conditionType = "ConditionType1"
		})

		It("should return -1 and nil", func() {
			index, result := getPatternConditionByType(conditions, conditionType)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(BeNil())
		})
	})

	Context("when condition is found", func() {
		BeforeEach(func() {
			conditions = []api.PatternCondition{
				{Type: "ConditionType1", Status: corev1.ConditionFalse},
				{Type: "ConditionType2", Status: corev1.ConditionTrue},
			}
			conditionType = "ConditionType2"

		})
		JustBeforeEach(func() {
			expectedIndex = 1
			expectedResult = &api.PatternCondition{Type: "ConditionType2", Status: corev1.ConditionTrue}
		})
		It("should return the index and the condition", func() {
			index, result := getPatternConditionByType(conditions, conditionType)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when condition is not found", func() {
		BeforeEach(func() {
			conditions = []api.PatternCondition{
				{Type: "ConditionType1", Status: corev1.ConditionFalse},
				{Type: "ConditionType3", Status: corev1.ConditionTrue},
			}
			conditionType = "ConditionType2"
		})

		It("should return -1 and nil", func() {
			index, result := getPatternConditionByType(conditions, conditionType)
			Expect(index).To(Equal(expectedIndex))
			Expect(result).To(BeNil())
		})
	})
})

var _ = Describe("GetCurrentClusterVersion", func() {
	var (
		clusterVersion *configv1.ClusterVersion
	)

	Context("when there are completed versions in the history", func() {
		BeforeEach(func() {
			clusterVersion = &configv1.ClusterVersion{
				Status: configv1.ClusterVersionStatus{
					History: []configv1.UpdateHistory{
						{State: "Completed", Version: "4.6.1"},
					},
					Desired: configv1.Release{
						Version: "4.7.0",
					},
				},
			}
		})

		It("should return the completed version", func() {
			version, err := getCurrentClusterVersion(clusterVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(version.String()).To(Equal("4.6.1"))
		})
	})

	Context("when there are no completed versions in the history", func() {
		BeforeEach(func() {
			clusterVersion = &configv1.ClusterVersion{
				Status: configv1.ClusterVersionStatus{
					History: []configv1.UpdateHistory{
						{State: "Partial", Version: "4.6.1"},
					},
					Desired: configv1.Release{
						Version: "4.7.0",
					},
				},
			}
		})

		It("should return the desired version", func() {
			version, err := getCurrentClusterVersion(clusterVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(version.String()).To(Equal("4.7.0"))
		})
	})
})

var _ = Describe("ParseAndReturnVersion", func() {
	Context("when the version string is valid", func() {
		It("should return the parsed version", func() {
			versionStr := "4.6.1"
			version, err := parseAndReturnVersion(versionStr)
			Expect(err).ToNot(HaveOccurred())
			Expect(version.String()).To(Equal(versionStr))
		})
	})

	Context("when the version string is invalid", func() {
		It("should return an error", func() {
			versionStr := "invalid-version"
			version, err := parseAndReturnVersion(versionStr)
			Expect(err).To(HaveOccurred())
			Expect(version).To(BeNil())
		})
	})
})
