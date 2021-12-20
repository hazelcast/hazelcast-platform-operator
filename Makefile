# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= latest-snapshot

BUNDLE_VERSION := $(VERSION)
VERSION_PARTS := $(subst ., ,$(VERSION))
PATCH_VERSION := $(word 3,$(VERSION_PARTS))
ifeq (,$(PATCH_VERSION))
BUNDLE_VERSION := $(BUNDLE_VERSION).0
endif


# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "preview,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=preview,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="preview,fast,stable")
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

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# hazelcast.com/hazelcast-platform-operator-bundle:$VERSION and hazelcast.com/hazelcast-platform-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= hazelcast.com/hazelcast-platform-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= hazelcast/hazelcast-platform-operator:$(VERSION)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
# Default namespace
NAMESPACE ?= default

# Path to the kubectl command, if it is not in $PATH
KUBECTL ?= kubectl

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

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

test-all: test test-e2e

test: test-unit test-it

test-unit: manifests generate fmt vet
	go test -v ./controllers/... -coverprofile cover.out
	go test -v ./api/... -coverprofile cover.out

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
GO_TEST_FLAGS ?= "-ee=true"
test-it: manifests generate fmt vet ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test -v ./test/integration/... -coverprofile cover.out $(GO_TEST_FLAGS) -timeout 5m

test-e2e: generate fmt vet ## Run end-to-end tests
	USE_EXISTING_CLUSTER=true NAME_PREFIX=$(NAME_PREFIX) go test -v ./test/e2e -coverprofile cover.out -namespace $(NAMESPACE) -eventually-timeout 8m -timeout 30m -delete-timeout 8m $(GO_TEST_FLAGS)
	
##@ Build

GO_BUILD_TAGS ?= "localrun"
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager -tags $(GO_BUILD_TAGS) main.go

run: manifests generate fmt vet ## Run a controller from your host.
	go run -tags $(GO_BUILD_TAGS)  ./main.go

docker-build: test docker-build-ci ## Build docker image with the manager.

docker-build-ci: ## Build docker image with the manager without running tests.
	docker build -t ${IMG} --build-arg version=${VERSION} .

docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
ifneq (,$(NAME_PREFIX))
	cd config/default && $(KUSTOMIZE) edit set nameprefix $(NAME_PREFIX)
endif
	cd config/default && $(KUSTOMIZE) edit set namespace $(NAMESPACE)
	cd config/rbac && $(KUSTOMIZE) edit set namespace $(NAMESPACE)
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete -f -

undeploy-keep-crd:
	cd config/default && $(KUSTOMIZE) edit remove resource ../crd
	$(KUSTOMIZE) build config/default | kubectl delete -f -

