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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	It("should return an error for an invalid URL", func() {
		invalidURL := "invalid-url"
		_, err := extractRepositoryName(invalidURL)
		Expect(err).To(HaveOccurred())
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
