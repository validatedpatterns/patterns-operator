# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.4
OPERATOR_NAME ?= patterns
GOFLAGS=-mod=vendor
REGISTRY ?= localhost
UPLOADREGISTRY ?= quay.io/validatedpatterns
GOLANGCI_IMG ?= docker.io/golangci/golangci-lint
GOLANGCI_VERSION ?= 2.8.0

# CI uses a non-writable home dir, make sure .cache is writable
ifeq ("${HOME}", "/")
HOME=/tmp
export HOME=/tmp
endif

APIKEYFILE ?= internal/controller/apikey.txt

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Override CONTAINER_PLATFORM and CONTAINER_OS to build for a different OS and
# architecture. These should probably always be amd64/linux unless you're
# running the container images on another OS or arch.
CONTAINER_OS ?= linux
CONTAINER_PLATFORM ?= amd64

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# quay.io/validatedpatterns/patterns-operator-bundle:$VERSION and quay.io/validatedpatterns/patterns-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= $(UPLOADREGISTRY)/$(OPERATOR_NAME)-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)
OPERATOR_IMG ?= $(OPERATOR_NAME)-operator:$(VERSION)
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.30

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: apikey controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: apikey manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -v ./... -coverprofile cover.out
	go tool cover -html="cover.out" -o coverage.html

##@ Build

# Override GOOS and GOARCH to build for a different OS and architecture. For
# example, MacOS M series (arm64) should be: GOOS=darwin GOARCH=arm64 make build
GOOS ?= linux
GOARCH ?= amd64

.PHONY: apikey
apikey: ## Generates an empty apikey file if one does not exist already
	@touch $(APIKEYFILE)

.PHONY: build
build: apikey generate fmt vet ## Build manager binary.
	GOOS=${GOOS} GOARCH=${GOARCH} hack/build.sh

