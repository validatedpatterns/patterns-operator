module github.com/hybrid-cloud-patterns/patterns-operator

go 1.16

require (
	github.com/argoproj/argo-cd/v2 v2.3.0-rc5.0.20220206192056-4b04a3918029
	github.com/ghodss/yaml v1.0.0
	github.com/go-errors/errors v1.4.2
	github.com/go-logr/logr v1.2.2
	github.com/google/uuid v1.3.0 // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v0.0.0-20211028023115-7224b732cc14 // indirect
	github.com/openshift/client-go v0.0.0-20210831095141-e19a065e79f7
	github.com/operator-framework/api v0.12.0
	github.com/operator-framework/operator-lifecycle-manager v0.20.0
	k8s.io/apimachinery v0.23.1
	k8s.io/cli-runtime v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/kubectl v0.23.1
	sigs.k8s.io/controller-runtime v0.11.0
)

replace (
	github.com/go-check/check => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
	//	github.com/bombsimon/logrusr => github.com/bombsimon/logrusr v2.0.1
	// Caused by Argo importing 'k8s.io/api'
	// Override all the v0.0.0 entries
	k8s.io/api => k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.23.0
	k8s.io/apiserver => k8s.io/apiserver v0.23.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.23.0
	k8s.io/client-go => k8s.io/client-go v0.23.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.23.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.23.0
	k8s.io/code-generator => k8s.io/code-generator v0.23.0
	k8s.io/component-base => k8s.io/component-base v0.23.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.23.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.23.0
	k8s.io/cri-api => k8s.io/cri-api v0.23.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.23.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.23.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.23.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.23.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.23.0
	k8s.io/kubectl => k8s.io/kubectl v0.23.0
	k8s.io/kubelet => k8s.io/kubelet v0.23.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.23.0
	k8s.io/metrics => k8s.io/metrics v0.23.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.23.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.23.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.23.0

)
