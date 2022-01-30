module github.com/hybrid-cloud-patterns/patterns-operator

go 1.16

require (
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v1.2.2
	github.com/google/uuid v1.3.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	helm.sh/helm/v3 v3.8.0
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
	k8s.io/helm v2.17.0+incompatible
	sigs.k8s.io/controller-runtime v0.11.0
)