.PHONY: clean
clean: ## Remove build artifacts and downloaded tools
	find bin/ -exec chmod +w "{}" \;
	rm -rf ./manager ./bin/*

.PHONY: run
run: apikey manifests generate fmt vet ## Run a controller from your host.
	GOOS=${GOOS} GOARCH=${GOARCH} hack/build.sh run


##@ Conatiner-related tasks
.PHONY: buildah-manifest
buildah-manifest: apikey ## creates the buildah manifest for multi-arch images
	# The rm is needed due to bug https://www.github.com/containers/podman/issues/19757
	buildah manifest rm "${REGISTRY}/${OPERATOR_IMG}" || /bin/true
	buildah manifest create "${REGISTRY}/${OPERATOR_IMG}"

.PHONY: podman-build-amd64
podman-build-amd64: buildah-manifest ## build the container in amd64
	@echo "Building the operator amd64"
	buildah build --secret id=apikey,src=$(APIKEYFILE) --platform linux/amd64 --format docker -f Dockerfile -t "${OPERATOR_IMG}-amd64"
	buildah manifest add --arch=amd64 "${REGISTRY}/${OPERATOR_IMG}" "${REGISTRY}/${OPERATOR_IMG}-amd64"

.PHONY: podman-build-arm64
podman-build-arm64: buildah-manifest ## build the container in arm64
	@echo "Building the operator arm64"
	buildah build --secret id=apikey,src=$(APIKEYFILE) --platform linux/arm64 --build-arg GOARCH="arm64" --format docker -f Dockerfile -t "${OPERATOR_IMG}-arm64"
	buildah manifest add --arch=arm64 "${REGISTRY}/${OPERATOR_IMG}" "${REGISTRY}/${OPERATOR_IMG}-arm64"

.PHONY: buildah-push
buildah-push: ## Uploads the container to quay.io/validatedpatterns/${OPERATOR_IMG}
	@echo "Uploading the ${REGISTRY}/${OPERATOR_IMG} container to ${UPLOADREGISTRY}/${OPERATOR_IMG}"
	buildah manifest push --all "${REGISTRY}/${OPERATOR_IMG}" "docker://${UPLOADREGISTRY}/${OPERATOR_IMG}"

.PHONY: golangci-lint
golangci-lint: apikey ## Run golangci-lint locally
	podman run --pull=newer --rm -v $(PWD):/app:rw,z -w /app $(GOLANGCI_IMG):v$(GOLANGCI_VERSION) golangci-lint run -v

##@ Legacy docker tasks
.PHONY: docker-build
docker-build: apikey podman-build-amd64 ## Build docker image with the manager.

.PHONY: docker-push
docker-push: buildah-push ## Push docker image with the manager.

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.16.4
ENVTEST_VERSION ?= release-0.19
GOLANGCI_LINT_VERSION ?= v2.0.2
GOVULNCHECK_VERSION ?= v1.1.4
# parameters to pass to govulnscan
GOVULNCHECK_OPTS ?=
# update for major version updates to YQ_VERSION!
YQ_API_VERSION = v4
YQ_VERSION = v4.41.1
KUSTOMIZE_VERSION ?= v5.4.3
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
YQ = $(LOCALBIN)/yq-$(YQ_VERSION)
GOVULNCHECK ?= $(LOCALBIN)/govulncheck-$(GOVULNCHECK_VERSION)

## Tool Versions
OPERATOR_SDK_VERSION ?= v1.37.0

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=linux && ARCH=amd64 && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
endif

.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	go build -o $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION) sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	go build -o $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION) sigs.k8s.io/controller-runtime/tools/setup-envtest

.PHONY: govulncheck
govulncheck: $(GOVULNCHECK) ## Download govulncheck
$(GOVULNCHECK): $(LOCALBIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))

.PHONY: govulnscan
govulnscan: govulncheck
	$(GOVULNCHECK) $(GOVULNCHECK_OPTS) ./... 2>&1 | tee govulncheck.results

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(MAKE) bundle-fixes bundle-date
	./hack/set_openshift_minimum_version.sh
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build --platform $(CONTAINER_OS)/$(CONTAINER_PLATFORM) -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	docker push $(BUNDLE_IMG)

.PHONY: csv-date
csv-date: ## Set createdAt date in the CSV.
	sed -r -i "s|createdAt: .*|createdAt: `date '+%Y-%m-%d %T'`|;"  ./config/manifests/bases/$(OPERATOR_NAME)-operator.clusterserviceversion.yaml

# Some fixes in the bundle
.PHONY: bundle-fixes
bundle-fixes: ## update container image
	sed -r -i "s|containerImage: .*|containerImage: $(IMG)|;" ./bundle/manifests/$(OPERATOR_NAME)-operator.clusterserviceversion.yaml

.PHONY: bundle-date
bundle-date: ## Set createdAt date in the bundle's CSV. Do not commit changes!
	sed -r -i "s|createdAt: .*|createdAt: `date '+%Y-%m-%d %T'`|;" ./bundle/manifests/$(OPERATOR_NAME)-operator.clusterserviceversion.yaml

.PHONY: bundle-date-reset
bundle-date-reset: ## Reset createdAt date to empty value
	sed -r -i "s|createdAt: .*|createdAt: \"\"|;" ./bundle/manifests/$(OPERATOR_NAME)-operator.clusterserviceversion.yaml

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.26.5/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build an OLM catalog image by adding the bundle image to a simple catalog using the
# operator package manager tool, 'opm'. For more information see:
# https://olm.operatorframework.io/docs/reference/catalog-templates
.PHONY: catalog-build
catalog-build: opm ## Build an OLM catalog image (for testing).
	cat catalog/template.yaml | BUNDLE_IMG=$(BUNDLE_IMG) VERSION=$(VERSION) envsubst > catalog/_template.yaml
	$(OPM) alpha render-template basic -o yaml < catalog/_template.yaml > catalog/catalog.yaml
	cd catalog ; docker build --platform $(CONTAINER_OS)/$(CONTAINER_PLATFORM) -t $(CATALOG_IMG) .

.PHONY: catalog-push
catalog-push: ## Push the OLM catalog image (for testing).
	docker push $(CATALOG_IMG)

.PHONY: catalog-install
catalog-install: config/samples/pattern-catalog-$(VERSION).yaml ## Install the OLM catalog on a cluster (for testing).
	-oc delete -f config/samples/pattern-catalog-$(VERSION).yaml
	oc create -f config/samples/pattern-catalog-$(VERSION).yaml

.PHONY: config/samples/pattern-catalog-$(VERSION).yaml
config/samples/pattern-catalog-$(VERSION).yaml:
	cp  config/samples/pattern-catalog.yaml config/samples/pattern-catalog-$(VERSION).yaml
	sed -i -e "s@CATALOG_IMG@$(CATALOG_IMG)@g" config/samples/pattern-catalog-$(VERSION).yaml

.PHONY: super-linter
super-linter: ## Runs super linter locally
	rm -rf .mypy_cache
	podman run -e RUN_LOCAL=true -e USE_FIND_ALGORITHM=true	\
					-e VALIDATE_ANSIBLE=false \
					-e VALIDATE_BASH=false \
					-e VALIDATE_CHECKOV=false \
					-e VALIDATE_DOCKERFILE_HADOLINT=false \
					-e VALIDATE_JSCPD=false \
					-e VALIDATE_JSON_PRETTIER=false \
					-e VALIDATE_MARKDOWN_PRETTIER=false \
					-e VALIDATE_PYTHON_PYLINT=false \
					-e VALIDATE_SHELL_SHFMT=false \
					-e VALIDATE_YAML=false \
					-e VALIDATE_YAML_PRETTIER=false \
					$(DISABLE_LINTERS) \
					-v $(PWD):/tmp/lint:rw,z \
					-w /tmp/lint \
					ghcr.io/super-linter/super-linter@sha256:6c71bd17ab38ceb7acb5b93ef72f5c2288b5456a5c82693ded3ee8bb501bba7f # slim-v8.1.0
