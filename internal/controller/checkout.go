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
	nethttp "net/http"
	"os"
	"regexp"
	"strings"

	"path/filepath"

	stdssh "golang.org/x/crypto/ssh"
	"k8s.io/client-go/kubernetes"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	argogit "github.com/argoproj/argo-cd/v3/util/git"
)

type GitAuthenticationBackend uint

const (
	GitAuthNone     GitAuthenticationBackend = 0
	GitAuthPassword GitAuthenticationBackend = 1
	GitAuthSsh      GitAuthenticationBackend = 2
)

const GitCustomCAFile = "/tmp/vp-git-cas.pem"
const GitHEAD = "HEAD"
const VPTmpFolder = "vp"

// GitOperations interface defines the methods used from the go-git package.
type GitOperations interface {
	OpenRepository(directory string) (*git.Repository, error)
	CloneRepository(directory string, isBare bool, options *git.CloneOptions) (*git.Repository, error)
}

// GitOperationsImpl implements the GitOperations interface using the actual go-git package.
type GitOperationsImpl struct{}

// OpenRepository opens a git repository.
func (g *GitOperationsImpl) OpenRepository(directory string) (*git.Repository, error) {
	return git.PlainOpen(directory)
}

func (g *GitOperationsImpl) CloneRepository(directory string, isBare bool, options *git.CloneOptions) (*git.Repository, error) {
	if err := os.MkdirAll(directory, os.ModePerm); err != nil {
		return nil, err
	}

	repo, err := git.PlainClone(directory, isBare, options)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// https://github.com/go-git/go-git/blob/master/_examples/commit/main.go
func checkout(fullClient kubernetes.Interface, gitOps GitOperations, url, directory, commit string, secret map[string][]byte) error {
	if err := cloneRepo(fullClient, gitOps, url, directory, secret); err != nil {
		return err
	}

	if commit == "" {
		// Nothing more to do
		return nil
	}

	if err := checkoutRevision(fullClient, gitOps, url, directory, commit, secret); err != nil {
		return err
	}

	return nil
}

func getHashFromReference(repo *git.Repository, name plumbing.ReferenceName) (plumbing.Hash, error) {
	b, err := repo.Reference(name, true)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	if !b.Name().IsTag() {
		return b.Hash(), nil
	}

	o, err := repo.Object(plumbing.AnyObject, b.Hash())
	if err != nil {
		return plumbing.ZeroHash, err
	}

	switch o := o.(type) {
	case *object.Tag:
		if o.TargetType != plumbing.CommitObject {
			return plumbing.ZeroHash, fmt.Errorf("unsupported tag object target %q", o.TargetType)
		}

		return o.Target, nil
	case *object.Commit:
		return o.Hash, nil
	}

	return plumbing.ZeroHash, fmt.Errorf("unsupported tag target %q", o.Type())
}

func getCommitFromTarget(repo *git.Repository, name string) (plumbing.Hash, error) {
	if name == "" {
		return getHashFromReference(repo, plumbing.NewBranchReferenceName("main"))
	}

	// Try as commit hash
	h := plumbing.NewHash(name)
	_, err := repo.Object(plumbing.AnyObject, h)
	if err == nil {
		return h, nil
	}

	// Explicitly handle the "HEAD" reference
	if name == GitHEAD {
		headRef, err := repo.Head()
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("failed to get HEAD reference: %w", err)
		}
		return headRef.Hash(), nil
	}

	// Try various reference types...
	if h, err := getHashFromReference(repo, plumbing.NewBranchReferenceName(name)); err == nil {
		return h, nil
	}

	if h, err := getHashFromReference(repo, plumbing.NewTagReferenceName(name)); err == nil {
		return h, nil
	}

	if h, err := getHashFromReference(repo, plumbing.NewRemoteHEADReferenceName(name)); err == nil {
		return h, nil
	}
	if h, err := getHashFromReference(repo, plumbing.NewRemoteReferenceName("origin", name)); err == nil {
		return h, nil
	}

	return plumbing.ZeroHash, fmt.Errorf("unknown target %q", name)
}

func checkoutRevision(fullClient kubernetes.Interface, gitOps GitOperations, url, directory, commit string, secret map[string][]byte) error {
	customClient := &nethttp.Client{
		Transport: getHTTPSTransport(fullClient),
	}
	// Override http(s) default protocol to use our custom client
	client.InstallProtocol("https", http.NewClient(customClient))
	repo, err := gitOps.OpenRepository(directory)
	if err != nil {
		return err
	}
	if repo == nil { // we mocked the above OpenRepository
		return nil
	}
	foptions, err := getFetchOptions(url, secret)
	if err != nil {
		return err
	}

	if err = repo.Fetch(foptions); err != nil && err != git.NoErrAlreadyUpToDate {
		fmt.Printf("Error fetching: %v\n", err)
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		fmt.Println("Error obtaining worktree")
		return err
	}

	h, err := getCommitFromTarget(repo, commit)
	coptions := git.CheckoutOptions{
		Force: true,
		Hash:  h,
	}

	if err != nil {
		return err
	}

	fmt.Printf("git checkout %s (%s)\n", h, commit)

	if err = w.Checkout(&coptions); err != nil && err != git.NoErrAlreadyUpToDate {
		fmt.Printf("Error during checkout")
		return err
	}
	// ... retrieving the commit being pointed by HEAD, it shows that the
	// repository is pointing to the giving commit in detached mode
	fmt.Println("git show-ref --head HEAD")
	ref, err := repo.Head()
	if err != nil {
		fmt.Println("Error obtaining HEAD")
		return err
	}

	fmt.Printf("%s\n", ref.Hash())
	return err
}

