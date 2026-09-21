# Image URL to use for local image-building targets.
IMG ?= $(shell basename "$$(pwd)")

PLATFORM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
ARCH := amd64
else ifeq ($(ARCH),arm64)
ARCH := arm64
endif

APPNAME := manager
GO_VERSION := $(shell awk '/^go / { print $$2; exit }' go.mod)
DOCKER := docker
SRC_FOLDER := .
KUBECONFIG_PATH ?= $(HOME)/.kube/config

.PHONY: all help fmt vet lint test build run example docker-build docker-build-local docker-run docker-push

all: test

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

fmt: ## Run gofmt against tracked Go files.
	NO_PROXY=* GOPROXY=off GOSUMDB=off gofmt -w $$(git ls-files -z '*.go' | xargs -0)

vet: ## Run go vet against code.
	NO_PROXY=* GOPROXY=off GOSUMDB=off go vet ./...

lint: ## Run the local golangci-lint binary.
	@test -x ./bin/golangci-lint || { echo "missing local golangci-lint" >&2; exit 1; }
	NO_PROXY=* GOPROXY=off GOSUMDB=off ./bin/golangci-lint run

test: ## Run offline Go tests.
	NO_PROXY=* GOPROXY=off GOSUMDB=off go test ./...

build: ## Build all Go packages.
	NO_PROXY=* GOPROXY=off GOSUMDB=off go build ./...

example: ## Build and run the vcflag example.
	NO_PROXY=* GOPROXY=off GOSUMDB=off go build ./examples/vcflag
	NO_PROXY=* GOPROXY=off GOSUMDB=off go run ./examples/vcflag --help

run: ## Run a controller from your host.
	NO_PROXY=* GOPROXY=off GOSUMDB=off go run ./main.go

docker-build: ## Build a local Docker image when Dockerfile is present.
	@test -f Dockerfile || { echo "Dockerfile is required; Docker build disabled" >&2; exit 1; }
	@echo "Building image ${IMG}:latest"
	$(DOCKER) build --target production -t ${IMG}:latest -f Dockerfile --build-arg GO_VERSION=${GO_VERSION} .

docker-build-local: ## Build a local image with an existing kubeconfig path.
	@test -f "${KUBECONFIG_PATH}" || { echo "KUBECONFIG_PATH must reference an existing file" >&2; exit 1; }
	@test -f Dockerfile || { echo "Dockerfile is required; Docker build disabled" >&2; exit 1; }
	@echo "Building image ${IMG}:latest"
	$(DOCKER) build --target production -t ${IMG}:latest -f Dockerfile --build-arg GO_VERSION=${GO_VERSION} .

docker-run: ## Run the built Docker image.
	@test -f ./config.test.yaml || cp ./config.test.yaml.sample ./config.test.yaml
	$(DOCKER) run -v "$$(pwd)/config.test.yaml:/opt/workspace/configs/config.yaml" -p 3030:3030 ${IMG}:latest

docker-push: ## Docker publishing is disabled.
	@echo "Docker publishing is disabled; no destination configured"; exit 1
