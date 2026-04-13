package console

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConsole(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Console Suite")
}

var _ = BeforeSuite(func() {
	// Ensure getDeploymentNamespace() always returns defaultNamespace in tests,
	// regardless of whether /var/run/secrets/.../namespace exists (e.g. in CI).
	os.Setenv("OPERATOR_NAMESPACE", defaultNamespace)
})
