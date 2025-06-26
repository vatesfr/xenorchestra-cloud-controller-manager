REGISTRY ?= ghcr.io
USERNAME ?= vatesfr
PROJECT ?= xenorchestra-cloud-controller-manager
IMAGE ?= $(REGISTRY)/$(USERNAME)/$(PROJECT)
HELMREPO ?= $(REGISTRY)/$(USERNAME)/charts
PLATFORM ?= linux/amd64
PUSH ?= false

VERSION ?= $(shell git describe --dirty --tag --match='v*')
SHA ?= $(shell git describe --match=none --always --abbrev=7 --dirty)
TAG ?= $(VERSION)

GO_LDFLAGS := -s -w
GO_LDFLAGS += -X k8s.io/component-base/version.gitVersion=$(VERSION)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
ARCHS = amd64

TESTARGS ?= "-v"

BUILD_ARGS := --platform=$(PLATFORM)
ifeq ($(PUSH),true)
BUILD_ARGS += --push=$(PUSH)
BUILD_ARGS += --output type=image,annotation-index.org.opencontainers.image.source="https://github.com/$(USERNAME)/$(PROJECT)",annotation-index.org.opencontainers.image.description="Xen Orchestra CCM for Kubernetes"
else
BUILD_ARGS += --output type=docker
endif

COSING_ARGS ?=

############

# Help Menu

define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- golang 1.20+
- golangci-lint

endef

export HELP_MENU_HEADER

help: ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z0-9%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

############
#
# Build Abstractions
#

build-all-archs:
	@for arch in $(ARCHS); do $(MAKE) ARCH=$${arch} build ; done

.PHONY: clean
clean: ## Clean
	rm -rf bin

.PHONY: build
build: ## Build
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(GO_LDFLAGS)" \
		-o bin/xenorchestra-cloud-controller-manager-$(ARCH) ./cmd/xenorchestra-cloud-controller-manager

.PHONY: run
run: build ## Run
	./bin/xenorchestra-cloud-controller-manager-$(ARCH) --v=5 --kubeconfig=$(HOME)/.kube/config --cloud-config=xo-config.yaml --controllers=cloud-node,cloud-node-lifecycle \
		--use-service-account-credentials --leader-elect=false --bind-address=127.0.0.1 --authorization-always-allow-paths=/healthz,/livez,/readyz,/metrics

.PHONY: lint
lint: ## Lint Code
	golangci-lint run --config .golangci.yml

.PHONY: unit
unit: ## Unit Tests
	go test -tags=unit $(shell go list ./...) $(TESTARGS)

############

.PHONY: helm-lint
helm-lint:
	@ct --config hack/ct.yml lint --check-version-increment=false

.PHONY: helm-unit
helm-unit: ## Helm Unit Tests
	@helm lint charts/xenorchestra-cloud-controller-manager
	@helm template -f charts/xenorchestra-cloud-controller-manager/ci/values.yaml \
		xenorchestra-cloud-controller-manager charts/xenorchestra-cloud-controller-manager >/dev/null

.PHONY: helm-login
helm-login: ## Helm Login
	@echo "${HELM_TOKEN}" | helm registry login $(REGISTRY) --username $(USERNAME) --password-stdin

.PHONY: helm-release
helm-release: ## Helm Release
	@rm -rf dist/
	@helm package charts/xenorchestra-cloud-controller-manager -d dist
	@helm push dist/xenorchestra-cloud-controller-manager-*.tgz oci://$(HELMREPO) 2>&1 | tee dist/.digest
	@cosign sign --yes $(COSING_ARGS) $(HELMREPO)/xenorchestra-cloud-controller-manager@$$(cat dist/.digest | awk -F "[, ]+" '/Digest/{print $$NF}')

############

.PHONY: docs
docs:
	yq -i '.appVersion = "$(TAG)"' charts/xenorchestra-cloud-controller-manager/Chart.yaml -y
	helm template -n kube-system xenorchestra-cloud-controller-manager \
		--set-string image.tag=$(TAG) \
		charts/xenorchestra-cloud-controller-manager > docs/deploy/cloud-controller-manager.yml
	helm-docs --sort-values-order=file charts/xenorchestra-cloud-controller-manager

release-update:
	git-chglog --config hack/chglog-config.yml -o CHANGELOG.md

############
#
# Docker Abstractions
#

docker-init:
	@docker run --rm --privileged multiarch/qemu-user-static -p yes ||:

	@docker context create multiarch ||:
	@docker buildx create --name multiarch --driver docker-container --use ||:
	@docker context use multiarch
	@docker buildx inspect --bootstrap multiarch

.PHONY: images
images: ## Build images
	docker buildx build $(BUILD_ARGS) \
		--build-arg VERSION="$(VERSION)" \
		--build-arg TAG="$(TAG)" \
		--build-arg SHA="$(SHA)" \
		-t $(IMAGE):$(TAG) \
		-f Dockerfile .

.PHONY: images-checks
images-checks: images
	trivy image --exit-code 1 --ignore-unfixed --severity HIGH,CRITICAL --no-progress $(IMAGE):$(TAG)

.PHONY: images-cosign
images-cosign:
	@cosign sign --yes $(COSING_ARGS) --recursive $(IMAGE):$(TAG)

.PHONY: mock
mock:
	mockgen -destination=pkg/xenorchestra/mocks/mock_library.go -package=mocks github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library Library,VM