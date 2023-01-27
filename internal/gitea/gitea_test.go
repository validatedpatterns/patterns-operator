package gitea

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("gitea server", func() {

	var _ = Context("validates local functions", func() {

		It("correctly retrieves the IDs from the namespace annotation", func() {
			var (
				originalUID = 1000400000
				originalGID = 1000500000
				ns          = v1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name: Namespace,
					Annotations: map[string]string{"openshift.io/sa.scc.supplemental-groups": fmt.Sprintf("%d/10000", originalGID),
						"openshift.io/sa.scc.uid-range": fmt.Sprintf("%d/10000", originalUID)},
				}}

				kCli = kubeClient{
					cli: newFakeClient(&ns),
				}
			)
			uid, gid, err := kCli.getUserGroupIDsFromNamespace()
			Expect(err).NotTo(HaveOccurred())
			Expect(uid).To(Equal(originalUID))
			Expect(gid).To(Equal(originalGID))
		})

		It("creates the secret with the user's credentials", func() {
			kCli := kubeClient{
				cli: newFakeClient(),
			}
			s, err := kCli.CreateUserCredentials("foo", "bar@foo.com", "foo-bar")
			Expect(err).NotTo(HaveOccurred())
			Expect(s).NotTo(BeNil())
			Expect(s.Data).NotTo(BeNil())
			Expect(s.Data["username"]).To(Equal([]byte("foo")))
			Expect(s.Data["password"]).To(HaveLen(16))
			Expect(s.Data["email"]).To(Equal([]byte("bar@foo.com")))
			Expect(s.Name).To(Equal("foo-bar"))
		})

		It("extracts the project name from the repo URL", func() {
			repoURL := "https://github.com/hybrid-cloud-patterns/multicloud-gitops"
			ret, err := extractGitRepoNameFromURL(repoURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(ret).To(Equal("multicloud-gitops"))
		})

		It("extracts the project name from the repo URL when it contains the suffix '.git'", func() {
			repoURL := "https://github.com/hybrid-cloud-patterns/multicloud-gitops.git"
			ret, err := extractGitRepoNameFromURL(repoURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(ret).To(Equal("multicloud-gitops"))
		})

		It("generates the custom chart values", func() {
			v, err := generateCustomChartValues(10, 20, "foo.domain")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal("containerSecurityContext:\n    allowPrivilegeEscalation: false\n    capabilities:\n        drop:\n            - ALL\n    runAsNonRoot: true\n    runAsUser: null\n    seccompProfile:\n        type: RuntimeDefault\ngitea:\n    admin:\n        existingSecret: gitea-admin-credentials\n    config:\n        server:\n            DISABLE_SSH: true\n            HTTP_PORT: 3000\n            ROOT_URL: https://foo.domain\n            START_SSH_SERVER: false\n        service:\n            DISABLE_REGISTRATION: true\nimage:\n    rootless: true\nmemcached:\n    enabled: false\npersistence:\n    size: 1Gi\npodSecurityContext: null\npostgresql:\n    containerSecurityContext:\n        allowPrivilegeEscalation: false\n        capabilities:\n            drop:\n                - ALL\n        runAsNonRoot: true\n        runAsUser: 10\n        seccompProfile:\n            type: RuntimeDefault\n    persistence:\n        size: 1Gi\n    securityContext:\n        fsGroup: 20\nstatefulset:\n    env:\n        - name: HOME\n          value: /data/git\n"))
		})

		It("generates a random password of the desired length", func() {
			p, err := generatePassword(16, 4, 4, 4)
			Expect(err).NotTo(HaveOccurred())
			Expect(p).To(HaveLen(16))
		})
	})

})

func newFakeClient(initObjects ...runtime.Object) rtclient.WithWatch {
	return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(initObjects...).Build()
}
