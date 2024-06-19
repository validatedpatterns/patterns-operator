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
	"fmt"
	"net/http"

	"code.gitea.io/sdk/gitea"
	"k8s.io/client-go/kubernetes"
)

type GiteaOperations interface {
	MigrateGiteaRepo(fullClient kubernetes.Interface, username, password, upstreamURL, giteaServerRoute string) (success bool, repositoryURL string, err error)
}

type GiteaOperationsImpl struct{}

// Function that creates a mirror repo in Gitea
func (g *GiteaOperationsImpl) MigrateGiteaRepo(
	fullClient kubernetes.Interface, username, password, upstreamURL, giteaServerRoute string) (success bool, repositoryURL string, err error) {
	option := gitea.SetBasicAuth(username, password)
	httpClient := &http.Client{
		Transport: getHTTPSTransport(fullClient),
	}

	giteaClient, err := gitea.NewClient(giteaServerRoute, option, gitea.SetHTTPClient(httpClient))
	if err != nil {
		return false, "", err
	}

	// Let's extract the repo name
	repoName, _ := extractRepositoryName(upstreamURL)

	// Check to see if the repo already exists
	repository, response, _ := giteaClient.GetRepo(GiteaAdminUser, repoName)

	// Repo has been already migrated
	if response.Response.StatusCode == http.StatusOK {
		return true, repository.HTMLURL, nil
	}

	// Default description will include repo name and that it was created by
	// the Validated Patterns operator.
	descriptionFormat := "The [%s] repository was migrated by the Validated Patterns Operator."

	description := fmt.Sprintf(descriptionFormat, repoName)

	repository, _, err = giteaClient.MigrateRepo(gitea.MigrateRepoOption{
		CloneAddr: upstreamURL,
		RepoOwner: username,
		RepoName:  repoName,
		Mirror:    true, // We want gitea to periodically check for changes so create a mirror
		// Private:     true, // TODO: uncomment this if you want the repo to be private on gitea
		Description: description,
	})

	if err != nil {
		return false, "", err
	}

	return true, repository.HTMLURL, nil
}
