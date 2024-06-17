package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	gomock "go.uber.org/mock/gomock"
	"k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MigrateGiteaRepo", func() {
	var (
		mockCtrl         *gomock.Controller
		mockKubeClient   *fake.Clientset
		giteaServer      *httptest.Server
		giteaServerRoute string
		giteaOperations  GiteaOperations
		username         string
		password         string
		upstreamURL      string
		repoName         string
		//descriptionFormat string
		//description       string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockKubeClient = fake.NewSimpleClientset()
		giteaOperations = &GiteaOperationsImpl{}
		username = "user"
		password = "pass"
		upstreamURL = "https://github.com/example/repo.git"
		repoName = "repo"
		//descriptionFormat = "The [%s] repository was migrated by the Validated Patterns Operator."
		//description = fmt.Sprintf(descriptionFormat, repoName)

		// Mock Gitea server
		giteaServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				w.WriteHeader(http.StatusOK)
			case fmt.Sprintf("/repos/%s/%s", GiteaAdminUser, repoName):
				w.WriteHeader(http.StatusNotFound)
			case "/api/v1/version":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"version": "1.21.11"}`))
			case "/api/v1/repos/migrate":
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"html_url": "https://gitea.example.com/user/repo"}`))
			default:
				fmt.Printf("FOKKO: %v\n", r.URL)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		giteaServerRoute = giteaServer.URL
	})

	AfterEach(func() {
		mockCtrl.Finish()
		giteaServer.Close()
	})

	Context("when the repository does not exist", func() {
		It("should migrate the repository successfully", func() {
			success, repositoryURL, err := giteaOperations.MigrateGiteaRepo(mockKubeClient, username, password, upstreamURL, giteaServerRoute)
			Expect(err).ToNot(HaveOccurred())
			Expect(success).To(BeTrue())
			Expect(repositoryURL).To(Equal("https://gitea.example.com/user/repo"))
		})
	})

	Context("when the repository already exists", func() {
		BeforeEach(func() {
			giteaServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					w.WriteHeader(http.StatusOK)
				case fmt.Sprintf("/repos/%s/%s", GiteaAdminUser, repoName):
					w.WriteHeader(http.StatusNotFound)
				case "/api/v1/version":
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"version": "1.21.11"}`))
				case "/api/v1/repos/migrate":
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte(`{"html_url": "https://gitea.example.com/user/repo"}`))
				case fmt.Sprintf("/repos/%s/%s", GiteaAdminUser, repoName):
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"html_url": "https://gitea.example.com/user/repo"}`))
				default:
					fmt.Printf("FOKKO: %v\n", r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			giteaServerRoute = giteaServer.URL
		})

		It("should not migrate the repository and return the existing repository URL", func() {
			success, repositoryURL, err := giteaOperations.MigrateGiteaRepo(mockKubeClient, username, password, upstreamURL, giteaServerRoute)
			Expect(err).ToNot(HaveOccurred())
			Expect(success).To(BeTrue())
			Expect(repositoryURL).To(Equal("https://gitea.example.com/user/repo"))
		})
	})

	Context("when there is an error creating the Gitea client", func() {
		It("should return an error", func() {
			// Use an invalid Gitea server route to simulate client creation failure
			invalidRoute := "http://invalid-url"
			success, repositoryURL, err := giteaOperations.MigrateGiteaRepo(mockKubeClient, username, password, upstreamURL, invalidRoute)
			Expect(err).To(HaveOccurred())
			Expect(success).To(BeFalse())
			Expect(repositoryURL).To(BeEmpty())
		})
	})

	Context("when there is an error during repository migration", func() {
		BeforeEach(func() {
			giteaServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/repos/migrate" {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			giteaServerRoute = giteaServer.URL
		})

		It("should return an error", func() {
			success, repositoryURL, err := giteaOperations.MigrateGiteaRepo(mockKubeClient, username, password, upstreamURL, giteaServerRoute)
			Expect(err).To(HaveOccurred())
			Expect(success).To(BeFalse())
			Expect(repositoryURL).To(BeEmpty())
		})
	})
})
