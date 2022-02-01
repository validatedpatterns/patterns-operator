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
	//	"log"
	"os"

	"github.com/go-git/go-git/v5"
	. "github.com/go-git/go-git/v5/_examples"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// https://github.com/go-git/go-git/blob/master/_examples/commit/main.go

func checkout(url, directory, token, branch, commit string) error {

	if err := cloneRepo(url, directory, token); err != nil {
		return err
	}

	if len(commit) == 0 {
		// Nothing more to do
		return nil
	}

	if err := checkoutRevision(directory, token, branch, commit); err != nil {
		return err
	}

	return nil
}

func checkoutRevision(directory, token, branch, commit string) error {

	Info("Accessing %s", directory)
	repo, err := git.PlainOpen(directory)
	if err != nil {
		return err
	}

	var foptions = &git.FetchOptions{
		Force:           true,
		InsecureSkipTLS: true,
		Tags:            git.AllTags,
	}

	if len(token) > 0 {
		// The intended use of a GitHub personal access token is in replace of your password
		// because access tokens can easily be revoked.
		// https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
		foptions.Auth = &http.BasicAuth{
			Username: "abc123", // yes, this can be anything except an empty string
			Password: token,
		}
	}

	if err := repo.Fetch(foptions); err != nil && err != git.NoErrAlreadyUpToDate {
		Info("Error fetching")
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		Info("Error obtaining worktree")
		return err
	}

	coptions := git.CheckoutOptions{
		Force: true,
	}
	if len(commit) > 0 {
		Info("git checkout %s (hash)", commit)
		coptions.Hash = plumbing.NewHash(commit)
	} else if len(branch) > 0 {
		Info("git checkout %s (branch)", branch)
		coptions.Branch = plumbing.ReferenceName(branch)
	} else {
		Info("git checkout main (default)")
		coptions.Branch = plumbing.ReferenceName("main")
	}
	if err := w.Checkout(&coptions); err != nil && err != git.NoErrAlreadyUpToDate {
		Info("Error during checkout")
		return err
	}

	// ... retrieving the commit being pointed by HEAD, it shows that the
	// repository is pointing to the giving commit in detached mode
	Info("git show-ref --head HEAD")
	ref, err := repo.Head()
	if err != nil {
		Info("Error obtaining HEAD")
		return err
	}

	Info("%s", ref.Hash())
	return err

}

func cloneRepo(url string, directory string, token string) error {

	Info("git clone %s into %s", url, directory)

	// Clone the given repository to the given directory
	var options = &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	}

	if len(token) > 0 {
		// The intended use of a GitHub personal access token is in replace of your password
		// because access tokens can easily be revoked.
		// https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
		options.Auth = &http.BasicAuth{
			Username: "abc123", // yes, this can be anything except an empty string
			Password: token,
		}
	}

	repo, err := git.PlainClone(directory, false, options)
	if err != nil {
		return err
	}

	// ... retrieving the commit being pointed by HEAD
	Info("git show-ref --head HEAD")
	ref, err := repo.Head()
	if err != nil {
		return err
	}
	Info("%s", ref.Hash())
	return nil
}

func repoHash(directory string) (error, string) {
	repo, err := git.PlainOpen(directory)
	if err != nil {
		return err, ""
	}

	// ... checking out to commit
	ref, err := repo.Head()
	if err != nil {
		return err, ""
	}

	return nil, ref.Hash().String()
}
