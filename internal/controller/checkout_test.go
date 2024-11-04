package controllers

import (
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	//+kubebuilder:scaffold:imports
)

var gitRepoURL = "https://github.com/validatedpatterns/.github"
var gitCommitHash = "d0f3fb283cfb17189cba89aa5ff57fd8dcb2a7fd"

var _ = Describe("Git Functions", func() {
	Context("cloneRepo", func() {
		It("should clone a repository and get the HEAD", func() {
			err := cloneRepo(nil, gitOpsImpl, gitRepoURL, tempDir, nil)
			Expect(err).ToNot(HaveOccurred())
			refHash, err := repoHash(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(refHash).ToNot(BeNil())
		})
	})

	Context("checkoutRevision", func() {
		It("should checkout a specific commit", func() {
			err := checkoutRevision(nil, gitOpsImpl, gitRepoURL, tempDir, gitCommitHash, nil) // some older existing commit hash
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("repoHash", func() {
		It("should get the repository hash", func() {
			refHash, err := repoHash(tempDir)
			Expect(err).ToNot(HaveOccurred())
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
			err := checkout(nil, gitOpsImpl, gitRepoURL, tempDir2, gitCommitHash, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should checkout repository without checking out if commit is empty", func() {
			err := checkout(nil, gitOpsImpl, gitRepoURL, tempDir, "", nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should checkout a repository and switch to its remote branch", func() {
			err := checkout(nil, gitOpsImpl, gitRepoURL, tempDir, "test-do-not-use", nil)
			Expect(err).ToNot(HaveOccurred())
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
			Expect(err).ToNot(HaveOccurred())

			// Create a commit to have a valid branch reference.
			commitHash, err := createTestCommit(repo, "test-branch", "Test commit on branch")
			Expect(err).ToNot(HaveOccurred())

			// Get hash from the branch reference.
			hash, err := getHashFromReference(repo, plumbing.NewBranchReferenceName("test-branch"))
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).To(Equal(commitHash))
		})

		It("should get hash from tag reference", func() {
			// Set up a test repository with a tag reference.
			repo, err := git.PlainInit(tempDir2, false)
			Expect(err).ToNot(HaveOccurred())

			// Create a commit to tag.
			commitHash, err := createTestCommit(repo, "main", "Test commit for tagging")
			Expect(err).ToNot(HaveOccurred())

			// Tag the commit.
			tagName := "v1.0.0"
			err = createTestTag(repo, commitHash, tagName)
			Expect(err).ToNot(HaveOccurred())

			// Get hash from the tag reference.
			hash, err := getHashFromReference(repo, plumbing.NewTagReferenceName(tagName))
			Expect(err).ToNot(HaveOccurred())
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
			When:  time.Now(),
		},
		All:               true,
		AllowEmptyCommits: true,
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