func cloneRepo(fullClient kubernetes.Interface, gitOps GitOperations, url, directory string, secret map[string][]byte) error {
	customClient := &nethttp.Client{
		Transport: getHTTPSTransport(fullClient),
	}
	// Override http(s) default protocol to use our custom client
	client.InstallProtocol("https", http.NewClient(customClient))

	gitDir := filepath.Join(directory, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		fmt.Printf("%s already exists\n", gitDir)
		return nil
	}
	fmt.Printf("git clone %s into %s\n", url, directory)

	options, err := getCloneOptions(url, secret)
	if err != nil {
		return err
	}

	repo, err := gitOps.CloneRepository(directory, false, options)
	if err != nil {
		return err
	}
	if repo == nil { // We mocked the above CloneRepository()
		return nil
	}

	// ... retrieving the commit being pointed by HEAD
	fmt.Println("git show-ref --head HEAD")
	if ref, err := repo.Head(); err != nil {
		return err
	} else {
		fmt.Printf("%s\n", ref.Hash())
	}

	return nil
}

func getFetchOptions(url string, secret map[string][]byte) (*git.FetchOptions, error) {
	var foptions = &git.FetchOptions{
		RemoteName:      "origin",
		Force:           true,
		InsecureSkipTLS: true,
		Tags:            git.AllTags,
	}
	switch authType := detectGitAuthType(secret); authType {
	case GitAuthPassword:
		foptions.Auth = getHttpAuth(secret)
	case GitAuthSsh:
		publicKey, err := getSshPublicKey(url, secret)
		if err != nil {
			return nil, err
		}
		foptions.Auth = publicKey
	}

	return foptions, nil
}

func getCloneOptions(url string, secret map[string][]byte) (*git.CloneOptions, error) {
	// Clone the given repository to the given directory
	var options = &git.CloneOptions{
		URL:          url,
		RemoteName:   "origin",
		Progress:     os.Stdout,
		Depth:        0,
		SingleBranch: false,
		Tags:         git.AllTags,
	}

	switch authType := detectGitAuthType(secret); authType {
	case GitAuthPassword:
		options.Auth = getHttpAuth(secret)
	case GitAuthSsh:
		publicKey, err := getSshPublicKey(url, secret)
		if err != nil {
			return nil, err
		}
		options.Auth = publicKey
	}

	return options, nil
}

func getHttpAuth(secret map[string][]byte) *http.BasicAuth {
	// The intended use of a GitHub personal access token is in replace of your password
	// because access tokens can easily be revoked.
	// https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
	auth := &http.BasicAuth{
		Username: string(getField(secret, "username")),
		Password: string(getField(secret, "password")),
	}

	return auth
}

func getSshPublicKey(url string, secret map[string][]byte) (*ssh.PublicKeys, error) {
	sshKey := getField(secret, "sshPrivateKey")
	if sshKey == nil {
		return nil, fmt.Errorf("could not get sshPrivateKey")
	}

	user := getUserFromURL(url)
	publicKey, keyError := ssh.NewPublicKeys(user, sshKey, "")
	if keyError != nil {
		return nil, fmt.Errorf("could not get publicKey: %s", keyError)
	}
	// FIXME(bandini): in the future we might want to support passing some known hosts
	publicKey.HostKeyCallback = stdssh.InsecureIgnoreHostKey() //nolint:gosec
	return publicKey, nil
}

// This returns the user prefix in git urls like:
// git@github.com:/foo/bar or "" when not found
func getUserFromURL(url string) string {
	tokens := strings.Split(url, "@")
	if len(tokens) > 1 {
		return tokens[0]
	}
	return ""
}

func repoHash(directory string) (string, error) {
	repo, err := git.PlainOpen(directory)
	if err != nil {
		return "", err
	}

	// ... checking out to commit
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

// Developed after https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#repositories
// if a secret has
// returns "" if a secret could not be parse, "ssh" if it is an ssh auth, and "password" if a username + pass auth
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

func getField(secret map[string][]byte, field string) []byte {
	value, hasField := secret[field]
	if hasField {
		return value
	}
	return nil
}

func getGitRemoteURL(repoPath, remoteName string) (string, error) {
	// Open the given repository
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}

	// Retrieve the remote configuration
	remote, err := r.Remote(remoteName)
	if err != nil {
		return "", err
	}

	// Get the first URL from the remote config (remotes can have multiple URLs)
	if len(remote.Config().URLs) == 0 {
		return "", fmt.Errorf("remote %s has no URLs", remoteName)
	}

	return remote.Config().URLs[0], nil
}

func getLocalGitPath(repoURL string) string {
	r := regexp.MustCompile("([/:])")
	normalizedGitURL := argogit.NormalizeGitURL(repoURL)
	if normalizedGitURL == "" {
		normalizedGitURL = repoURL
	}
	root := filepath.Join(os.TempDir(), VPTmpFolder, r.ReplaceAllString(normalizedGitURL, "_"))
	if root == os.TempDir() {
		return filepath.Join(os.TempDir(), VPTmpFolder, "vp-git-repo-fallback")
	}
	return root
}
