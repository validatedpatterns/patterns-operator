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

func checkout(url string, directory string, token string, commit string) error {

	if len(commit) > 0 {
		Info("git clone %s@%s into %s", url, commit, directory)
	} else {
		Info("git clone %s into %s", url, directory)
	}

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

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	if len(commit) == 0 {
		// Nothing more to do
		return nil
	}

	// ... checking out to commit
	Info("git checkout %s", commit)
	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	})
	if err != nil {
		return err
	}

	// ... retrieving the commit being pointed by HEAD, it shows that the
	// repository is pointing to the giving commit in detached mode
	Info("git show-ref --head HEAD")
	ref, err = repo.Head()
	if err != nil {
		return err
	}

	Info("%s", ref.Hash())
	return err
}
