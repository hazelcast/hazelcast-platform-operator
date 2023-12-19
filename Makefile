# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= latest-snapshot
COV_PKG=$(shell go list ./... | grep -v /apidocgen | grep -v /test | tr '\n' ",")

BUNDLE_VERSION := $(VERSION)
VERSION_PARTS := $(subst ., ,$(VERSION))
PATCH_VERSION := $(word 3,$(VERSION_PARTS))
ifeq (,$(PATCH_VERSION))
BUNDLE_VERSION := $(BUNDLE_VERSION).0
endif


### TOOL VERSIONS
TOOLBIN = $(shell pwd)/bin
# https://github.com/kubernetes/kubernetes/releases
# Used API version is set in go.mod file
K8S_VERSION ?= 1.25.4
SETUP_ENVTEST_VERSION ?= latest
ENVTEST_K8S_VERSION ?= 1.25.x
# https://github.com/operator-framework/operator-sdk/releases
OPERATOR_SDK_VERSION ?= v1.25.2
# https://github.com/kubernetes-sigs/controller-tools/releases
CONTROLLER_GEN_VERSION ?= v0.10.0
# https://github.com/kubernetes-sigs/controller-runtime/releases
# It is set in the go.mod file
CONTROLLER_RUNTIME_VERSION ?= v0.13.1
# https://github.com/redhat-openshift-ecosystem/ocp-olm-catalog-validator/releases
OCP_OLM_CATALOG_VALIDATOR_VERSION ?= v0.0.1
# https://github.com/operator-framework/operator-registry/releases
OPM_VERSION ?= v1.26.2
# https://github.com/onsi/ginkgo/releases
# It is set in the go.mod file
GINKGO_VERSION ?= $(shell go list -m -f "{{.Version}}" github.com/onsi/ginkgo/v2)
# https://github.com/kubernetes-sigs/kustomize/releases
KUSTOMIZE_VERSION ?= v4.5.3
# https://github.com/helm/helm/releases
HELM_VERSION ?= v3.10.3
# https://github.com/mikefarah/yq/releases
YQ_VERSION ?= v4.30.7


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
IMAGE_TAG_BASE ?= hazelcast/hazelcast-platform-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd"

# If namespace is empty, override it as default
ifeq (,$(NAMESPACE))
override NAMESPACE = default
endif

ifeq (,$(WATCHED_NAMESPACES))
override WATCHED_NAMESPACES := $(NAMESPACE)
endif


# Path to the kubectl command, if it is not in $PATH
KUBECTL ?= kubectl

OPERATOR_CHART ?= ./helm-charts/hazelcast-platform-operator
CRD_CHART := $(OPERATOR_CHART)/charts/hazelcast-platform-operator-crds

PHONE_HOME_ENABLED ?= false
DEVELOPER_MODE_ENABLED ?= true
INSTALL_CRDS ?= false
DEBUG_ENABLED ?= false

RELEASE_NAME ?= v1
CRD_RELEASE_NAME ?= hazelcast-platform-operator-crds
DEPLOYMENT_NAME := $(RELEASE_NAME)-hazelcast-platform-operator
STRING_SET_VALUES := developerModeEnabled=$(DEVELOPER_MODE_ENABLED),phoneHomeEnabled=$(PHONE_HOME_ENABLED),installCRDs=$(INSTALL_CRDS),image.imageOverride=$(IMG),watchedNamespaces='{$(WATCHED_NAMESPACES)}',debug.enabled=$(DEBUG_ENABLED)

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
	@$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	@$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet -tags "$(GO_BUILD_TAGS)" ./...

test-all: test test-e2e

test: test-unit test-it

test-unit: GO_BUILD_TAGS = "hazelcastinternal,unittest"
test-unit: manifests generate fmt vet
	PHONE_HOME_ENABLED=$(PHONE_HOME_ENABLED) DEVELOPER_MODE_ENABLED=$(DEVELOPER_MODE_ENABLED) go test -tags $(GO_BUILD_TAGS) -v ./controllers/... -coverprofile=cover-controllers.out -coverpkg $(COV_PKG)
	PHONE_HOME_ENABLED=$(PHONE_HOME_ENABLED) DEVELOPER_MODE_ENABLED=$(DEVELOPER_MODE_ENABLED) go test -tags $(GO_BUILD_TAGS) -v ./internal/... -coverprofile=cover-internal.out -coverpkg $(COV_PKG)
	PHONE_HOME_ENABLED=$(PHONE_HOME_ENABLED) DEVELOPER_MODE_ENABLED=$(DEVELOPER_MODE_ENABLED) go test -tags $(GO_BUILD_TAGS) -v ./api/... -coverprofile=cover-api.out -coverpkg $(COV_PKG)

