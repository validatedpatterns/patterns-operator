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

var _ = Describe("getUserFromURL", func() {
	Context("with SSH URL", func() {
		It("should return the user from git@github.com:user/repo", func() {
			Expect(getUserFromURL("git@github.com:user/repo")).To(Equal("git"))
		})
	})

	Context("with custom user in SSH URL", func() {
		It("should return the user from customuser@gitlab.com:user/repo", func() {
			Expect(getUserFromURL("customuser@gitlab.com:user/repo")).To(Equal("customuser"))
		})
	})

	Context("with HTTPS URL", func() {
		It("should return empty string", func() {
			Expect(getUserFromURL("https://github.com/user/repo")).To(Equal(""))
		})
	})

	Context("with plain URL", func() {
		It("should return empty string", func() {
			Expect(getUserFromURL("github.com/user/repo")).To(Equal(""))
		})
	})
})

var _ = Describe("getLocalGitPath", func() {
	It("should return a valid path for HTTPS URLs", func() {
		result := getLocalGitPath("https://github.com/user/repo")
		Expect(result).ToNot(BeEmpty())
		Expect(result).To(ContainSubstring(VPTmpFolder))
	})

	It("should return a valid path for SSH URLs", func() {
		result := getLocalGitPath("git@github.com:user/repo.git")
		Expect(result).ToNot(BeEmpty())
		Expect(result).To(ContainSubstring(VPTmpFolder))
	})

	It("should return different paths for different repos", func() {
		path1 := getLocalGitPath("https://github.com/user/repo1")
		path2 := getLocalGitPath("https://github.com/user/repo2")
		Expect(path1).ToNot(Equal(path2))
	})
})

var _ = Describe("detectGitAuthType", func() {
	Context("with SSH secret", func() {
		It("should return GitAuthSsh", func() {
			secret := map[string][]byte{
				"sshPrivateKey": []byte("private-key-data"),
			}
			Expect(detectGitAuthType(secret)).To(Equal(GitAuthSsh))
		})
	})

	Context("with password secret", func() {
		It("should return GitAuthPassword", func() {
			secret := map[string][]byte{
				"username": []byte("user"),
				"password": []byte("pass"),
			}
			Expect(detectGitAuthType(secret)).To(Equal(GitAuthPassword))
		})
	})

	Context("with GitHub App secret", func() {
		It("should return GitAuthGitHubApp", func() {
			secret := map[string][]byte{
				"githubAppID":             []byte("12345"),
				"githubAppInstallationID": []byte("67890"),
				"githubAppPrivateKey":     []byte("key-data"),
			}
			Expect(detectGitAuthType(secret)).To(Equal(GitAuthGitHubApp))
		})
	})

	Context("with empty secret", func() {
		It("should return GitAuthNone", func() {
			secret := map[string][]byte{}
			Expect(detectGitAuthType(secret)).To(Equal(GitAuthNone))
		})
	})

	Context("with nil secret", func() {
		It("should return GitAuthNone", func() {
			var secret map[string][]byte
			Expect(detectGitAuthType(secret)).To(Equal(GitAuthNone))
		})
	})

	Context("with partial password secret (missing password)", func() {
		It("should return GitAuthNone", func() {
			secret := map[string][]byte{
				"username": []byte("user"),
			}
			Expect(detectGitAuthType(secret)).To(Equal(GitAuthNone))
		})
	})
})

var _ = Describe("getField", func() {
	Context("when the field exists", func() {
		It("should return the value", func() {
			secret := map[string][]byte{
				"username": []byte("testuser"),
			}
			Expect(string(getField(secret, "username"))).To(Equal("testuser"))
		})
	})

	Context("when the field does not exist", func() {
		It("should return nil", func() {
			secret := map[string][]byte{
				"username": []byte("testuser"),
			}
			Expect(getField(secret, "nonexistent")).To(BeNil())
		})
	})
})

var _ = Describe("getHttpAuth", func() {
	It("should return BasicAuth with correct credentials", func() {
		secret := map[string][]byte{
			"username": []byte("myuser"),
			"password": []byte("mypass"),
		}
		auth := getHttpAuth(secret)
		Expect(auth).ToNot(BeNil())
		Expect(auth.Username).To(Equal("myuser"))
		Expect(auth.Password).To(Equal("mypass"))
	})
})

var _ = Describe("getFetchOptions", func() {
	Context("with no authentication", func() {
		It("should return options without auth", func() {
			opts, err := getFetchOptions(nil, "https://github.com/user/repo", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts).ToNot(BeNil())
			Expect(opts.RemoteName).To(Equal("origin"))
			Expect(opts.Force).To(BeTrue())
			Expect(opts.Auth).To(BeNil())
		})
	})

	Context("with password authentication", func() {
		It("should return options with basic auth", func() {
			secret := map[string][]byte{
				"username": []byte("user"),
				"password": []byte("pass"),
			}
			opts, err := getFetchOptions(nil, "https://github.com/user/repo", secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts.Auth).ToNot(BeNil())
		})
	})
})

