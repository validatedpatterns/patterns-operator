package controllers

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Git Functions", func() {

	Context("checkoutRevision", func() {
		It("should checkout a specific commit", func() {
			err := checkoutRevision(tempDir, "", "3086ab9e72e9f9ea369813c76f35772a3c8ea2a4") //some older existing commit hash
			Expect(err).To(BeNil())
		})
	})

	Context("cloneRepo", func() {
		It("should clone a repository and get the HEAD", func() {
			refHash, err := repoHash(tempDir)
			Expect(err).To(BeNil())
			Expect(refHash).ToNot(BeNil())
		})
	})

	Context("repoHash", func() {
		It("should get the repository hash", func() {
			refHash, err := repoHash(tempDir)
			Expect(err).To(BeNil())
			Expect(refHash).ToNot(BeNil())
		})
	})

	Context("checkout", func() {
		var tempDir2 string
		var _ = BeforeEach(func() {
			tempDir2 = createTempDir("vp-test")
		})
		var _ = AfterEach(func() {
			cleanupTempDir(tempDir2)
		})
		It("should clone repository and checkout a specific commit", func() {
			token := "your_token"
			commit := "3086ab9e72e9f9ea369813c76f35772a3c8ea2a4"

			err := checkout(tempLocalGitCopy, tempDir2, token, commit)
			Expect(err).To(BeNil())
		})

		It("should clone repository without checking out if commit is empty", func() {
			token := "your_token"
			commit := ""

			err := checkout(tempLocalGitCopy, tempDir2, token, commit)
			Expect(err).To(BeNil())
		})
	})

	Context("getHashFromReference", func() {
		var tempDir2 string
		var _ = BeforeEach(func() {
			tempDir2 = createTempDir("vp-test")
		})
		var _ = AfterEach(func() {
			cleanupTempDir(tempDir2)
		})
		It("should get hash from branch reference", func() {
			// Set up a test repository with a branch reference.
			repo, err := git.PlainInit(tempDir2, false)
			Expect(err).To(BeNil())

			// Create a commit to have a valid branch reference.
			commitHash, err := createTestCommit(repo, "test-branch", "Test commit on branch")
			Expect(err).To(BeNil())

			// Get hash from the branch reference.
			hash, err := getHashFromReference(repo, plumbing.NewBranchReferenceName("test-branch"))
			Expect(err).To(BeNil())
			Expect(hash).To(Equal(commitHash))
		})

		It("should get hash from tag reference", func() {
			// Set up a test repository with a tag reference.
			repo, err := git.PlainInit(tempDir2, false)
			Expect(err).To(BeNil())

			// Create a commit to tag.
			commitHash, err := createTestCommit(repo, "main", "Test commit for tagging")
			Expect(err).To(BeNil())

			// Tag the commit.
			tagName := "v1.0.0"
			err = createTestTag(repo, commitHash, tagName)
			Expect(err).To(BeNil())

			// Get hash from the tag reference.
			hash, err := getHashFromReference(repo, plumbing.NewTagReferenceName(tagName))
			Expect(err).To(BeNil())
			Expect(hash).To(Equal(commitHash))
		})
	})
})

// Function to create a test commit and return its hash.
func createTestCommit(repo *git.Repository, branchName, commitMessage string) (plumbing.Hash, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	commit, err := worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
		},
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	refName := plumbing.NewBranchReferenceName(branchName)
	branchRef := plumbing.NewHashReference(refName, commit)
	if err := repo.Storer.SetReference(branchRef); err != nil {
		return plumbing.ZeroHash, err
	}

	return commit, nil
}

// Function to create a test tag for a given commit hash.
func createTestTag(repo *git.Repository, commitHash plumbing.Hash, tagName string) error {
	_ = plumbing.NewTagReferenceName(tagName)
	_, err := repo.CreateTag(tagName, commitHash, nil)
	return err
}