lint: lint-go lint-yaml

LINTER_SETUP_DIR=$(shell pwd)/lintbin
LINTER_PATH="${LINTER_SETUP_DIR}/bin:${PATH}"
lint-go: setup-linters
	PATH=${LINTER_PATH} golangci-lint run --build-tags $(GO_BUILD_TAGS)

lint-yaml: setup-linters
	PATH=${LINTER_PATH} yamllint -c ./hack/yamllint.yaml .

setup-linters:
	source hack/setup-linters.sh; get_linters ${LINTER_SETUP_DIR}

# Use tilt tool to deploy operator and its resources to the local K8s cluster in the current context 
tilt: 
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) tilt up

tilt-debug:
	DEBUG_ENABLED=true tilt up

tilt-debug-remote-ttl:
	DEBUG_ENABLED=true ALLOW_REMOTE=true USE_TTL_REG=true tilt up

# Use tilt tool to deploy operator and its resources to any K8s cluster in the current context 
tilt-remote: 
	 DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) ALLOW_REMOTE=true tilt up

# Use tilt tool to deploy operator and its resources to any K8s cluster in the current context with ttl.sh configured for image registry.
tilt-remote-ttl:
	 DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) ALLOW_REMOTE=true USE_TTL_REG=true tilt up

ENVTEST_ASSETS_DIR=$(TOOLBIN)/envtest
GO_TEST_FLAGS ?= "-ee=true"
COVER_OUT ?= "cover.out"

test-it: manifests generate fmt vet envtest ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_ASSETS_DIR) -p path)" PHONE_HOME_ENABLED=$(PHONE_HOME_ENABLED) DEVELOPER_MODE_ENABLED=$(DEVELOPER_MODE_ENABLED) go test -tags $(GO_BUILD_TAGS) -v ./test/integration/... -ginkgo.label-filter="slow || fast" -coverprofile $(COVER_OUT) -coverpkg $(COV_PKG) $(GO_TEST_FLAGS) -eventually-timeout 30s -timeout 5m

test-it-focus: manifests generate fmt vet envtest ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_ASSETS_DIR) -p path)" PHONE_HOME_ENABLED=$(PHONE_HOME_ENABLED) DEVELOPER_MODE_ENABLED=$(DEVELOPER_MODE_ENABLED) go test -tags $(GO_BUILD_TAGS) -v ./test/integration/... -coverprofile $(COVER_OUT) $(GO_TEST_FLAGS) -eventually-timeout 30s -timeout 5m

E2E_TEST_SUITE ?= hz || mc || hz_persistence || hz_expose_externally || map || map_persistence || cache_persistence || hz_wan || custom_class || multimap || topic || replicatedmap || queue || cache || resilience || backup || jetjob || jetjobsnapshot
ifeq (,$(E2E_TEST_SUITE))
E2E_TEST_LABELS =
else 
E2E_TEST_LABELS = && $(E2E_TEST_SUITE)
endif
GINKGO_PARALLEL_PROCESSES ?= 4
GINKGO_KIND_PARALLEL_PROCESSES ?= 2

test-e2e-split-kind: generate fmt vet ginkgo ## Run end-to-end tests on Kind
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) $(GINKGO) -r --compilers=2 --keep-going --junit-report=test_report_$(REPORT_SUFFIX).xml --output-dir=allure-results/$(WORKFLOW_ID) --procs $(GINKGO_KIND_PARALLEL_PROCESSES) --flake-attempts 2 --trace --slow-spec-threshold=100s --tags $(GO_BUILD_TAGS) $(FOCUSED_TESTS) --vv --progress --timeout 70m --coverprofile cover.out ./test/e2e -- -namespace "$(NAMESPACE)" -hazelcast-version "$(HZ_VERSION)" -mc-version "$(MC_VERSION)" $(GO_TEST_FLAGS)

