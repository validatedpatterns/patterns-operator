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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-errors/errors"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	//+kubebuilder:scaffold:imports
)

// Self-signed test CA certificate for getHTTPSTransport tests
var testCACert = []byte(`-----BEGIN CERTIFICATE-----
MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zAKBggqhkjOPQQDAzASMRAw
DgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYwMDAw
WjASMRAwDgYDVQQKEwdBY21lIENvMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE7Jdx
McVMDPH0GQKXW9z+Xa0+H/GVvOdxDeGR5dEWBq4eFTkJ5x7+h/bfaSeQGVCBm/s
ZBeXnOJtIG01kv6mBcExZ6YGXpeLdpaIuKsFr7TMjjQ4L/HTPgwIvTAUUoYEo4GG
MIGDMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMB
Af8EBTADAQH/MB0GA1UdDgQWBBRWbLiq20RFEfwvuGBEdFaxdkQiMDAsBgNVHREE
JTAjgglsb2NhbGhvc3SHBH8AAAGHEAAAAAAAAAAAAAAAAAAAAAEKHBR0ZXN0MBIG
A1UdEAQLMAmCB3Rlc3RjYTAKBggqhkjOPQQDAwNnADBkAjBK9MEtFB6VYkOngzWd
Ft0LstEoFkHkWJxgSZ8WlKnmPPKQee3ZIB3JkKRfV2Y80cICMANZmsSy1HTRrbXI
Jfc+jIf39GhvPMfxR3BBfrIvdBH2oKC1PNi6N1iFYrPiKaMs6A==
-----END CERTIFICATE-----
`)

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
	It("should extract the repository name from various URL formats", func() {
		for _, testCase := range testCases {
			repoName, err := extractRepositoryName(testCase.inputURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(repoName).To(Equal(testCase.expectedName))
		}
	})

	Context("when the git URL is SSH format", func() {
		It("should extract the repository name correctly", func() {
			gitURL := "git@github.com:user/repo.git"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(repoName).To(Equal("repo"))
		})

		It("should report an error when there are too many :", func() {
			gitURL := "git@github.com:user:repo.git"
			repoName, err := extractRepositoryName(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(repoName).To(BeEmpty())
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

var _ = Describe("ExtractGitFQDNHostname", func() {
	Context("when the git URL is in SSH format", func() {
		It("should extract the FQDN hostname correctly", func() {
			gitURL := "git@github.com:user/repo.git"
			hostname, err := extractGitFQDNHostname(gitURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(hostname).To(Equal("github.com"))
		})

		It("should return an error for invalid SSH git URL", func() {
			gitURL := "git@github.com:@user/repo:extra"
			hostname, err := extractGitFQDNHostname(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(hostname).To(BeEmpty())
		})
	})

	Context("when the git URL is in HTTP/HTTPS format", func() {
		It("should extract the FQDN hostname correctly", func() {
			gitURL := "https://github.com/user/repo.git"
			hostname, err := extractGitFQDNHostname(gitURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(hostname).To(Equal("github.com"))
		})

		It("should return an error for invalid HTTP/HTTPS git URL", func() {
			gitURL := "https://github.com:user/repo.git"
			hostname, err := extractGitFQDNHostname(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(hostname).To(BeEmpty())
		})

		It("should return an error for non-absolute HTTP/HTTPS URL", func() {
			gitURL := "github.com/user/repo.git"
			hostname, err := extractGitFQDNHostname(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(hostname).To(BeEmpty())
		})

		It("should return an error for empty hostname in URL", func() {
			gitURL := "https:///user/repo.git"
			hostname, err := extractGitFQDNHostname(gitURL)
			Expect(err).To(HaveOccurred())
			Expect(hostname).To(BeEmpty())
		})
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

var _ = Describe("IntOrZero", func() {
	var (
		secret         map[string][]byte
		key            string
		expectedResult int64
	)

	Context("when the secret map is nil", func() {
		BeforeEach(func() {
			secret = nil
			expectedResult = int64(0)
		})
		It("should return 0 for the result", func() {
			result, err := IntOrZero(secret, key)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when an invalid key is selected from the secret map", func() {
		BeforeEach(func() {
			key = "key"
			secret = map[string][]byte{
				key: []byte("123"),
			}
			expectedResult = int64(0)
		})
		It("should return 0 for the result", func() {
			result, err := IntOrZero(secret, "invalid-key")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when the secret map is properly populated and a value is selected", func() {
		BeforeEach(func() {
			key = "key"
			secret = map[string][]byte{
				key: []byte("123"),
			}
			expectedResult = int64(123)
		})
		It("should return the correct integer value", func() {
			result, err := IntOrZero(secret, key)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when the secret map contains invalid integers", func() {
		BeforeEach(func() {
			key = "key"
			secret = map[string][]byte{
				key: []byte("invalid"),
			}
		})
		It("an error should be reported", func() {
			_, err := IntOrZero(secret, key)
			Expect(err).To(HaveOccurred())
		})
	})

})

var _ = Describe("detectGitAuthType", func() {
	var (
		secret         map[string][]byte
		expectedResult GitAuthenticationBackend
	)

	Context("When a secret containing no authentication details is provided", func() {
		BeforeEach(func() {
			secret = nil
			expectedResult = GitAuthNone
		})
		It("should return GitAuthNone", func() {
			result := detectGitAuthType(secret)
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when a username and password is provided", func() {
		BeforeEach(func() {
			secret = map[string][]byte{
				"username": []byte("myusername"),
				"password": []byte("mypassword"),
			}
			expectedResult = GitAuthPassword
		})
		It("should return GitAuthPassword", func() {
			result := detectGitAuthType(secret)
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when a SSH private key is provided", func() {
		BeforeEach(func() {
			secret = map[string][]byte{
				"sshPrivateKey": []byte("ssh-key-data"),
			}
			expectedResult = GitAuthSsh
		})
		It("should return GitAuthSsh", func() {
			result := detectGitAuthType(secret)
			Expect(result).To(Equal(expectedResult))
		})
	})

	Context("when a GitHub App is provided", func() {
		BeforeEach(func() {
			secret = map[string][]byte{
				"githubAppID":             []byte("github-app-id"),
				"githubAppInstallationID": []byte("github-app-installation-id"),
				"githubAppPrivateKey":     []byte("github-app-private-key"),
			}
			expectedResult = GitAuthGitHubApp
		})
		It("should return GitAuthGitHubApp", func() {
			result := detectGitAuthType(secret)
			Expect(result).To(Equal(expectedResult))
		})
	})

})

var _ = Describe("getGitHubAppAuthTransport", func() {
	var (
		clientset *fake.Clientset
		secret    map[string][]byte
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		secret = map[string][]byte{
			"githubAppPrivateKey": []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAuuCWq9FEvSUw9fhuQzs8D+fE/GHLTruKVxlWwECKhju/5yrf
Eougt6DGQXDDpPY8PtW9aDd45/8iIJJuCPrTU8poDN52qhI0VQPYqyMDcHJZQcXX
pfyCrwTrJf5o++l9sOP6IVZBIZTGtiKOPJNE04K/5UebCQ7mwDgPOdT7tXxmFzcO
1PW+uSq8XRF8PfvMDVAub04X9sm8TNGhkd5yoR9fXFRNjRvzBgc0uikvIVv7HyY2
HF1sk1MyC9qW0WpiiRPFoEnLh2m+qCnmlZUzeP8fmKZsCLmJazzFcjhX+ExU80fd
sNxpMfRO+hDenVFTOuJpSa6h88LY7GPBBsy8DQIDAQABAoIBAQC3Mi/CY7XVDj5/
AnllIw5wMS7kkyHxHtwxIj/u29ZwXOZ1QYvI7GQzX0K7KEZC0rigiHvTTH4UQAI+
mA2SdADy5Ts3UmZVtt7icJDYw8w9UXu6hK4wo+egl1vFtS9JtM1ouTSdtabHusdK
CXoSW/RevJBNvfJ34MnIqawTb30JnSrIWpnwASx4jfjj8vT+jcAGiNDCIptwQN1a
YHGsfmKt3OW0avu9r8Y5W+dgZLDXtEy7/jrVQRNiCNMZBc3WFNWcHzAOJbQfax3v
EOGkypI0lqD7CDUo7bFDL/D5FtCvKC8IgPfQ84x6kUObHcxHnQZra6VZ3I0IlMMg
adBrvIlBAoGBANtXFe8xzPvGCYwpwMuI7FSafKL7vGAh/OQDjgL/hATxEghMTggD
bLFHgp56UojaRCq3HtuBXzocUPyXdk+CiqX/Zj4+5O8fFGlS5PRVOhafOViYIyE4
11FZvR3rgFWq5TQ5tQMtRUHeq6WbMMnfZjciOGGt3kG5k2jnqrQU6jaVAoGBANoc
g0PjOHWTgus506e0Lkowel7NojFF5Gt5t/zoyexYCDm43FwvIW8uV/SdDEU9uXaG
05TlPGdKFaAqQ9kHSE/O0yrd4GCLDQmt3I1++GqD+JVzJ8uPhQhN7vxfOz3RVQ9+
DOk/7NGRBgdueUdRXMjg4NscEsfzupuZJglL82mZAoGAG/uTP83hselFBI27G/xe
8jg3WG+3S6hqZAiUEIvaourCey6I8frF3iQaZO+EIhN+iNiN5kEuDfLY3jDQljo4
SA86UwyhFmSnrPw3W3iYDZTIsyXNrYpb5fQF7ZBC8ir4TN5j2oDnCg1HZrxS0B5h
Iv2JpeSRq17qkIKlw427h7UCgYBmtD5rXTdcxhVDxnsP4Rxa+vDka1gQc6TXpv0o
LkXG8L0O0SmSju7jd6MbIEiC4knOsjY3SqpiyNPeE4jXTUKTsgRljwz06QU+pYvR
ZRR8s5/+X7dBd1dhTbFXTVCMD2JKZUSXIO7Wz79TCIY7OujB/oJjKpj9ZptcYYUz
o3v/IQKBgQCLPnHN78nJsuaRGPL2Ypg8Ku/u4xQPA//ng0cF6d2TIDXLpBKbtUOQ
Gt2KENSCnHhtaj/RnMlG4I2RVTUTkKn5u81cY+XuGbvjVt2MDCcTniRzOiTkHXgO
9lJ+GXjeWhXo+wKlT5YX4s0U8AZIQNQU/Rtrx8vGu9d1SbKiF7Mnlw==
-----END RSA PRIVATE KEY-----`),
		}
	})

	Context("When empty values are provided", func() {
		BeforeEach(func() {
			secret = map[string][]byte{}
		})

		It("a an error should be returned", func() {
			transport, err := getGitHubAppAuthTransport(clientset, secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to initialize GitHub installation transport"))
			Expect(transport).To(BeNil())
		})
	})

	Context("When the default values are provided", func() {

		It("no errors should be returned", func() {
			transport, err := getGitHubAppAuthTransport(clientset, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(transport).ToNot(BeNil())
		})
		It("default github API address should be present", func() {
			transport, err := getGitHubAppAuthTransport(clientset, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(transport.BaseURL).To(Equal("https://api.github.com"))
		})
	})

	Context("When a custom GitHub Enterprise Address is provided", func() {
		JustBeforeEach(func() {
			secret["githubAppEnterpriseBaseUrl"] = []byte("https://github.mycompany.com/api/v3")
		})

		It("custom github API address should be present", func() {
			transport, err := getGitHubAppAuthTransport(clientset, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(transport.BaseURL).To(Equal("https://github.mycompany.com/api/v3"))
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

var _ = Describe("GenerateRandomPassword", func() {
	Context("when generating a random password", func() {
		It("should return a password of the correct length", func() {
			length := 32
			password, err := GenerateRandomPassword(length, DefaultRandRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(password)).To(BeNumerically(">", length)) // base64 encoding increases the length
		})

		It("should return different passwords on subsequent calls", func() {
			length := 32
			password1, err1 := GenerateRandomPassword(length, DefaultRandRead)
			password2, err2 := GenerateRandomPassword(length, DefaultRandRead)
			Expect(err1).ToNot(HaveOccurred())
			Expect(err2).ToNot(HaveOccurred())
			Expect(password1).ToNot(Equal(password2))
		})
	})

	Context("when an error occurs while generating random bytes", func() {
		It("should return an error", func() {
			length := 32

			// Define a mock randRead function that returns an error
			mockRandRead := func([]byte) (int, error) {
				return 0, errors.New("random error")
			}

			password, err := GenerateRandomPassword(length, mockRandRead)
			Expect(err).To(HaveOccurred())
			Expect(password).To(BeEmpty())
		})
	})
})

var _ = Describe("CreateTrustedBundleCM", func() {
	var (
		clientset *fake.Clientset
		namespace string
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		namespace = "default"
	})

	Context("when the ConfigMap does not exist", func() {
		It("should create the ConfigMap", func() {
			err := createTrustedBundleCM(clientset, namespace)
			Expect(err).ToNot(HaveOccurred())

			// Verify that the ConfigMap was created
			cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "trusted-ca-bundle", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())
			Expect(cm.Labels["config.openshift.io/inject-trusted-cabundle"]).To(Equal("true"))
		})
	})

	Context("when the ConfigMap already exists", func() {
		BeforeEach(func() {
			// Pre-create the ConfigMap
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "trusted-ca-bundle",
					Namespace: namespace,
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not return an error", func() {
			err := createTrustedBundleCM(clientset, namespace)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when an error occurs while checking for the ConfigMap", func() {
		It("should return the error", func() {
			// Inject an error into the fake client
			clientset.PrependReactor("get", "configmaps", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, kubeerrors.NewInternalError(fmt.Errorf("some error"))
			})

			err := createTrustedBundleCM(clientset, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some error"))
		})
	})

	Context("when an error occurs while creating the ConfigMap", func() {
		It("should return the error", func() {
			// Inject an error into the fake client
			clientset.PrependReactor("create", "configmaps", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, kubeerrors.NewInternalError(fmt.Errorf("some create error"))
			})

			err := createTrustedBundleCM(clientset, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some create error"))
		})
	})
})

var _ = Describe("WriteConfigMapKeyToFile", func() {
	var (
		clientset     *fake.Clientset
		namespace     string
		configMap     *corev1.ConfigMap
		configMapName string
		key           string
		filePath      string
		appendToFile  bool
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		namespace = "default"
		configMapName = "test-configmap"
		key = "test-key"
		appendToFile = false

		// Create a temporary file for testing
		tmpFile, err := os.CreateTemp("", "testfile")
		Expect(err).ToNot(HaveOccurred())
		filePath = tmpFile.Name()
		tmpFile.Close()
	})

	AfterEach(func() {
		os.Remove(filePath)
	})

	Context("when the ConfigMap and key exist", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					key: "test-value",
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should write the value to the file", func() {
			err := writeConfigMapKeyToFile(clientset, namespace, configMapName, key, filePath, appendToFile)
			Expect(err).ToNot(HaveOccurred())

			// Verify the content of the file
			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("test-value\n"))
		})

		It("should append the value to the file if appendToFile is true", func() {
			// Write initial content to the file
			initialContent := "initial-content\n"
			err := os.WriteFile(filePath, []byte(initialContent), 0600)
			Expect(err).ToNot(HaveOccurred())

			// Set appendToFile to true
			appendToFile = true

			err = writeConfigMapKeyToFile(clientset, namespace, configMapName, key, filePath, appendToFile)
			Expect(err).ToNot(HaveOccurred())

			// Verify the content of the file
			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal(initialContent + "test-value\n"))
		})
	})

	Context("when the ConfigMap does not exist", func() {
		It("should return an error", func() {
			err := writeConfigMapKeyToFile(clientset, namespace, configMapName, key, filePath, appendToFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("error getting ConfigMap %s in namespace %s", configMapName, namespace)))
		})
	})

	Context("when the key does not exist in the ConfigMap", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					"another-key": "another-value",
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error", func() {
			err := writeConfigMapKeyToFile(clientset, namespace, configMapName, key, filePath, appendToFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("key %s not found in ConfigMap %s", key, configMapName)))
		})
	})

	Context("when an error occurs while opening the file", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					key: "test-value",
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error", func() {
			invalidFilePath := "/invalid-path/testfile"

			err := writeConfigMapKeyToFile(clientset, namespace, configMapName, key, invalidFilePath, appendToFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error opening file"))
		})
	})
})

var _ = Describe("GetConfigMapKey", func() {
	var (
		clientset     *fake.Clientset
		namespace     string
		configMap     *corev1.ConfigMap
		configMapName string
		key           string
		value         string
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		namespace = "default"
		configMapName = "test-configmap"
		key = "test-key"
		value = "test-value"
	})

	Context("when the ConfigMap and key exist", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					key: value,
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the value for the specified key", func() {
			result, err := getConfigMapKey(clientset, namespace, configMapName, key)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(value))
		})
	})

	Context("when the ConfigMap does not exist", func() {
		It("should return an error", func() {
			result, err := getConfigMapKey(clientset, namespace, configMapName, key)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("error getting ConfigMap %s in namespace %s", configMapName, namespace)))
		})
	})

	Context("when the key does not exist in the ConfigMap", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					"another-key": "another-value",
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error", func() {
			result, err := getConfigMapKey(clientset, namespace, configMapName, key)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("key %s not found in ConfigMap %s", key, configMapName)))
		})
	})

	Context("when an error occurs while getting the ConfigMap", func() {
		It("should return an error", func() {
			// Inject an error into the fake client
			clientset.PrependReactor("get", "configmaps", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, kubeerrors.NewInternalError(fmt.Errorf("some error"))
			})

			result, err := getConfigMapKey(clientset, namespace, configMapName, key)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
			Expect(err.Error()).To(ContainSubstring("some error"))
		})
	})
})

func parsePEM(certPEM string) *x509.Certificate {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		fmt.Printf("Could not decode: %s\n", certPEM)
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Printf("Could not parse: %s\n", err)
		return nil
	}
	return cert
}

var _ = Describe("GetHTTPSTransport", func() {
	var (
		clientset       *fake.Clientset
		namespace       string
		kubeRootCA      string
		trustedCABundle string
		configMapName1  string
		configMapName2  string
		key1            string
		key2            string
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		namespace = "openshift-config-managed"
		kubeRootCA = `-----BEGIN CERTIFICATE-----
MIIDUTCCAjmgAwIBAgIIWt3N131wkCwwDQYJKoZIhvcNAQELBQAwNjE0MDIGA1UE
Awwrb3BlbnNoaWZ0LXNlcnZpY2Utc2VydmluZy1zaWduZXJAMTcxODUxODc1OTAe
Fw0yNDA2MTYwNjE5MTlaFw0yNjA4MTUwNjE5MjBaMDYxNDAyBgNVBAMMK29wZW5z
aGlmdC1zZXJ2aWNlLXNlcnZpbmctc2lnbmVyQDE3MTg1MTg3NTkwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDFi3A7uIICJBWma5+Jwep4VShGTcdSfL6u
Flha8tM2TvDXf4Q/Mo4ZquX4Jrg+FwS2LxeChrpCv1PyEs7e1g7lH20FiIVSCPmC
PsxwprxnwXBYzbANr0m3WUt4SSR05JAkthCMm/FqoZHBP97Ih/E4yNCGJ2x3dEE8
DS18IN+/0yuGROmQZvB1j4njsa8tUQVcYmM+XoETxOViGhbL9S0B3YQA1mRTSyne
SE/k/1JffYXyBoU3IkPjCnrOOjoXp87TTnYZrElAJ7PTEScLwuhWQXM1iSLKfZBG
FWqjF4k7awTE0fSLXzEPOPrHO5gk+s/osu1AvHUifx3+gN+Xya0/AgMBAAGjYzBh
MA4GA1UdDwEB/wQEAwICpDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRDG/i4
PSo63fBMyNOWI1W0pUN+WzAfBgNVHSMEGDAWgBRDG/i4PSo63fBMyNOWI1W0pUN+
WzANBgkqhkiG9w0BAQsFAAOCAQEAR/jMbOO531vkFus6df/EKgyY9CFR1xsSZlJu
AhI0G79vmddMnuM+P0PvFGkxTzjUYjmQ0ICiLXAO7nf7aJNTB7GqHGyX87GjnOZa
HLCARIrSm/FNSEJt3XjAqjXMbLM3sCM1DZVw5XzW8DaIctK9eYTyAoz9zC7Nb8hg
d6UDTwBDD60GYz2ZH6eSD5513s2uZZwPGGQ2pceJ06Oc6nQW2Pf+fLWlweSJrC6u
o20m7jT37A4pzD9p5XTw/QL2Nns6ReJtYpGpOElsqPvRV0YS72yQsp6AoI6VWDmH
vqVDy2lIuMbqRGLTpEOxLFU3T81/jk0IPGZ0qyWTplbobN3/yQ==
-----END CERTIFICATE-----`
		trustedCABundle = `-----BEGIN CERTIFICATE-----
MIIDMjCCAhqgAwIBAgIIUuYsPPrEW8cwDQYJKoZIhvcNAQELBQAwNzESMBAGA1UE
CxMJb3BlbnNoaWZ0MSEwHwYDVQQDExhrdWJlLWFwaXNlcnZlci1sYi1zaWduZXIw
HhcNMjQwNjE2MDYwNzExWhcNMzQwNjE0MDYwNzExWjA3MRIwEAYDVQQLEwlvcGVu
c2hpZnQxITAfBgNVBAMTGGt1YmUtYXBpc2VydmVyLWxiLXNpZ25lcjCCASIwDQYJ
KoZIhvcNAQEBBQADggEPADCCAQoCggEBANxEUZA/DSmiqccIas7RIygbCQy+pdqC
BITCG50vZWDoRt/2okXBlcdVe7auSRQi64vdpLV7hk4MDMHS8itMebiZCz80X7d6
QHGlN6xkb4/8LYBeTZxthv/6ButlHDoBHw3kj627cj5BhDfNyqSrG4heyc8hJxPa
SiFMf+30QBSYzjcujPYENnLEfvZkhriJjwVaNIgoQd5Ti99zQYqhMttmJoRxYS0x
P5IdGU0Tngsex54OPuaxhowTE4bkyOShjvkkby9NL20Sab0sqwhGPhjzFD4iqF1X
pKfTx3q6IDhnXqh5jaQ7rGL01aOa8+Aeo7i8GGXL3cFtlXFz8JTH2m8CAwEAAaNC
MEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFCKd
IDiv1Fzh83q1D2OT+ymL3w8pMA0GCSqGSIb3DQEBCwUAA4IBAQCQzUJVzR7izaQG
ellU6uWUw/syXmgVy0DIj5FLxBHWvay15gRd5iTHtujBWEOVSuFboD+nPyVSMuFv
y4X/QT0KzohWTOg5PcPSm+0/dxSsXD9jvpi5kmnrGWiBC6WH2wwibSZ+hU/Q1fWj
IzB+UEPRiXlNA/9r6pKF0VX8TfiTfKtOETk4ZoTMPRwbafZ6J6neciF6+I4BvPIf
LEpkMLMAMU6wfSCRuGzMjIHbXTuFN/Aokt354FB7ooL3jgCBgiJD8/mDeEZv0ACd
dQLf0FVEbkwJvrQj3kN9A8xfz3L4463AC1v3kmAMtLZSDEyqLM1zXW0Eifqu98d1
f3k4g5eL
-----END CERTIFICATE-----`
		configMapName1 = "kube-root-ca.crt"
		configMapName2 = "trusted-ca-bundle"
		key1 = "ca.crt"
		key2 = "ca-bundle.crt"
	})

	Context("when both ConfigMaps and keys exist", func() {
		BeforeEach(func() {
			configMap1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName1,
					Namespace: namespace,
				},
				Data: map[string]string{
					key1: kubeRootCA,
				},
			}
			configMap2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName2,
					Namespace: namespace,
				},
				Data: map[string]string{
					key2: trustedCABundle,
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap1, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap2, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create a transport with the combined CA certificates", func() {
			transport := getHTTPSTransport(clientset)
			Expect(transport).ToNot(BeNil())
			caCertPool := transport.TLSClientConfig.RootCAs
			Expect(caCertPool).ToNot(BeNil())

			// Verify the certificates were added to the pool
			kubeRootCert := parsePEM(kubeRootCA)
			Expect(kubeRootCert).ToNot(BeNil())
			trustedCACert := parsePEM(trustedCABundle)
			Expect(trustedCACert).ToNot(BeNil())

			Expect(caCertPool.Subjects()).To(ContainElement(kubeRootCert.RawSubject))  //nolint:staticcheck
			Expect(caCertPool.Subjects()).To(ContainElement(trustedCACert.RawSubject)) //nolint:staticcheck
		})
	})

	Context("when one of the ConfigMaps does not exist", func() {
		BeforeEach(func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName1,
					Namespace: namespace,
				},
				Data: map[string]string{
					key1: kubeRootCA,
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create a transport with the available CA certificate", func() {
			transport := getHTTPSTransport(clientset)
			Expect(transport).ToNot(BeNil())

			caCertPool := transport.TLSClientConfig.RootCAs
			Expect(caCertPool).ToNot(BeNil())

			// Verify the certificate was added to the pool
			kubeRootCert := parsePEM(kubeRootCA)
			Expect(kubeRootCert).ToNot(BeNil())

			Expect(caCertPool.Subjects()).To(ContainElement(kubeRootCert.RawSubject)) //nolint:staticcheck
		})
	})

	Context("when both ConfigMaps do not exist", func() {
		It("should fallback to system CA certificates", func() {
			transport := getHTTPSTransport(clientset)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})

	Context("when an error occurs while getting a ConfigMap", func() {
		It("should print an error message and fallback to system CA certificates", func() {
			clientset.PrependReactor("get", "configmaps", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("some error")
			})

			transport := getHTTPSTransport(clientset)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})
})

var _ = Describe("createNamespace", func() {
	var (
		kubeClient kubernetes.Interface
		namespace  string
	)

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		namespace = "test-ns"
	})

	It("should not return an error if the namespace already exists", func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err := kubeClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		err = createNamespace(kubeClient, namespace)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should create the namespace if it does not exist", func() {
		err := createNamespace(kubeClient, namespace)
		Expect(err).ToNot(HaveOccurred())

		_, err = kubeClient.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return an error if there is an error checking if the namespace exists", func() {
		kubeClient.(*fake.Clientset).PrependReactor("get", "namespaces", func(testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, kubeerrors.NewInternalError(fmt.Errorf("internal error"))
		})

		err := createNamespace(kubeClient, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("internal error"))
	})

	It("should return an error if there is an error creating the namespace", func() {
		kubeClient.(*fake.Clientset).PrependReactor("get", "namespaces", func(testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, kubeerrors.NewNotFound(corev1.Resource("namespace"), namespace)
		})
		kubeClient.(*fake.Clientset).PrependReactor("create", "namespaces", func(testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, kubeerrors.NewInternalError(fmt.Errorf("internal error"))
		})
		err := createNamespace(kubeClient, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("internal error"))
	})
})

var _ = Describe("hasExperimentalCapability", func() {
	Context("when capability exists in comma-separated list", func() {
		It("should return true", func() {
			Expect(hasExperimentalCapability("cap1,cap2,cap3", "cap2")).To(BeTrue())
		})
	})

	Context("when capability does not exist", func() {
		It("should return false", func() {
			Expect(hasExperimentalCapability("cap1,cap2,cap3", "cap4")).To(BeFalse())
		})
	})

	Context("with empty capabilities string", func() {
		It("should return false", func() {
			Expect(hasExperimentalCapability("", "cap1")).To(BeFalse())
		})
	})

	Context("with single capability", func() {
		It("should return true if matches", func() {
			Expect(hasExperimentalCapability("cap1", "cap1")).To(BeTrue())
		})
	})

	Context("with whitespace around capabilities", func() {
		It("should handle trimmed comparison", func() {
			Expect(hasExperimentalCapability("cap1, cap2, cap3", "cap2")).To(BeTrue())
		})
	})
})

var _ = Describe("getHTTPSTransport", func() {
	Context("with nil client", func() {
		It("should return transport with system certs", func() {
			transport := getHTTPSTransport(nil)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig).ToNot(BeNil())
		})
	})

	Context("with fake client with no configmaps", func() {
		It("should return transport with system certs", func() {
			kubeClient := fake.NewSimpleClientset()
			transport := getHTTPSTransport(kubeClient)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig).ToNot(BeNil())
		})
	})

	Context("with fake client with cert configmaps", func() {
		It("should return transport with custom certs", func() {
			kubeClient := fake.NewSimpleClientset()
			// Create kube-root-ca configmap
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-root-ca.crt",
					Namespace: "openshift-config-managed",
				},
				Data: map[string]string{
					"ca.crt": "-----BEGIN CERTIFICATE-----\n" +
						"MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw\n" +
						"DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow\n" +
						"EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABLU3\n" +
						"jSayahkJYT5/UqIqViZFMVh16yrQ1mOA8V/k3H8Pk/DL1tJ1yXYEptzhKELNJIjp\n" +
						"zUv0jVJHPnLGVaikzlKjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr\n" +
						"BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1\n" +
						"NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2wpSek3WdNcr\n" +
						"jSuvziv6OERWSEZObKHVIJl/Cj9SWWECIGB/W0PCjZjKXBzgoW0OzXRiDP/WRxW6\n" +
						"frNHC7GJcIqs\n-----END CERTIFICATE-----\n",
				},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("openshift-config-managed").Create(context.TODO(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			transport := getHTTPSTransport(kubeClient)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})
})

var _ = Describe("compareMaps", func() {
	Context("with identical maps", func() {
		It("should return true", func() {
			m1 := map[string][]byte{"key1": []byte("val1"), "key2": []byte("val2")}
			m2 := map[string][]byte{"key1": []byte("val1"), "key2": []byte("val2")}
			Expect(compareMaps(m1, m2)).To(BeTrue())
		})
	})

	Context("with different lengths", func() {
		It("should return false", func() {
			m1 := map[string][]byte{"key1": []byte("val1")}
			m2 := map[string][]byte{"key1": []byte("val1"), "key2": []byte("val2")}
			Expect(compareMaps(m1, m2)).To(BeFalse())
		})
	})

	Context("with different values", func() {
		It("should return false", func() {
			m1 := map[string][]byte{"key1": []byte("val1")}
			m2 := map[string][]byte{"key1": []byte("val2")}
			Expect(compareMaps(m1, m2)).To(BeFalse())
		})
	})

	Context("with different keys", func() {
		It("should return false", func() {
			m1 := map[string][]byte{"key1": []byte("val1")}
			m2 := map[string][]byte{"key2": []byte("val1")}
			Expect(compareMaps(m1, m2)).To(BeFalse())
		})
	})

	Context("with empty maps", func() {
		It("should return true", func() {
			m1 := map[string][]byte{}
			m2 := map[string][]byte{}
			Expect(compareMaps(m1, m2)).To(BeTrue())
		})
	})
})

var _ = Describe("GenerateRandomPassword", func() {
	Context("with default random reader", func() {
		It("should generate a password of expected length", func() {
			password, err := GenerateRandomPassword(15, DefaultRandRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(password).ToNot(BeEmpty())
			// base64 encoded 15 bytes = 20 chars
			Expect(password).To(HaveLen(20))
		})
	})

	Context("with failing random reader", func() {
		It("should return an error", func() {
			failReader := func(b []byte) (int, error) {
				return 0, fmt.Errorf("random read failed")
			}
			_, err := GenerateRandomPassword(15, failReader)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("random read failed"))
		})
	})
})

var _ = Describe("writeConfigMapKeyToFile", func() {
	var kubeClient kubernetes.Interface
	var td string

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		td = createTempDir("vp-write-cm-test")
	})
	AfterEach(func() {
		cleanupTempDir(td)
	})

	Context("when configmap exists and key is found", func() {
		It("should write the value to the file", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
				Data:       map[string]string{"ca.crt": "cert-data"},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("test-ns").Create(context.TODO(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			filePath := filepath.Join(td, "ca.crt")
			err = writeConfigMapKeyToFile(kubeClient, "test-ns", "test-cm", "ca.crt", filePath, false)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("cert-data\n"))
		})
	})

	Context("when configmap does not exist", func() {
		It("should return an error", func() {
			filePath := filepath.Join(td, "ca.crt")
			err := writeConfigMapKeyToFile(kubeClient, "test-ns", "nonexistent", "ca.crt", filePath, false)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when key does not exist in configmap", func() {
		It("should return an error", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
				Data:       map[string]string{"other-key": "data"},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("test-ns").Create(context.TODO(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			filePath := filepath.Join(td, "ca.crt")
			err = writeConfigMapKeyToFile(kubeClient, "test-ns", "test-cm", "ca.crt", filePath, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key ca.crt not found"))
		})
	})

	Context("when appending to file", func() {
		It("should append the content", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
				Data:       map[string]string{"ca.crt": "cert-data"},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("test-ns").Create(context.TODO(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			filePath := filepath.Join(td, "ca.crt")
			err = os.WriteFile(filePath, []byte("existing-data\n"), 0600)
			Expect(err).ToNot(HaveOccurred())

			err = writeConfigMapKeyToFile(kubeClient, "test-ns", "test-cm", "ca.crt", filePath, true)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("existing-data"))
			Expect(string(content)).To(ContainSubstring("cert-data"))
		})
	})
})

var _ = Describe("getConfigMapKey", func() {
	var kubeClient kubernetes.Interface

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
	})

	Context("when configmap and key exist", func() {
		It("should return the value", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
				Data:       map[string]string{"mykey": "myvalue"},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("test-ns").Create(context.TODO(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			val, err := getConfigMapKey(kubeClient, "test-ns", "test-cm", "mykey")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal("myvalue"))
		})
	})

	Context("when configmap does not exist", func() {
		It("should return an error", func() {
			_, err := getConfigMapKey(kubeClient, "test-ns", "nonexistent", "mykey")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when key does not exist", func() {
		It("should return an error", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
				Data:       map[string]string{"otherkey": "othervalue"},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("test-ns").Create(context.TODO(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			_, err = getConfigMapKey(kubeClient, "test-ns", "test-cm", "mykey")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key mykey not found"))
		})
	})
})

var _ = Describe("createTrustedBundleCM", func() {
	var kubeClient kubernetes.Interface

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
	})

	Context("when configmap does not exist", func() {
		It("should create it", func() {
			err := createTrustedBundleCM(kubeClient, "test-ns")
			Expect(err).ToNot(HaveOccurred())

			cm, err := kubeClient.CoreV1().ConfigMaps("test-ns").Get(context.TODO(), "trusted-ca-bundle", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(cm.Labels["config.openshift.io/inject-trusted-cabundle"]).To(Equal("true"))
		})
	})

	Context("when configmap already exists", func() {
		It("should not error", func() {
			err := createTrustedBundleCM(kubeClient, "test-ns")
			Expect(err).ToNot(HaveOccurred())

			// Call again
			err = createTrustedBundleCM(kubeClient, "test-ns")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("logOnce", func() {
	BeforeEach(func() {
		// Reset the logKeys map before each test
		logKeys = map[string]bool{}
	})

	It("should log a message the first time", func() {
		logOnce("test message")
		Expect(logKeys).To(HaveKey("test message"))
	})

	It("should not add duplicate entries", func() {
		logOnce("duplicate message")
		logOnce("duplicate message")
		Expect(logKeys).To(HaveLen(1))
	})

	It("should handle different messages", func() {
		logOnce("message 1")
		logOnce("message 2")
		Expect(logKeys).To(HaveLen(2))
	})
})

var _ = Describe("IsCommonSlimmed", func() {
	var td string

	BeforeEach(func() {
		td = createTempDir("vp-slimmed-test")
	})
	AfterEach(func() {
		cleanupTempDir(td)
	})

	Context("when common/operator-install exists", func() {
		It("should return false (not slimmed)", func() {
			err := os.MkdirAll(filepath.Join(td, "common", "operator-install"), 0755)
			Expect(err).ToNot(HaveOccurred())
			Expect(IsCommonSlimmed(td)).To(BeFalse())
		})
	})

	Context("when common/operator-install does not exist", func() {
		It("should return true (slimmed)", func() {
			Expect(IsCommonSlimmed(td)).To(BeTrue())
		})
	})

	Context("when path does not exist", func() {
		It("should return true (slimmed)", func() {
			Expect(IsCommonSlimmed("/nonexistent/path")).To(BeTrue())
		})
	})
})

var _ = Describe("IntOrZero", func() {
	Context("when key exists with valid integer", func() {
		It("should return the integer value", func() {
			secret := map[string][]byte{"count": []byte("42")}
			val, err := IntOrZero(secret, "count")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(int64(42)))
		})
	})

	Context("when key does not exist", func() {
		It("should return 0", func() {
			secret := map[string][]byte{}
			val, err := IntOrZero(secret, "missing")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(int64(0)))
		})
	})

	Context("when key exists with invalid integer", func() {
		It("should return an error", func() {
			secret := map[string][]byte{"count": []byte("not-a-number")}
			_, err := IntOrZero(secret, "count")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when key exists with negative integer", func() {
		It("should return the negative value", func() {
			secret := map[string][]byte{"count": []byte("-5")}
			val, err := IntOrZero(secret, "count")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(int64(-5)))
		})
	})
})

var _ = Describe("getClusterWideArgoNamespace", func() {
	It("should return the ApplicationNamespace", func() {
		Expect(getClusterWideArgoNamespace()).To(Equal(ApplicationNamespace))
	})
})

var _ = Describe("Pattern condition search functions", func() {
	var conditions []api.PatternCondition

	BeforeEach(func() {
		conditions = []api.PatternCondition{
			{
				Type:   api.Synced,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   api.Degraded,
				Status: corev1.ConditionFalse,
			},
		}
	})

	Describe("getPatternConditionByStatus", func() {
		Context("when conditions is nil", func() {
			It("should return -1 and nil", func() {
				idx, cond := getPatternConditionByStatus(nil, corev1.ConditionTrue)
				Expect(idx).To(Equal(-1))
				Expect(cond).To(BeNil())
			})
		})

		Context("when condition exists", func() {
			It("should return the index and condition", func() {
				idx, cond := getPatternConditionByStatus(conditions, corev1.ConditionTrue)
				Expect(idx).To(Equal(0))
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(api.Synced))
			})
		})

		Context("when condition does not exist", func() {
			It("should return -1 and nil", func() {
				idx, cond := getPatternConditionByStatus(conditions, corev1.ConditionUnknown)
				Expect(idx).To(Equal(-1))
				Expect(cond).To(BeNil())
			})
		})
	})

	Describe("getPatternConditionByType", func() {
		Context("when conditions is nil", func() {
			It("should return -1 and nil", func() {
				idx, cond := getPatternConditionByType(nil, api.Synced)
				Expect(idx).To(Equal(-1))
				Expect(cond).To(BeNil())
			})
		})

		Context("when condition type exists", func() {
			It("should return the index and condition", func() {
				idx, cond := getPatternConditionByType(conditions, api.Degraded)
				Expect(idx).To(Equal(1))
				Expect(cond).ToNot(BeNil())
				Expect(cond.Status).To(Equal(corev1.ConditionFalse))
			})
		})

		Context("when condition type does not exist", func() {
			It("should return -1 and nil", func() {
				idx, cond := getPatternConditionByType(conditions, api.Unknown)
				Expect(idx).To(Equal(-1))
				Expect(cond).To(BeNil())
			})
		})
	})
})

var _ = Describe("parseAndReturnVersion", func() {
	Context("with a valid version string", func() {
		It("should return the parsed version", func() {
			v, err := parseAndReturnVersion("4.12.5")
			Expect(err).ToNot(HaveOccurred())
			Expect(v).ToNot(BeNil())
			Expect(v.Major()).To(Equal(uint64(4)))
			Expect(v.Minor()).To(Equal(uint64(12)))
			Expect(v.Patch()).To(Equal(uint64(5)))
		})
	})

	Context("with an invalid version string", func() {
		It("should return an error", func() {
			_, err := parseAndReturnVersion("not-a-version")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with empty string", func() {
		It("should return an error", func() {
			_, err := parseAndReturnVersion("")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("getCurrentClusterVersion", func() {
	Context("with completed history entry", func() {
		It("should return the completed version", func() {
			cv := &configv1.ClusterVersion{
				Status: configv1.ClusterVersionStatus{
					History: []configv1.UpdateHistory{
						{Version: "4.13.5", State: "Partial"},
						{Version: "4.13.4", State: "Completed"},
					},
					Desired: configv1.Release{Version: "4.13.5"},
				},
			}
			v, err := getCurrentClusterVersion(cv)
			Expect(err).ToNot(HaveOccurred())
			Expect(v.String()).To(Equal("4.13.4"))
		})
	})

	Context("with no completed history", func() {
		It("should fall back to desired version", func() {
			cv := &configv1.ClusterVersion{
				Status: configv1.ClusterVersionStatus{
					History: []configv1.UpdateHistory{
						{Version: "4.13.5", State: "Partial"},
					},
					Desired: configv1.Release{Version: "4.13.5"},
				},
			}
			v, err := getCurrentClusterVersion(cv)
			Expect(err).ToNot(HaveOccurred())
			Expect(v.String()).To(Equal("4.13.5"))
		})
	})

	Context("with empty history", func() {
		It("should fall back to desired version", func() {
			cv := &configv1.ClusterVersion{
				Status: configv1.ClusterVersionStatus{
					Desired: configv1.Release{Version: "4.12.0"},
				},
			}
			v, err := getCurrentClusterVersion(cv)
			Expect(err).ToNot(HaveOccurred())
			Expect(v.String()).To(Equal("4.12.0"))
		})
	})
})

var _ = Describe("newSecret", func() {
	It("should create a secret with correct properties", func() {
		data := map[string][]byte{"key": []byte("value")}
		labels := map[string]string{"app": "test"}
		s := newSecret("my-secret", "my-ns", data, labels)
		Expect(s.Name).To(Equal("my-secret"))
		Expect(s.Namespace).To(Equal("my-ns"))
		Expect(s.Data).To(Equal(data))
		Expect(s.Labels).To(Equal(labels))
	})
})

var _ = Describe("DropLocalGitPaths", func() {
	It("should remove the vp temp folder", func() {
		td := filepath.Join(os.TempDir(), VPTmpFolder, "test-drop")
		err := os.MkdirAll(td, 0755)
		Expect(err).ToNot(HaveOccurred())

		err = DropLocalGitPaths()
		Expect(err).ToNot(HaveOccurred())

		_, err = os.Stat(td)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("should not error if folder does not exist", func() {
		err := DropLocalGitPaths()
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("getPatternConditionByStatus", func() {
	var conditions []api.PatternCondition

	BeforeEach(func() {
		conditions = []api.PatternCondition{
			{
				Type:   api.GitInSync,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   api.Degraded,
				Status: corev1.ConditionFalse,
			},
			{
				Type:   api.Progressing,
				Status: corev1.ConditionUnknown,
			},
		}
	})

	Context("when condition with given status exists", func() {
		It("should return the index and the condition for ConditionTrue", func() {
			idx, cond := getPatternConditionByStatus(conditions, corev1.ConditionTrue)
			Expect(idx).To(Equal(0))
			Expect(cond).ToNot(BeNil())
			Expect(cond.Type).To(Equal(api.GitInSync))
		})

		It("should return the index and the condition for ConditionFalse", func() {
			idx, cond := getPatternConditionByStatus(conditions, corev1.ConditionFalse)
			Expect(idx).To(Equal(1))
			Expect(cond).ToNot(BeNil())
			Expect(cond.Type).To(Equal(api.Degraded))
		})

		It("should return the index and the condition for ConditionUnknown", func() {
			idx, cond := getPatternConditionByStatus(conditions, corev1.ConditionUnknown)
			Expect(idx).To(Equal(2))
			Expect(cond).ToNot(BeNil())
			Expect(cond.Type).To(Equal(api.Progressing))
		})
	})

	Context("when condition with given status does not exist", func() {
		It("should return -1 and nil", func() {
			// All statuses are accounted for, so create a new slice with only one status
			limited := []api.PatternCondition{
				{Type: api.GitInSync, Status: corev1.ConditionTrue},
			}
			idx, cond := getPatternConditionByStatus(limited, corev1.ConditionFalse)
			Expect(idx).To(Equal(-1))
			Expect(cond).To(BeNil())
		})
	})

	Context("when conditions slice is nil", func() {
		It("should return -1 and nil", func() {
			idx, cond := getPatternConditionByStatus(nil, corev1.ConditionTrue)
			Expect(idx).To(Equal(-1))
			Expect(cond).To(BeNil())
		})
	})

	Context("when conditions slice is empty", func() {
		It("should return -1 and nil", func() {
			idx, cond := getPatternConditionByStatus([]api.PatternCondition{}, corev1.ConditionTrue)
			Expect(idx).To(Equal(-1))
			Expect(cond).To(BeNil())
		})
	})

	Context("when multiple conditions have the same status", func() {
		It("should return the first matching index", func() {
			dupes := []api.PatternCondition{
				{Type: api.GitInSync, Status: corev1.ConditionTrue},
				{Type: api.Synced, Status: corev1.ConditionTrue},
			}
			idx, cond := getPatternConditionByStatus(dupes, corev1.ConditionTrue)
			Expect(idx).To(Equal(0))
			Expect(cond).ToNot(BeNil())
			Expect(cond.Type).To(Equal(api.GitInSync))
		})
	})
})

var _ = Describe("getPatternConditionByType", func() {
	var conditions []api.PatternCondition

	BeforeEach(func() {
		conditions = []api.PatternCondition{
			{
				Type:    api.GitInSync,
				Status:  corev1.ConditionTrue,
				Message: "in sync",
			},
			{
				Type:    api.Degraded,
				Status:  corev1.ConditionFalse,
				Message: "not degraded",
			},
			{
				Type:    api.Progressing,
				Status:  corev1.ConditionTrue,
				Message: "progressing",
			},
		}
	})

	Context("when condition with given type exists", func() {
		It("should return the index and the condition for GitInSync", func() {
			idx, cond := getPatternConditionByType(conditions, api.GitInSync)
			Expect(idx).To(Equal(0))
			Expect(cond).ToNot(BeNil())
			Expect(cond.Message).To(Equal("in sync"))
		})

		It("should return the index and the condition for Degraded", func() {
			idx, cond := getPatternConditionByType(conditions, api.Degraded)
			Expect(idx).To(Equal(1))
			Expect(cond).ToNot(BeNil())
			Expect(cond.Status).To(Equal(corev1.ConditionFalse))
		})

		It("should return the index and the condition for Progressing", func() {
			idx, cond := getPatternConditionByType(conditions, api.Progressing)
			Expect(idx).To(Equal(2))
			Expect(cond).ToNot(BeNil())
			Expect(cond.Message).To(Equal("progressing"))
		})
	})

	Context("when condition with given type does not exist", func() {
		It("should return -1 and nil for Missing type", func() {
			idx, cond := getPatternConditionByType(conditions, api.Missing)
			Expect(idx).To(Equal(-1))
			Expect(cond).To(BeNil())
		})

		It("should return -1 and nil for Suspended type", func() {
			idx, cond := getPatternConditionByType(conditions, api.Suspended)
			Expect(idx).To(Equal(-1))
			Expect(cond).To(BeNil())
		})
	})

	Context("when conditions slice is nil", func() {
		It("should return -1 and nil", func() {
			idx, cond := getPatternConditionByType(nil, api.GitInSync)
			Expect(idx).To(Equal(-1))
			Expect(cond).To(BeNil())
		})
	})

	Context("when conditions slice is empty", func() {
		It("should return -1 and nil", func() {
			idx, cond := getPatternConditionByType([]api.PatternCondition{}, api.GitInSync)
			Expect(idx).To(Equal(-1))
			Expect(cond).To(BeNil())
		})
	})
})

var _ = Describe("getClusterWideArgoNamespace", func() {
	It("should return the ApplicationNamespace constant", func() {
		ns := getClusterWideArgoNamespace()
		Expect(ns).To(Equal(ApplicationNamespace))
		Expect(ns).To(Equal("openshift-gitops"))
	})
})

var _ = Describe("writeConfigMapKeyToFile", func() {
	var (
		kubeClient kubernetes.Interface
		tmpDir     string
	)

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		tmpDir = createTempDir("vp-writecm-test")
	})

	AfterEach(func() {
		cleanupTempDir(tmpDir)
	})

	Context("when the ConfigMap and key exist", func() {
		BeforeEach(func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"my-key": "my-value-content",
				},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should write the value to a file (truncate mode)", func() {
			filePath := filepath.Join(tmpDir, "output.txt")
			err := writeConfigMapKeyToFile(kubeClient, "default", "test-cm", "my-key", filePath, false)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("my-value-content\n"))
		})

		It("should append the value to a file (append mode)", func() {
			filePath := filepath.Join(tmpDir, "output.txt")
			// Write initial content
			err := os.WriteFile(filePath, []byte("existing-content\n"), 0600)
			Expect(err).ToNot(HaveOccurred())

			err = writeConfigMapKeyToFile(kubeClient, "default", "test-cm", "my-key", filePath, true)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("existing-content\nmy-value-content\n"))
		})

		It("should overwrite existing content in truncate mode", func() {
			filePath := filepath.Join(tmpDir, "output.txt")
			err := os.WriteFile(filePath, []byte("old-content\n"), 0600)
			Expect(err).ToNot(HaveOccurred())

			err = writeConfigMapKeyToFile(kubeClient, "default", "test-cm", "my-key", filePath, false)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("my-value-content\n"))
		})
	})

	Context("when the ConfigMap does not exist", func() {
		It("should return an error", func() {
			filePath := filepath.Join(tmpDir, "output.txt")
			err := writeConfigMapKeyToFile(kubeClient, "default", "nonexistent-cm", "my-key", filePath, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error getting ConfigMap"))
		})
	})

	Context("when the key does not exist in the ConfigMap", func() {
		BeforeEach(func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"other-key": "other-value",
				},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error about missing key", func() {
			filePath := filepath.Join(tmpDir, "output.txt")
			err := writeConfigMapKeyToFile(kubeClient, "default", "test-cm", "missing-key", filePath, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key missing-key not found"))
		})
	})
})

var _ = Describe("getConfigMapKey", func() {
	var kubeClient kubernetes.Interface

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
	})

	Context("when ConfigMap and key exist", func() {
		BeforeEach(func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"ca.crt": "certificate-data-here",
				},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the value", func() {
			val, err := getConfigMapKey(kubeClient, "default", "test-cm", "ca.crt")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal("certificate-data-here"))
		})
	})

	Context("when the ConfigMap does not exist", func() {
		It("should return an error", func() {
			_, err := getConfigMapKey(kubeClient, "default", "nonexistent", "key")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error getting ConfigMap"))
		})
	})

	Context("when the key does not exist", func() {
		BeforeEach(func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"other-key": "value",
				},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error about missing key", func() {
			_, err := getConfigMapKey(kubeClient, "default", "test-cm", "missing-key")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key missing-key not found"))
		})
	})
})

var _ = Describe("getHTTPSTransport", func() {
	Context("with nil client", func() {
		It("should return a transport with system cert pool", func() {
			transport := getHTTPSTransport(nil)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig).ToNot(BeNil())
			Expect(transport.TLSClientConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
		})
	})

	Context("with a fake client and no configmaps", func() {
		It("should return a transport falling back to system certs", func() {
			kubeClient := fake.NewSimpleClientset()
			transport := getHTTPSTransport(kubeClient)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig).ToNot(BeNil())
		})
	})

	Context("with a fake client and kube-root-ca.crt configmap", func() {
		It("should use the CA data from the configmap", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-root-ca.crt",
					Namespace: "openshift-config-managed",
				},
				Data: map[string]string{
					"ca.crt": string(testCACert),
				},
			}
			kubeClient := fake.NewSimpleClientset(cm)
			transport := getHTTPSTransport(kubeClient)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})

	Context("with both kube-root-ca and trusted-ca-bundle", func() {
		It("should merge both CA bundles", func() {
			cm1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-root-ca.crt",
					Namespace: "openshift-config-managed",
				},
				Data: map[string]string{
					"ca.crt": string(testCACert),
				},
			}
			cm2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "trusted-ca-bundle",
					Namespace: "openshift-config-managed",
				},
				Data: map[string]string{
					"ca-bundle.crt": string(testCACert),
				},
			}
			kubeClient := fake.NewSimpleClientset(cm1, cm2)
			transport := getHTTPSTransport(kubeClient)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})
})

var _ = Describe("IsCommonSlimmed", func() {
	var tmpDir string

	BeforeEach(func() {
		tmpDir = createTempDir("vp-slimmed-test")
	})

	AfterEach(func() {
		cleanupTempDir(tmpDir)
	})

	Context("when common/operator-install directory exists", func() {
		It("should return false (not slimmed)", func() {
			err := os.MkdirAll(filepath.Join(tmpDir, "common", "operator-install"), 0755)
			Expect(err).ToNot(HaveOccurred())

			Expect(IsCommonSlimmed(tmpDir)).To(BeFalse())
		})
	})

	Context("when common/operator-install directory does not exist", func() {
		It("should return true (slimmed)", func() {
			Expect(IsCommonSlimmed(tmpDir)).To(BeTrue())
		})
	})

	Context("when common directory exists but operator-install does not", func() {
		It("should return true (slimmed)", func() {
			err := os.MkdirAll(filepath.Join(tmpDir, "common"), 0755)
			Expect(err).ToNot(HaveOccurred())

			Expect(IsCommonSlimmed(tmpDir)).To(BeTrue())
		})
	})
})

var _ = Describe("IntOrZero", func() {
	Context("when the key exists and has a valid integer value", func() {
		It("should return the integer value", func() {
			secret := map[string][]byte{
				"appID": []byte("12345"),
			}
			val, err := IntOrZero(secret, "appID")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(int64(12345)))
		})
	})

	Context("when the key does not exist", func() {
		It("should return 0", func() {
			secret := map[string][]byte{}
			val, err := IntOrZero(secret, "appID")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(int64(0)))
		})
	})

	Context("when the key exists but has an invalid value", func() {
		It("should return an error", func() {
			secret := map[string][]byte{
				"appID": []byte("not-a-number"),
			}
			_, err := IntOrZero(secret, "appID")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the key exists with a negative value", func() {
		It("should return the negative integer", func() {
			secret := map[string][]byte{
				"appID": []byte("-42"),
			}
			val, err := IntOrZero(secret, "appID")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(int64(-42)))
		})
	})

	Context("when the key exists with a zero value", func() {
		It("should return 0", func() {
			secret := map[string][]byte{
				"appID": []byte("0"),
			}
			val, err := IntOrZero(secret, "appID")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(int64(0)))
		})
	})

	Context("when the key exists with an empty string value", func() {
		It("should return an error", func() {
			secret := map[string][]byte{
				"appID": []byte(""),
			}
			_, err := IntOrZero(secret, "appID")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("DefaultRandRead", func() {
	It("should fill the buffer with random bytes", func() {
		buf := make([]byte, 32)
		n, err := DefaultRandRead(buf)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(32))
		// Verify not all zeros (extremely unlikely with random data)
		allZeros := true
		for _, b := range buf {
			if b != 0 {
				allZeros = false
				break
			}
		}
		Expect(allZeros).To(BeFalse())
	})

	It("should fill different buffers with different data", func() {
		buf1 := make([]byte, 32)
		buf2 := make([]byte, 32)
		_, err1 := DefaultRandRead(buf1)
		_, err2 := DefaultRandRead(buf2)
		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
		Expect(buf1).ToNot(Equal(buf2))
	})
})

var _ = Describe("logOnce", func() {
	It("should not panic when called multiple times with same message", func() {
		// Reset the logKeys map for a clean test
		logKeys = map[string]bool{}
		Expect(func() {
			logOnce("test message for logOnce")
			logOnce("test message for logOnce")
			logOnce("test message for logOnce")
		}).ToNot(Panic())
	})

	It("should record the message in the logKeys map", func() {
		logKeys = map[string]bool{}
		logOnce("unique log message")
		Expect(logKeys).To(HaveKey("unique log message"))
		Expect(logKeys["unique log message"]).To(BeTrue())
	})

	It("should handle multiple different messages", func() {
		logKeys = map[string]bool{}
		logOnce("message one")
		logOnce("message two")
		Expect(logKeys).To(HaveKey("message one"))
		Expect(logKeys).To(HaveKey("message two"))
		Expect(logKeys).To(HaveLen(2))
	})
})

var _ = Describe("createNamespace", func() {
	var kubeClient kubernetes.Interface

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
	})

	Context("when the namespace does not exist", func() {
		It("should create it", func() {
			err := createNamespace(kubeClient, "new-namespace")
			Expect(err).ToNot(HaveOccurred())

			ns, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), "new-namespace", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ns.Name).To(Equal("new-namespace"))
		})
	})

	Context("when the namespace already exists", func() {
		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "existing-ns"}}
			_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not return an error", func() {
			err := createNamespace(kubeClient, "existing-ns")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when there is an API error", func() {
		It("should return the error", func() {
			fakeClient := fake.NewSimpleClientset()
			fakeClient.PrependReactor("get", "namespaces", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, kubeerrors.NewInternalError(fmt.Errorf("internal error"))
			})
			err := createNamespace(fakeClient, "error-ns")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("createTrustedBundleCM", func() {
	var kubeClient kubernetes.Interface

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
	})

	Context("when the configmap does not exist", func() {
		It("should create it with the correct labels", func() {
			err := createTrustedBundleCM(kubeClient, "test-namespace")
			Expect(err).ToNot(HaveOccurred())

			cm, err := kubeClient.CoreV1().ConfigMaps("test-namespace").Get(context.Background(), "trusted-ca-bundle", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(cm.Labels).To(HaveKeyWithValue("config.openshift.io/inject-trusted-cabundle", "true"))
		})
	})

	Context("when the configmap already exists", func() {
		BeforeEach(func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "trusted-ca-bundle",
					Namespace: "test-namespace",
				},
			}
			_, err := kubeClient.CoreV1().ConfigMaps("test-namespace").Create(context.Background(), cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not return an error", func() {
			err := createTrustedBundleCM(kubeClient, "test-namespace")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when there is a get error other than NotFound", func() {
		It("should return the error", func() {
			fakeClient := fake.NewSimpleClientset()
			fakeClient.PrependReactor("get", "configmaps", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, kubeerrors.NewInternalError(fmt.Errorf("internal error"))
			})
			err := createTrustedBundleCM(fakeClient, "test-namespace")
			Expect(err).To(HaveOccurred())
		})
	})
})
