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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/go-errors/errors"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
		fakeClient client.Client
		namespace  string
	)

	BeforeEach(func() {
		namespace = "default"
	})

	Context("when the ConfigMap does not exist", func() {
		It("should create the ConfigMap", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects().Build()

			err := createTrustedBundleCM(fakeClient, namespace)
			Expect(err).ToNot(HaveOccurred())

			// Verify that the ConfigMap was created
			cm := corev1.ConfigMap{}
			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "trusted-ca-bundle", Namespace: namespace}, &cm)
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(cm).Build()
		})

		It("should not return an error", func() {
			err := createTrustedBundleCM(fakeClient, namespace)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when an error occurs while checking for the ConfigMap", func() {
		It("should return the error", func() {
			// Inject an error into the fake client
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return kubeerrors.NewInternalError(fmt.Errorf("some error"))
				}}).WithScheme(testEnv.Scheme).Build()

			err := createTrustedBundleCM(fakeClient, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some error"))
		})
	})

	Context("when an error occurs while creating the ConfigMap", func() {
		It("should return the error", func() {
			// Inject an error into the fake client
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					return kubeerrors.NewInternalError(fmt.Errorf("some create error"))
				}},
			).WithScheme(testEnv.Scheme).Build()

			err := createTrustedBundleCM(fakeClient, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some create error"))
		})
	})
})

var _ = Describe("WriteConfigMapKeyToFile", func() {
	var (
		fakeClient    client.Client
		namespace     string
		configMap     *corev1.ConfigMap
		configMapName string
		key           string
		filePath      string
		appendToFile  bool
	)

	BeforeEach(func() {
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()
		})

		It("should write the value to the file", func() {
			err := writeConfigMapKeyToFile(fakeClient, namespace, configMapName, key, filePath, appendToFile)
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

			err = writeConfigMapKeyToFile(fakeClient, namespace, configMapName, key, filePath, appendToFile)
			Expect(err).ToNot(HaveOccurred())

			// Verify the content of the file
			content, err := os.ReadFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal(initialContent + "test-value\n"))
		})
	})

	Context("when the ConfigMap does not exist", func() {
		It("should return an error", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects().Build()
			err := writeConfigMapKeyToFile(fakeClient, namespace, configMapName, key, filePath, appendToFile)
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()
		})

		It("should return an error", func() {
			err := writeConfigMapKeyToFile(fakeClient, namespace, configMapName, key, filePath, appendToFile)
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()
		})

		It("should return an error", func() {
			invalidFilePath := "/invalid-path/testfile"

			err := writeConfigMapKeyToFile(fakeClient, namespace, configMapName, key, invalidFilePath, appendToFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error opening file"))
		})
	})
})

var _ = Describe("GetConfigMapKey", func() {
	var (
		fakeClient    client.Client
		namespace     string
		configMap     *corev1.ConfigMap
		configMapName string
		key           string
		value         string
	)

	BeforeEach(func() {
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()
		})

		It("should return the value for the specified key", func() {
			result, err := getConfigMapKey(fakeClient, namespace, configMapName, key)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(value))
		})
	})

	Context("when the ConfigMap does not exist", func() {
		It("should return an error", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects().Build()
			result, err := getConfigMapKey(fakeClient, namespace, configMapName, key)
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()
		})

		It("should return an error", func() {
			result, err := getConfigMapKey(fakeClient, namespace, configMapName, key)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("key %s not found in ConfigMap %s", key, configMapName)))
		})
	})

	Context("when an error occurs while getting the ConfigMap", func() {
		It("should return an error", func() {
			// Inject an error into the fake client
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return kubeerrors.NewInternalError(fmt.Errorf("some error"))
				}}).WithScheme(testEnv.Scheme).Build()

			result, err := getConfigMapKey(fakeClient, namespace, configMapName, key)
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
		fakeClient      client.Client
		namespace       string
		kubeRootCA      string
		trustedCABundle string
		configMapName1  string
		configMapName2  string
		key1            string
		key2            string
	)

	BeforeEach(func() {
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap1, configMap2).Build()
		})

		It("should create a transport with the combined CA certificates", func() {
			transport := getHTTPSTransport(fakeClient)
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
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()
		})

		It("should create a transport with the available CA certificate", func() {
			transport := getHTTPSTransport(fakeClient)
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
			transport := getHTTPSTransport(fakeClient)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})

	Context("when an error occurs while getting a ConfigMap", func() {
		It("should print an error message and fallback to system CA certificates", func() {
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return fmt.Errorf("some error")
				}}).WithScheme(testEnv.Scheme).Build()

			transport := getHTTPSTransport(fakeClient)
			Expect(transport).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})
})

var _ = Describe("createNamespace", func() {
	var (
		fakeClient client.Client
		namespace  string
	)

	BeforeEach(func() {
		namespace = "test-ns"
	})

	It("should not return an error if the namespace already exists", func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
			WithRuntimeObjects(ns).Build()
		err := createNamespace(fakeClient, namespace)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should create the namespace if it does not exist", func() {
		err := createNamespace(fakeClient, namespace)
		Expect(err).ToNot(HaveOccurred())
		found := &corev1.Namespace{}
		err = fakeClient.Get(context.Background(), types.NamespacedName{Name: namespace}, found)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return an error if there is an error checking if the namespace exists", func() {

		fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
			interceptor.Funcs{Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return fmt.Errorf("internal error")
			}}).WithScheme(testEnv.Scheme).Build()

		err := createNamespace(fakeClient, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("internal error"))
	})

	It("should return an error if there is an error creating the namespace", func() {
		fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
			interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return kubeerrors.NewNotFound(corev1.Resource("namespace"), namespace)
				},
				Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					return kubeerrors.NewInternalError(fmt.Errorf("internal error"))
				},
			}).WithScheme(testEnv.Scheme).Build()

		err := createNamespace(fakeClient, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("internal error"))
	})
})
