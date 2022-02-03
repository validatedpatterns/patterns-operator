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
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	//	. "github.com/go-git/go-git/v5/_examples"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

	if err := checkoutRevision(directory, token, commit); err != nil {
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
	if len(name) == 0 {
		return getHashFromReference(repo, plumbing.NewBranchReferenceName("main"))
	}

	// Try as commit hash
	h := plumbing.NewHash(name)
	_, err := repo.Object(plumbing.AnyObject, h)
	if err == nil {
		return h, err
	}

	// Try various reference types...

	if h, err := getHashFromReference(repo, plumbing.NewBranchReferenceName(name)); err == nil {
		return h, err
	}

	if h, err := getHashFromReference(repo, plumbing.NewTagReferenceName(name)); err == nil {
		return h, err
	}

	if h, err := getHashFromReference(repo, plumbing.NewRemoteHEADReferenceName(name)); err == nil {
		return h, err
	}

	return plumbing.ZeroHash, fmt.Errorf("unknown target %q", name)
}

func checkoutRevision(directory, token, commit string) error {

	fmt.Printf("Accessing %s", directory)
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
		fmt.Printf("Error fetching")
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		fmt.Printf("Error obtaining worktree")
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

	fmt.Printf("git checkout %s (%s)", h, commit)

	if err := w.Checkout(&coptions); err != nil && err != git.NoErrAlreadyUpToDate {
		fmt.Printf("Error during checkout")
		return err
	}

	// ... retrieving the commit being pointed by HEAD, it shows that the
	// repository is pointing to the giving commit in detached mode
	fmt.Printf("git show-ref --head HEAD")
	ref, err := repo.Head()
	if err != nil {
		fmt.Printf("Error obtaining HEAD")
		return err
	}

	fmt.Printf("%s", ref.Hash())
	return err

}

func cloneRepo(url string, directory string, token string) error {

	fmt.Printf("git clone %s into %s", url, directory)

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
	fmt.Printf("git show-ref --head HEAD")
	ref, err := repo.Head()
	if err != nil {
		return err
	}
	fmt.Printf("%s", ref.Hash())
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