test-e2e: generate fmt vet ginkgo ## Run end-to-end tests
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) $(GINKGO) -r --keep-going --junit-report=test_report_$(REPORT_SUFFIX).xml --output-dir=allure-results/$(WORKFLOW_ID) --procs $(GINKGO_PARALLEL_PROCESSES) --trace --label-filter="(slow || fast) $(E2E_TEST_LABELS)" --slow-spec-threshold=100s --tags $(GO_BUILD_TAGS) --vv --progress --timeout 70m --flake-attempts 2 --output-interceptor-mode=none --coverprofile cover.out ./test/e2e -- -namespace "$(NAMESPACE)" -deployNamespace "$(WATCHED_NAMESPACES)" -hazelcast-version "$(HZ_VERSION)" -mc-version "$(MC_VERSION)" $(GO_TEST_FLAGS)

test-ph: generate fmt vet ginkgo ## Run phone-home tests
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) $(GINKGO) -r --keep-going --junit-report=test_report_$(REPORT_SUFFIX).xml --output-dir=allure-results/$(WORKFLOW_ID) --trace --slow-spec-threshold=100s --tags $(GO_BUILD_TAGS) --vv --progress --timeout 40m --output-interceptor-mode=none --coverprofile cover.out ./test/ph -- -namespace "$(NAMESPACE)" -hazelcast-version "$(HZ_VERSION)" -mc-version "$(MC_VERSION)" -eventually-timeout 8m  -delete-timeout 8m $(GO_TEST_FLAGS)

test-high-availability: generate fmt vet ginkgo ## Run high-availability tests
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) $(GINKGO) -r --keep-going --junit-report=test_report_$(REPORT_SUFFIX).xml --output-dir=allure-results/$(WORKFLOW_ID) --procs 1 --trace --label-filter="high_availability" --slow-spec-threshold=100s --tags $(GO_BUILD_TAGS) --vv --progress --timeout 70m --flake-attempts 2 --output-interceptor-mode=none --coverprofile cover.out ./test/e2e -- -namespace "$(NAMESPACE)" -hazelcast-version "$(HZ_VERSION)" -mc-version "$(MC_VERSION)" $(GO_TEST_FLAGS)

test-high-load: generate fmt vet ginkgo ## Run high-load tests
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) $(GINKGO) -r --keep-going --junit-report=test_report_$(REPORT_SUFFIX).xml --output-dir=allure-results/$(WORKFLOW_ID) --procs 1 --trace --label-filter="high_load" --slow-spec-threshold=100s --tags $(GO_BUILD_TAGS) --vv --progress --timeout 70m --flake-attempts 1 --output-interceptor-mode=none --coverprofile cover.out ./test/e2e -- -namespace "$(NAMESPACE)" -hazelcast-version "$(HZ_VERSION)" -mc-version "$(MC_VERSION)" $(GO_TEST_FLAGS)

test-soak: generate fmt vet ginkgo ## Run soak tests
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) $(GINKGO) -r --keep-going --junit-report=test_report_$(REPORT_SUFFIX).xml --output-dir=allure-results/$(WORKFLOW_ID) --procs 1 --trace --label-filter="soak" --slow-spec-threshold=100s --tags $(GO_BUILD_TAGS) --vv --progress --timeout 1500m --flake-attempts 1 --output-interceptor-mode=none --coverprofile cover.out ./test/e2e -- -namespace "$(NAMESPACE)" -hazelcast-version "$(HZ_VERSION)" -mc-version "$(MC_VERSION)" $(GO_TEST_FLAGS)

test-e2e-focus: generate fmt vet ginkgo ## Run focused end-to-end tests
	DEPLOYMENT_NAME=$(DEPLOYMENT_NAME) $(GINKGO) --trace --slow-spec-threshold=100s --tags $(GO_BUILD_TAGS) -v --progress --timeout 70m --coverprofile cover.out ./test/e2e -- -namespace "$(NAMESPACE)" -hazelcast-version "$(HZ_VERSION)" -mc-version "$(MC_VERSION)" $(GO_TEST_FLAGS)

##@ Build
GO_BUILD_TAGS = hazelcastinternal
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager -tags "$(GO_BUILD_TAGS)" main.go

build-tilt: generate fmt vet # This is not going to work if client and server cpu architectures are different
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "$(GO_BUILD_TAGS)" -ldflags "-s -w" -o bin/tilt/manager main.go

build-tilt-debug: generate fmt vet # This is not going to work if client and server cpu architectures are different
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "$(GO_BUILD_TAGS)" -gcflags "-N -l" -o bin/tilt/manager-debug main.go

run: manifests generate fmt vet ## Run a controller from your host.
	PHONE_HOME_ENABLED=$(PHONE_HOME_ENABLED) DEVELOPER_MODE_ENABLED=$(DEVELOPER_MODE_ENABLED) go run -tags "$(GO_BUILD_TAGS)" ./main.go