clean-up-namespace: ## Clean up all the resources that were created by the operator for a specific kubernetes namespace
	$(eval mc := $(shell $(KUBECTL) get managementcenter -n $(NAMESPACE) -o name))
	$(eval hz := $(shell $(KUBECTL) get hazelcast -n $(NAMESPACE) -o name))
	[[ "$(hz)" != "" ]] && $(KUBECTL) delete $(hz) -n $(NAMESPACE) --wait=true --timeout=1m || echo "no hazelcast resources"
	[[ "$(mc)" != "" ]] && $(KUBECTL) delete $(mc) -n $(NAMESPACE) --wait=true --timeout=1m || echo "no managementcenter resources"
	$(KUBECTL) delete secret hazelcast-license-key -n $(NAMESPACE) --wait=false || echo "no hazelcast-license-key secret found"
	$(KUBECTL) delete pvc -l app.kubernetes.io/managed-by=hazelcast-platform-operator -n $(NAMESPACE) --wait=true --timeout=1m
	$(KUBECTL) delete svc -l app.kubernetes.io/managed-by=hazelcast-platform-operator -n $(NAMESPACE) --wait=true --timeout=4m
	$(MAKE) undeploy-keep-crd
	@if [[ -n "$($(KUBECTL) get hazelcast -n $(NAMESPACE) -o name)" ]]; then \
		$(KUBECTL) patch $(hz) -p '{"metadata":{"finalizers":null}}' --type=merge; \
	fi
	@if [[ -n "$($(KUBECTL) get managementcenter -n $(NAMESPACE) -o name)" ]]; then \
		$(KUBECTL) patch $(mc) -p '{"metadata":{"finalizers":null}}' --type=merge; \
	fi
	$(KUBECTL) delete namespace $(NAMESPACE) --wait=true --timeout 1m

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: bundle
bundle: operator-sdk manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(BUNDLE_VERSION) $(BUNDLE_METADATA_OPTS)
	sed -i  "s|containerImage: REPLACE_IMG|containerImage: $(IMG)|" bundle/manifests/hazelcast-platform-operator.clusterserviceversion.yaml
	sed -i  "s|createdAt: REPLACE_DATE|createdAt: \"$$(date +%F)T11:59:59Z\"|" bundle/manifests/hazelcast-platform-operator.clusterserviceversion.yaml
	$(OPERATOR_SDK) bundle validate ./bundle --select-optional suite=operatorframework

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$${OS}-$${ARCH}-opm ;\
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

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

generate-bundle-yaml: manifests kustomize ## Generate one file deployment bundle.yaml
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > bundle.yaml

STS_NAME ?= hazelcast
expose-local: ## Port forward hazelcast Pod so that it's accessible from localhost
	while [ true ] ; do \
		$(KUBECTL) get sts $(STS_NAME) &> /dev/null && break ; \
		sleep 5 ; \
	done;
	$(KUBECTL) wait --for=condition=ready pod $(STS_NAME)-0 --timeout=15m
	$(KUBECTL) port-forward statefulset/$(STS_NAME) 8000:5701

# Detect the OS to set per-OS defaults
OS_NAME = $(shell uname -s | tr A-Z a-z)

OPERATOR_SDK_VERSION ?= v1.13.1
OPERATOR_SDK_URL=https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$(OS_NAME)_amd64

OPERATOR_SDK=${shell pwd}/bin/operator-sdk
.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK)

.PHONY: print-bundle-version
print-bundle-version:
	@echo -n $(BUNDLE_VERSION)

$(OPERATOR_SDK):
	curl -sSL $(OPERATOR_SDK_URL) -o $(OPERATOR_SDK) --create-dirs || (echo "curl returned $$? trying to fetch operator-sdk."; exit 1)
	chmod +x $(OPERATOR_SDK)


OCP_OLM_CATALOG_VALIDATOR=${shell pwd}/bin/ocp-olm-catalog-validator
OCP_OLM_CATALOG_VALIDATOR_VERSION ?= v0.0.1
OCP_OLM_CATALOG_VALIDATOR_URL=https://github.com/redhat-openshift-ecosystem/ocp-olm-catalog-validator/releases/download/$(OCP_OLM_CATALOG_VALIDATOR_VERSION)/$(OS_NAME)-amd64-ocp-olm-catalog-validator
.PHONY: ocp-olm-catalog-validator
ocp-olm-catalog-validator: $(OCP_OLM_CATALOG_VALIDATOR)

$(OCP_OLM_CATALOG_VALIDATOR):
	curl -sSL $(OCP_OLM_CATALOG_VALIDATOR_URL) -o $(OCP_OLM_CATALOG_VALIDATOR) --create-dirs || (echo "curl returned $$? trying to fetch ocp-olm-catalog-validator."; exit 1)
	chmod +x $(OCP_OLM_CATALOG_VALIDATOR)

bundle-ocp-validate: ocp-olm-catalog-validator
	 $(OCP_OLM_CATALOG_VALIDATOR) ./bundle  --optional-values="file=./bundle/metadata/annotations.yaml"
