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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	nethttp "net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"crypto/rand"
	"crypto/tls"
	"crypto/x509"

	"github.com/Masterminds/semver/v3"
	"github.com/go-errors/errors"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	configv1 "github.com/openshift/api/config/v1"
)

var (
	logKeys = map[string]bool{}
)

func logOnce(message string) {
	if _, ok := logKeys[message]; ok {
		return
	}
	logKeys[message] = true
	log.Println(message)
}

// getPatternConditionByStatus returns a copy of the pattern condition defined by the status and the index in the slice if it exists, otherwise -1 and nil
func getPatternConditionByStatus(conditions []api.PatternCondition, conditionStatus corev1.ConditionStatus) (int, *api.PatternCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Status == conditionStatus {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

// getPatternConditionByType returns a copy of the pattern condition defined by the type and the index in the slice if it exists, otherwise -1 and nil
func getPatternConditionByType(conditions []api.PatternCondition, conditionType api.PatternConditionType) (int, *api.PatternCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

// status:
//  history:
//   - completionTime: null
//     image: quay.io/openshift-release-dev/ocp-release@sha256:af19e94813478382e36ae1fa2ae7bbbff1f903dded6180f4eb0624afe6fc6cd4
//     startedTime: "2023-07-18T07:48:54Z"
//     state: Partial
//     verified: true
//     version: 4.13.5
//   - completionTime: "2023-07-18T07:08:50Z"
//     image: quay.io/openshift-release-dev/ocp-release@sha256:e3fb8ace9881ae5428ae7f0ac93a51e3daa71fa215b5299cd3209e134cadfc9c
//     startedTime: "2023-07-18T06:48:44Z"
//     state: Completed
//     verified: false
//     version: 4.13.4
//   observedGeneration: 4
//     version: 4.10.32

// This function returns the current version of the cluster. Ideally
// We return the first version with Completed status
// https://pkg.go.dev/github.com/openshift/api/config/v1#ClusterVersionStatus specifies that the ordering is preserved
// We do have a fallback in case the history does either not exist or it simply has never completed an update:
// in such cases we just fallback to the status.desired.version
func getCurrentClusterVersion(clusterversion *configv1.ClusterVersion) (*semver.Version, error) {
	// First, check the history for completed versions
	for _, v := range clusterversion.Status.History {
		if v.State == "Completed" {
			return parseAndReturnVersion(v.Version)
		}
	}

	// If no completed versions are found, use the desired version
	return parseAndReturnVersion(clusterversion.Status.Desired.Version)
}

func parseAndReturnVersion(versionStr string) (*semver.Version, error) {
	s, err := semver.NewVersion(versionStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version %s: %w", versionStr, err)
	}
	return s, nil
}

// Extract the last part of a git repo url
func extractRepositoryName(gitURL string) (string, error) {
	if strings.HasPrefix(gitURL, "git@") {
		parts := strings.Split(gitURL, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid ssh git URL")
		}
		s := strings.TrimSuffix(parts[1], "/") // remove trailing slash for repos like git@github.com:mbaldessari/common.git/
		return path.Base(strings.TrimSuffix(s, ".git")), nil
	}
	// We use ParseRequestURI because the URL is assument to be absolute
	// and we want to error out if that is not the case
	parsedURL, err := url.ParseRequestURI(gitURL)
	if err != nil {
		return "", err
	}

	// Extract the last part of the path, which is usually the repository name
	repoName := path.Base(parsedURL.Path)

	// Remove the ".git" extension if present
	if len(repoName) > 4 && repoName[len(repoName)-4:] == ".git" {
		repoName = repoName[:len(repoName)-4]
	}

	return repoName, nil
}

func extractGitFQDNHostname(gitURL string) (string, error) {
	if strings.HasPrefix(gitURL, "git@") {
		parts := strings.Split(gitURL, "@")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid SSH git URL")
		}
		domainParts := strings.Split(parts[1], ":")
		return domainParts[0], nil
	}

	// Parse the URL for HTTP/HTTPS
	parsedURL, err := url.ParseRequestURI(gitURL)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %w", err)
	}
	if parsedURL.Hostname() == "" {
		return "", fmt.Errorf("error parsing URL (empty hostname): %s", gitURL)
	}
	return parsedURL.Hostname(), nil
}

func validGitRepoURL(repoURL string) error {
	switch {
	case strings.HasPrefix(repoURL, "git@"):
		return nil
	case strings.HasPrefix(repoURL, "https://"),
		strings.HasPrefix(repoURL, "http://"):
		return nil
	default:
		return errors.New(fmt.Errorf("repository URL must be either http/https or start with git@ when using ssh authentication: %s", repoURL))
	}
}

// compareMaps compares two map[string][]byte and returns true if they are equal.
func compareMaps(m1, m2 map[string][]byte) bool {
	if len(m1) != len(m2) {
		return false
	}

	for key, val1 := range m1 {
		val2, ok := m2[key]
		if !ok || !bytes.Equal(val1, val2) {
			return false
		}
	}

	return true
}
func newSecret(name, namespace string, secret map[string][]byte, labels map[string]string) *corev1.Secret {
	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: secret,
	}
	return k8sSecret
}

