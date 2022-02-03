module github.com/hybrid-cloud-patterns/patterns-operator

go 1.16

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-errors/errors v1.4.2
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v1.2.2
	github.com/google/uuid v1.3.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v0.0.0-20211028023115-7224b732cc14 // indirect
	github.com/openshift/client-go v0.0.0-20210831095141-e19a065e79f7
	github.com/operator-framework/api v0.12.0
	helm.sh/helm/v3 v3.8.0
	k8s.io/api v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
	k8s.io/helm v2.17.0+incompatible // indirect
	sigs.k8s.io/controller-runtime v0.11.0
)
