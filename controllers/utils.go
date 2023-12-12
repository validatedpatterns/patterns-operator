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
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-errors/errors"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"

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

// ContainsString checks if the string array contains the given string.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// RemoveString removes the given string from the string array if exists.
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

func ParametersToMap(parameters []api.PatternParameter) map[string]any {
	output := map[string]any{}
	for _, p := range parameters {
		keys := strings.Split(p.Name, ".")
		max := len(keys) - 1
		current := output

		for i, key := range keys {
			fmt.Printf("Step %d %s\n", i, key)
			if i == max {
				current[key] = p.Value
			} else {
				if val, ok := current[key]; ok {
					fmt.Printf("Using %q\n", key)
					current = val.(map[string]any)
				} else if i < len(key) {
					fmt.Printf("Adding %q\n", key)
					current[key] = map[string]any{}
					current = current[key].(map[string]any)
				}
			}
		}
	}

	return output
}

// getPatternConditionByStatus returns a copy of the pattern condition defined by the status and the index in the slice if it exists, otherwise -1 and nil
func getPatternConditionByStatus(conditions []api.PatternCondition, conditionStatus v1.ConditionStatus) (int, *api.PatternCondition) {
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