docker-build: test docker-build-ci ## Build docker image with the manager.

PARDOT_ID ?= "dockerhub"
docker-build-ci: ## Build docker image with the manager without running tests.
	DOCKER_BUILDKIT=1 docker build -t ${IMG} --build-arg version=${VERSION} --build-arg pardotID=${PARDOT_ID} .

##@ Deployment
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

docker-push-latest:
	docker tag ${IMG} ${IMAGE_TAG_BASE}:latest
	docker push ${IMAGE_TAG_BASE}:latest

sync-manifests: manifests yq
# Move CRDs into helm template
	@cat config/crd/bases/* >> all-crds.yaml && mv all-crds.yaml $(CRD_CHART)/templates/
# Role and ClusterRole syncing is done manually

install-crds: helm sync-manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config. NOTE: 'default' namespace is used for the CRD chart release since we are checking if the CRDs is installed before, then we are skipping CRDs installation. To be able to achieve this, we need static CRD_RELEASE_NAME and namespace
	$(HELM) template $(CRD_RELEASE_NAME) $(CRD_CHART) | $(KUBECTL) apply -f -

install-operator: helm sync-manifests
	$(HELM) upgrade --install $(RELEASE_NAME) $(OPERATOR_CHART) --set $(STRING_SET_VALUES) -n $(NAMESPACE)

uninstall-crds: helm sync-manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(HELM) template $(CRD_RELEASE_NAME) $(CRD_CHART) | $(KUBECTL) delete -f -

uninstall-operator: helm sync-manifests
	$(HELM) uninstall $(RELEASE_NAME) -n $(NAMESPACE)

webhook-install: helm sync-manifests
	$(HELM) template $(RELEASE_NAME) $(OPERATOR_CHART) -s templates/webhook.yaml --namespace=$(NAMESPACE) | $(KUBECTL) apply -f -

webhook-uninstall: helm sync-manifests
	$(HELM) template $(RELEASE_NAME) $(OPERATOR_CHART) -s templates/webhook.yaml --namespace=$(NAMESPACE) | $(KUBECTL) delete -f -

deploy: install-crds install-operator ## Deploy controller to the K8s cluster specified in ~/.kube/config.

undeploy: uninstall-operator uninstall-crds ## Undeploy controller from the K8s cluster specified in ~/.kube/config.

deploy-tilt: helm sync-manifests generate
	@$(HELM) template $(RELEASE_NAME) $(OPERATOR_CHART) --set $(STRING_SET_VALUES),podSecurityContext=null,securityContext=null --namespace=$(NAMESPACE)

undeploy-tilt: 
	$(MAKE) -s deploy-tilt RELEASE_NAME=$(RELEASE_NAME) OPERATOR_CHART=$(OPERATOR_CHART) NAMESPACE=$(NAMESPACE) | $(KUBECTL) delete -f -

undeploy-keep-crd: uninstall-operator

clean-up-namespace: ## Clean up all the resources that were created by the operator for a specific kubernetes namespace
	$(eval CR_NAMES := $(shell $(KUBECTL) get crd -o jsonpath='{range.items[*]}{..metadata.name}{"\n"}{end}' | grep hazelcast.com))
	for CR_NAME in $(CR_NAMES); do \
		crs=$$($(KUBECTL) get $${CR_NAME} -n $(NAMESPACE) -o name); \
		[[ "$${crs}" != "" ]] && $(KUBECTL) delete $${crs} -n $(NAMESPACE) --wait=true --timeout=30s || echo "no $${CR_NAME} resources" ;\
	done 
	$(KUBECTL) delete secret hazelcast-license-key -n $(NAMESPACE) --wait=false || echo "no hazelcast-license-key secret found"
	$(MAKE) undeploy-keep-crd

	for CR_NAME in $(CR_NAMES); do \
		crs=$$($(KUBECTL) get $${CR_NAME} -n $(NAMESPACE) -o name); \
		[[ "$${crs}" != "" ]] && $(KUBECTL) patch $${crs} -n $(NAMESPACE) -p '{"metadata":{"finalizers":null}}' --type=merge || echo "$${CR_NAME} already deleted";\
	done 

	$(KUBECTL) delete pvc -l app.kubernetes.io/managed-by=hazelcast-platform-operator -n $(NAMESPACE) --wait=true --timeout=1m
	$(KUBECTL) delete svc -l app.kubernetes.io/managed-by=hazelcast-platform-operator -n $(NAMESPACE) --wait=true --timeout=8m
	$(KUBECTL) delete namespace $(NAMESPACE) --grace-period=0 --force

.PHONY: bundle
bundle: operator-sdk manifests kustomize yq ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	($(YQ) 'select(.kind == "ClusterRole")  | .' config/rbac/role.yaml && \
	 echo "---" && \
	 $(YQ)  eval-all '. | select(.kind == "Role" ) | . as $$item ireduce ({}; . *+ $$item) '  config/rbac/role.yaml) > config/rbac/role.yaml.new && mv config/rbac/role.yaml.new config/rbac/role.yaml
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --use-image-digests --overwrite --version $(BUNDLE_VERSION) $(BUNDLE_METADATA_OPTS)
	$(MAKE) manifests # Revert changes done for generating bundle
	sed -i  "s|containerImage: REPLACE_IMG|containerImage: $(IMG)|" bundle/manifests/hazelcast-platform-operator.clusterserviceversion.yaml
	sed -i  "s|createdAt: REPLACE_DATE|createdAt: \"$$(date +%F)T11:59:59Z\"|" bundle/manifests/hazelcast-platform-operator.clusterserviceversion.yaml
	$(OPERATOR_SDK) bundle validate ./bundle --select-optional suite=operatorframework

olm-deploy: operator-sdk ## Deploying Operator with OLM bundle. Available modes are AllNamespace|OwnNamespace|SingleNamespace
	@$(eval CONTAINER_IMAGE=ttl.sh/$(shell uuidgen | tr "[:upper:]" "[:lower:]"):4h)
	@$(eval BUNDLE_IMAGE=ttl.sh/$(shell uuidgen | tr "[:upper:]" "[:lower:]"):4h)
	@$(eval VERSION=1.0.0)
	$(MAKE) docker-build-ci IMG=$(CONTAINER_IMAGE) VERSION=$(VERSION)
	$(MAKE) docker-push IMG=$(CONTAINER_IMAGE)
	$(MAKE) bundle IMG=${CONTAINER_IMAGE} VERSION=$(VERSION)
	@printf "  com.redhat.openshift.versions: v4.8\n  operators.operatorframework.io.bundle.channel.default.v1: alpha" >> ./bundle/metadata/annotations.yaml
	docker build -f bundle.Dockerfile -t ${BUNDLE_IMAGE} .
	docker push ${BUNDLE_IMAGE}
	$(KUBECTL) create namespace $(NS)
	operator-sdk run bundle ${BUNDLE_IMAGE} --namespace=$(NS) --timeout=10m --install-mode=$(MODE)

cleanup-olm: operator-sdk ## Clean up an Operator deployed with OLM
	operator-sdk cleanup hazelcast-platform-operator --namespace=$(NS)
	$(KUBECTL) delete namespace $(NS) --wait=true --timeout 5m

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	DOCKER_BUILDKIT=1 docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

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

.PHONY: print-bundle-version 
print-bundle-version: 
	@echo -n $(BUNDLE_VERSION)

bundle-ocp-validate: ocp-olm-catalog-validator
	 $(OCP_OLM_CATALOG_VALIDATOR) ./bundle  --optional-values="file=./bundle/metadata/annotations.yaml"

api-ref-doc: 
	@go build -o bin/docgen  ./apidocgen/main.go 
	@./bin/docgen ./api/v1alpha1/*.go


##@ Tool installation

OS=$(shell go env GOOS)
ARCH=$(shell go env GOARCH)

.PHONY: print
print:
	@print #empty command
	$(eval PRINT_TOOL_NAME=true)

ENVTEST = $(TOOLBIN)/setup-envtest/$(SETUP_ENVTEST_VERSION)/setup-envtest
.PHONY: envtest
envtest: ## Download setup-envtest locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION))
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(ENVTEST); fi

OPERATOR_SDK_URL=https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$(OS)_$(ARCH)
OPERATOR_SDK=${TOOLBIN}/operator-sdk/$(OPERATOR_SDK_VERSION)/operator-sdk
.PHONY: operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
	@[ -f $(OPERATOR_SDK) ] || { \
		curl -sSL $(OPERATOR_SDK_URL) -o $(OPERATOR_SDK) --create-dirs ;\
		chmod +x $(OPERATOR_SDK);\
	}
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(OPERATOR_SDK); fi

CONTROLLER_GEN = $(TOOLBIN)/controller-gen/$(CONTROLLER_GEN_VERSION)/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION))
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(CONTROLLER_GEN); fi

KUSTOMIZE = $(TOOLBIN)/kustomize/$(KUSTOMIZE_VERSION)/kustomize
.PHONY: kustomize
kustomize: ## Download kustomize locally if necessary.
	@$(eval KUSTOMIZE_MAJOR_VERSION=$(firstword $(subst ., ,$(KUSTOMIZE_VERSION))))
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/$(KUSTOMIZE_MAJOR_VERSION)@$(KUSTOMIZE_VERSION))
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(KUSTOMIZE); fi

GINKGO = $(TOOLBIN)/ginkgo/$(GINKGO_VERSION)/ginkgo
.PHONY: ginkgo
ginkgo: ## Download ginkgo locally if necessary.
	@$(eval GINKGO_MAJOR_VERSION=$(firstword $(subst ., ,$(GINKGO_VERSION)))) 
	@[ -f $(GINKGO) ] || { \
	mkdir -p $(dir $(GINKGO)) ;\
	go get github.com/onsi/ginkgo/$(GINKGO_MAJOR_VERSION)@$(GINKGO_VERSION) ;\
	GOBIN=$(dir $(GINKGO)) go install -mod=mod github.com/onsi/ginkgo/$(GINKGO_MAJOR_VERSION)/ginkgo@$(GINKGO_VERSION) ;\
	}
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(GINKGO); fi

OPM = $(TOOLBIN)/opm/$(OPM_VERSION)/opm
.PHONY: opm
opm: ## Download opm locally if necessary.
	@[ -f $(OPM) ] || { \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$(OS)-$(ARCH)-opm --create-dirs ;\
	chmod +x $(OPM) ;\
	}
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(OPM); fi

OCP_OLM_CATALOG_VALIDATOR_URL=https://github.com/redhat-openshift-ecosystem/ocp-olm-catalog-validator/releases/download/$(OCP_OLM_CATALOG_VALIDATOR_VERSION)/$(OS)-$(ARCH)-ocp-olm-catalog-validator
OCP_OLM_CATALOG_VALIDATOR=$(TOOLBIN)/ocp-olm-catalog-validator/$(OCP_OLM_CATALOG_VALIDATOR_VERSION)/ocp-olm-catalog-validator
.PHONY: ocp-olm-catalog-validator
ocp-olm-catalog-validator: ## Download ocp-olm-catalog-validator locally if necessary.
	@[ -f $(OCP_OLM_CATALOG_VALIDATOR) ] || { \
	curl -sSL $(OCP_OLM_CATALOG_VALIDATOR_URL) -o $(OCP_OLM_CATALOG_VALIDATOR) --create-dirs ;\
	chmod +x $(OCP_OLM_CATALOG_VALIDATOR) ;\
	}
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(OCP_OLM_CATALOG_VALIDATOR); fi

YQ=${TOOLBIN}/yq/$(YQ_VERSION)/yq
.PHONY: yq
yq: ## Download yq locally if necessary.
	@[ -f $(YQ) ] || { \
		curl -sSL https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_$(OS)_$(ARCH) -o $(YQ) --create-dirs ;\
		chmod +x $(YQ);\
	}
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(YQ); fi


HELM = $(TOOLBIN)/helm/$(HELM_VERSION)/helm
.PHONY: helm
helm: ## Download helm locally if necessary.
	@[ -f $(HELM) ] || { \
	mkdir -p $(dir $(HELM)) ;\
	TMP_DIR=$$(mktemp -d) ;\
	curl -sSLo $${TMP_DIR}/temp.tar.gz https://get.helm.sh/helm-$(HELM_VERSION)-$(OS)-$(ARCH).tar.gz  &>/dev/null;\
	tar --directory $${TMP_DIR} -zxvf $${TMP_DIR}/temp.tar.gz &>/dev/null;\
	mv $${TMP_DIR}/$(OS)-$(ARCH)/helm $(HELM);\
	rm -rf $${TMP_DIR};\
	chmod +x $(HELM);\
	}
	@if [ "$(PRINT_TOOL_NAME)" == "true" ]; then echo -n $(HELM); fi

# go-get-tool will 'go install' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp &> /dev/null;\
mkdir -p $(dir $(1)) ;\
GOBIN=$(dir $(1)) go install $(2) &> /dev/null ;\
rm -rf $$TMP_DIR ;\
}
endef