func createTrustedBundleCM(fullClient kubernetes.Interface) error {
	ns := getClusterWideArgoNamespace()
	name := "trusted-ca-bundle"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"config.openshift.io/inject-trusted-cabundle": "true",
			},
		},
	}
	_, err := fullClient.CoreV1().ConfigMaps(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			_, err = fullClient.CoreV1().ConfigMaps(ns).Create(context.TODO(), cm, metav1.CreateOptions{})
			return err
		}
		return err
	}
	return nil
}

func getClusterWideArgoNamespace() string {
	// Once we add support for running the cluster-wide argo instance
	// we will need to amend the logic here
	return ApplicationNamespace
}

// writeConfigMapKeyToFile writes the value of a specified key from a ConfigMap to a file.
// `configMapName` is the name of the ConfigMap.
// `namespace` is the namespace where the ConfigMap resides.
// `key` is the key within the ConfigMap whose value will be written to the file.
// `filePath` is the path to the file where the value will be written.
// `append` will append the data to the file
func writeConfigMapKeyToFile(fullClient kubernetes.Interface, namespace, configMapName, key, filePath string, appendToFile bool) error {
	// Get the ConfigMap
	configMap, err := fullClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting ConfigMap %s in namespace %s: %w", configMapName, namespace, err)
	}
	// Get the value for the specified key
	value, ok := configMap.Data[key]
	if !ok {
		return fmt.Errorf("key %s not found in ConfigMap %s", key, configMapName)
	}
	// Determine the file mode: append or truncate
	var fileMode int
	if appendToFile {
		fileMode = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	} else {
		fileMode = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	}
	// Open the file with the determined mode
	file, err := os.OpenFile(filePath, fileMode, 0644) //nolint:mnd
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", filePath, err)
	}
	defer file.Close()
	// Write (or append) the value to the file
	if _, err = file.WriteString(value + "\n"); err != nil {
		return fmt.Errorf("error writing to file %s: %w", filePath, err)
	}

	return nil
}

func hasExperimentalCapability(capabilities, name string) bool {
	s := strings.Split(capabilities, ",")
	if len(s) == 0 {
		return false
	}
	for _, element := range s {
		if strings.TrimSpace(element) == name {
			return true
		}
	}
	return false
}

// getConfigMapKeyToFile gets the value of a specified key from a ConfigMap.
// `configMapName` is the name of the ConfigMap.
// `namespace` is the namespace where the ConfigMap resides.
// `key` is the key within the ConfigMap whose value will be written to the file.
func getConfigMapKey(fullClient kubernetes.Interface, namespace, configMapName, key string) (string, error) {
	// Get the ConfigMap
	configMap, err := fullClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error getting ConfigMap %s in namespace %s: %w", configMapName, namespace, err)
	}
	// Get the value for the specified key
	value, ok := configMap.Data[key]
	if !ok {
		return "", fmt.Errorf("key %s not found in ConfigMap %s", key, configMapName)
	}

	return value, nil
}

func getHTTPSTransport(fullClient kubernetes.Interface) *nethttp.Transport {
	// Here we dump all the CAs in kube-root-ca.crt and in openshift-config-managed/trusted-ca-bundle to a file
	// and then we call git config --global http.sslCAInfo /path/to/your/cacert.pem
	// This makes us trust our self-signed CAs or any custom CAs a customer might have. We try and ignore any errors here
	var err error
	var kuberoot string = ""
	var trustedcabundle string = ""

	if fullClient != nil {
		kuberoot, err = getConfigMapKey(fullClient, "openshift-config-managed", "kube-root-ca.crt", "ca.crt")
		if err != nil {
			fmt.Printf("Error while getting kube-root-ca.crt configmap: %v", err)
		}
		trustedcabundle, err = getConfigMapKey(fullClient, "openshift-config-managed", "trusted-ca-bundle", "ca-bundle.crt")
		if err != nil {
			fmt.Printf("Error while getting trusted-ca-bundle configmap: %v", err)
		}
	}
	myTransport := &nethttp.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		Proxy: nethttp.ProxyFromEnvironment,
	}
	var cacerts bytes.Buffer
	if kuberoot != "" {
		cacerts.WriteString(kuberoot)
	}
	if trustedcabundle != "" {
		cacerts.WriteString("\n")
		cacerts.WriteString(trustedcabundle)
		cacerts.WriteString("\n")
	}
	// We run either in a test env or we could not fetch any certificates at all
	// Fallback to system certs
	var caCertPool *x509.CertPool
	var certErr error
	if kuberoot == "" && trustedcabundle == "" {
		caCertPool, certErr = x509.SystemCertPool()
	} else {
		caCertPool = x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(cacerts.Bytes())
	}
	if certErr != nil {
		return myTransport
	}
	myTransport.TLSClientConfig.RootCAs = caCertPool
	return myTransport
}

// GenerateRandomPassword generates a random password of specified length
func GenerateRandomPassword(length int, randRead func([]byte) (int, error)) (string, error) {
	rndbytes := make([]byte, length)
	_, err := randRead(rndbytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(rndbytes), nil
}

func DefaultRandRead(b []byte) (int, error) {
	return rand.Read(b)
}
