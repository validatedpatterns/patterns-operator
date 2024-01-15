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
	"os"
	"path/filepath"
	"regexp"

	"github.com/argoproj/argo-cd/v2/util/git"
)

type GitAuthenticationBackend uint

const (
	GitAuthNone     GitAuthenticationBackend = 0
	GitAuthPassword GitAuthenticationBackend = 1
	GitAuthSsh      GitAuthenticationBackend = 2
)

const VPTmpFolder = "vp"

type GitOperations interface {
	CloneAndCheckout(repoURL, revision, localFolder string, gitAuth map[string][]byte) error
}

type GitOperationsImpl struct{}

func getField(secret map[string][]byte, field string) []byte {
	value, hasField := secret[field]
	if hasField {
		return value
	}
	return nil
}

func getHttpAuth(secret map[string][]byte) git.GenericHTTPSCreds {
	username := string(getField(secret, "username"))
	password := string(getField(secret, "password"))
	creds := git.NewHTTPSCreds(username, password, "", "", false, "", &git.NoopCredsStore{}, true)
	return creds
}

func getSshAuth(secret map[string][]byte) (git.SSHCreds, error) {
	sshKey := getField(secret, "sshPrivateKey")
	if sshKey == nil {
		return git.SSHCreds{}, fmt.Errorf("Could not get sshPrivateKey")
	}

	// FIXME(bandini): in the future we might want to support passing some known hosts
	creds := git.NewSSHCreds(string(sshKey), "", true, &git.NoopCredsStore{})
	return creds, nil
}

// Developed after https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#repositories
func detectGitAuthType(secret map[string][]byte) GitAuthenticationBackend {
	if _, ok := secret["sshPrivateKey"]; ok {
		return GitAuthSsh
	}
	_, hasUser := secret["username"]
	_, hasPassword := secret["password"]
	if hasUser && hasPassword {
		return GitAuthPassword
	}
	return GitAuthNone
}

func getAuth(gitAuth map[string][]byte) (git.Creds, error) {
	var creds git.Creds
	authType := detectGitAuthType(gitAuth)
	if authType == GitAuthPassword {
		creds = getHttpAuth(gitAuth)
		return creds, nil
	} else if authType == GitAuthSsh {
		creds, err := getSshAuth(gitAuth)
		return creds, err
	} else if authType == GitAuthNone {
		creds = git.NopCreds{}
		return creds, nil
	}
	return git.NopCreds{}, fmt.Errorf("Could not detect the authentication type")
}

func getLocalGitPath(repoURL string) (string, error) {
	r := regexp.MustCompile("([/:])")
	normalizedGitURL := git.NormalizeGitURL(repoURL)
	if normalizedGitURL == "" {
		return "", fmt.Errorf("repository %q cannot be initialized: %w", repoURL, git.ErrInvalidRepoURL)
	}
	root := filepath.Join(os.TempDir(), VPTmpFolder, r.ReplaceAllString(normalizedGitURL, "_"))
	if root == os.TempDir() {
		return "", fmt.Errorf("repository %q cannot be initialized, because its root would be system temp at %s", repoURL, root)
	}
	return root, nil
}

func (g *GitOperationsImpl) CloneAndCheckout(repoURL, revision, localFolder string, gitAuth map[string][]byte) error {
	var client git.Client
	var err error

	creds, err := getAuth(gitAuth)
	if err != nil {
		return fmt.Errorf("Could not get Authentication info: %w", err)
	}
	client, err = git.NewClientExt(repoURL, localFolder, creds, false, false, "")
	if err != nil {
		return fmt.Errorf("failed to create Git client: %w", err)
	}
	err = client.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize the repository: %w", err)
	}

	err = client.Fetch(revision)
	if err != nil {
		return fmt.Errorf("failed to fetch the repository: %w", err)
	}

	err = client.Checkout(revision, true)
	if err != nil {
		return fmt.Errorf("failed to checkout the revision: %w", err)
	}
	return nil
}

func GetGit(gitOps GitOperations, repoURL, revision, localFolder string, gitAuth map[string][]byte) error {
	return gitOps.CloneAndCheckout(repoURL, revision, localFolder, gitAuth)
}