var _ = Describe("getCloneOptions", func() {
	Context("with no authentication", func() {
		It("should return options without auth", func() {
			opts, err := getCloneOptions(nil, "https://github.com/user/repo", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts).ToNot(BeNil())
			Expect(opts.URL).To(Equal("https://github.com/user/repo"))
			Expect(opts.RemoteName).To(Equal("origin"))
			Expect(opts.Auth).To(BeNil())
		})
	})

	Context("with password authentication", func() {
		It("should return options with basic auth", func() {
			secret := map[string][]byte{
				"username": []byte("user"),
				"password": []byte("pass"),
			}
			opts, err := getCloneOptions(nil, "https://github.com/user/repo", secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts.Auth).ToNot(BeNil())
		})
	})

	Context("with SSH authentication and invalid key", func() {
		It("should return an error", func() {
			secret := map[string][]byte{
				"sshPrivateKey": []byte("invalid-key"),
			}
			_, err := getCloneOptions(nil, "git@github.com:user/repo", secret)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with SSH authentication and missing key", func() {
		It("should return an error for missing sshPrivateKey", func() {
			secret := map[string][]byte{
				"sshPrivateKey": nil,
			}
			_, err := getCloneOptions(nil, "git@github.com:user/repo", secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not get sshPrivateKey"))
		})
	})

	Context("with GitHub App authentication and missing fields", func() {
		It("should return an error for invalid app credentials", func() {
			secret := map[string][]byte{
				"githubAppID":             []byte("notanumber"),
				"githubAppInstallationID": []byte("12345"),
				"githubAppPrivateKey":     []byte("invalid-key"),
			}
			_, err := getCloneOptions(nil, "https://github.com/user/repo", secret)
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("getSshPublicKey", func() {
	Context("with valid SSH key", func() {
		It("should return public keys", func() {
			testKey := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\n" +
				"b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABFwAAAAdzc2gtcn\n" +
				"NhAAAAAwEAAQAAAQEAoRkE3Prx0r2BD4F1MY8xw1Sv6y1/L1bnt5S7CtrBXtQ4bpVzuuqs\n" +
				"CxCzhhW/qf59LoEFJU3qpU934Ss1hKFFwFD4ad20rBSJun6h0dLjPE65h11WlNz9DtbTmk\n" +
				"EnC4vGRcVBRDyZ8Nk1IkOs57kGMDd3R+kYSWjVH34MXZGq3LPCTbXRZUk+KXAjUcyjqJYa\n" +
				"rld/bAlNdZ1a3QY2Osb38T2BHdl5FjRV3o5u5449v3HlA4ky/yizSkb5f/6ihQXLMIEfq6\n" +
				"UI2ycvMjPxEkgcI1acGukljFRtePTXIYqPaMWV8qSFGaDXrHvpYOS32jcfIjDoDrN5yTXr\n" +
				"zSZ+OMshlwAAA8gUsDamFLA2pgAAAAdzc2gtcnNhAAABAQChGQTc+vHSvYEPgXUxjzHDVK\n" +
				"/rLX8vVue3lLsK2sFe1DhulXO66qwLELOGFb+p/n0ugQUlTeqlT3fhKzWEoUXAUPhp3bSs\n" +
				"FIm6fqHR0uM8TrmHXVaU3P0O1tOaQScLi8ZFxUFEPJnw2TUiQ6znuQYwN3dH6RhJaNUffg\n" +
				"xdkarcs8JNtdFlST4pcCNRzKOolhquV39sCU11nVrdBjY6xvfxPYEd2XkWNFXejm7njj2/\n" +
				"ceUDiTL/KLNKRvl//qKFBcswgR+rpQjbJy8yM/ESSBwjVpwa6SWMVG149Nchio9oxZXypI\n" +
				"UZoNese+lg5LfaNx8iMOgOs3nJNevNJn44yyGXAAAAAwEAAQAAAQA7wC9VFQBnZSE+0onY\n" +
				"oV9YLwt2o2/Wa5nTNedv/bYWCYmKvoTnsY2xJvcnBt8JWposiu8RKIac3M4+ZkvZzwUzcP\n" +
				"TKM1CFOLLiyIAVdm4Q2rQmeGCaIyL7A4QFZR/pwOR/0UtFV2LTeYSjGk3BvpcEgDYOJm77\n" +
				"H1ZY8WP9un8Qj0ceRTD36eNYI75NPO3gEgT2BIaZ9t09u7CkHags/forLvubzmYOfIMeXN\n" +
				"nadsmOWRsaBqQrtuH3qbtLsNGuVwE/FDxl9SpLbK7sKOVGG6JmpL6OXGEhJxgMuAYOguia\n" +
				"V3Xrzt0deiQRGO30THObpS1g+fVkLlRiMzWLGoKeXAaVAAAAgCY6DNY2DahM84M2qhj8Ef\n" +
				"5ypRnqJ6HUoFgV4Hf8sgOXiDamhEwAsbeC94WYTBtMWsQgnmrlYgK7jTxusXbEg5Ac+7Ah\n" +
				"Zc3g2rkn/S4xNhZKJMxlzPVhYLKQhc9ZpCOXK0TjMs/3V2yFdfMoCp6AUUj382NLjNGlkx\n" +
				"gIPf9t6bmYAAAAgQDMp7cLP2bamWqY8SzXlUCsH12nm/txE74G2BCJpR8wBoHkKjGbrdzE\n" +
				"LFRoqs4nPsGIoXS2n4GyZZoY3dUEN6lmWlRUrE8lxhq1Ob4KZ+u3+ozQpr3WS8qIvtNmhn\n" +
				"2T5jXyDgnTf6oZmNZavJ8f4Tjm5p2PXAkbzMC9yH4+DjHOEwAAAIEAyYPDioLOqmCs4pNa\n" +
				"ZauWNlLt74Zmnr/mfxHiXHUnIliOXvgx38SVX5vOD7O3HAEMRDuD+lhdJH7LZhuuhQTj34\n" +
				"ZtzOASRqSaCPd9AAf2bZ/aar69YSaDLA4gKvrjyqqeK9VuKUEKsGoiID2NNzTB8kIfbNVG\n" +
				"lAzsUT+xOKn2fu0AAAANbWljaGVsZUBvc2hpZQECAwQFBg==\n" +
				"-----END OPENSSH PRIVATE KEY-----\n")

			secret := map[string][]byte{
				"sshPrivateKey": testKey,
			}
			publicKey, err := getSshPublicKey("git@github.com:user/repo", secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(publicKey).ToNot(BeNil())
			Expect(publicKey.User).To(Equal("git"))
		})
	})

	Context("with missing SSH key", func() {
		It("should return an error", func() {
			secret := map[string][]byte{}
			_, err := getSshPublicKey("git@github.com:user/repo", secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not get sshPrivateKey"))
		})
	})

	Context("with invalid SSH key", func() {
		It("should return an error", func() {
			secret := map[string][]byte{
				"sshPrivateKey": []byte("not-a-valid-key"),
			}
			_, err := getSshPublicKey("git@github.com:user/repo", secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not get publicKey"))
		})
	})
})

var _ = Describe("getGitRemoteURL", func() {
	var tempDir2 string

	BeforeEach(func() {
		tempDir2 = createTempDir("vp-remote-test")
	})
	AfterEach(func() {
		cleanupTempDir(tempDir2)
	})

	Context("when repository exists with a remote", func() {
		It("should return the remote URL", func() {
			err := cloneRepo(nil, gitOpsImpl, gitRepoURL, tempDir2, nil)
			Expect(err).ToNot(HaveOccurred())

			url, err := getGitRemoteURL(tempDir2, "origin")
			Expect(err).ToNot(HaveOccurred())
			Expect(url).To(Equal(gitRepoURL))
		})
	})

	Context("when repository does not exist", func() {
		It("should return an error", func() {
			_, err := getGitRemoteURL("/nonexistent/path", "origin")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when remote does not exist", func() {
		It("should return an error", func() {
			_, err := git.PlainInit(tempDir2+"_init", false)
			Expect(err).ToNot(HaveOccurred())
			defer cleanupTempDir(tempDir2 + "_init")

			_, err = getGitRemoteURL(tempDir2+"_init", "nonexistent")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("repoHash on non-existent repo", func() {
	It("should return error", func() {
		_, err := repoHash("/nonexistent/path")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("getCommitFromTarget", func() {
	var tempDir2 string
	var repo *git.Repository

	BeforeEach(func() {
		tempDir2 = createTempDir("vp-commit-test")
		var err error
		repo, err = git.PlainInit(tempDir2, false)
		Expect(err).ToNot(HaveOccurred())

		_, err = createTestCommit(repo, "main", "Initial commit")
		Expect(err).ToNot(HaveOccurred())

		// Set HEAD to the main branch
		ref := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main"))
		err = repo.Storer.SetReference(ref)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		cleanupTempDir(tempDir2)
	})

	It("should return HEAD hash when target is HEAD", func() {
		hash, err := getCommitFromTarget(repo, "HEAD")
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).ToNot(Equal(plumbing.ZeroHash))
	})

	It("should return hash for branch name", func() {
		hash, err := getCommitFromTarget(repo, "main")
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).ToNot(Equal(plumbing.ZeroHash))
	})

	It("should return main when target is empty", func() {
		hash, err := getCommitFromTarget(repo, "")
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).ToNot(Equal(plumbing.ZeroHash))
	})

	It("should return error for unknown target", func() {
		_, err := getCommitFromTarget(repo, "nonexistent-ref-xyz")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown target"))
	})
})
