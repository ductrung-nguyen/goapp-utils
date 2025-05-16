
# Image URL to use all building/pushing image targets
IMG ?= $(shell basename `pwd`)

PLATFORM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)

ifeq ($(ARCH),x86_64)
	ARCH := amd64
else ifeq ($(ARCH),arm64)
	ARCH := arm64
endif

APPNAME := manager
GO_VERSION := $(shell cat src/go.mod | grep go | head -n1 | cut -d" " -f2)
DOCKER := docker

GIT_USERNAME := ""
GIT_PASSWORD := $(shell python3 -c "import urllib.parse;print(urllib.parse.quote('$(cat ~/.winpass)'))")

# tag of the docker image
TAG := $(shell date --iso=seconds)

SRC_FOLDER=.
CHART_FOLDER=deployment

# ginkgo version
GINKGO_VERSION := $(shell cat ${SRC_FOLDER}/go.mod | grep ginkgo/v2 | cut -d" " -f2)
INSTALLED_GINKGO_VERSION =  $(shell (ginkgo version | cut -d ' ' -f3) || '')


# Registry
REGISTRY := dockerhub.rnd.amadeus.net:5002/splunk/app-controller
GOLANG_CI_LINT_VERSION := 2.1.6
INSTALLED_GOLANG_CI_LINT_VERSION =  $(shell (cd ${SRC_FOLDER} && ./bin/golangci-lint --version | grep -o "version \S*" | cut -d" " -f2) || '')

KUBECONFORM_VERSION := 0.6.6
INSTALLED_KUBECONFORM_VERSION =  $(shell (kubeconform -v) || '')

.PHONY: all $(DIRS)
all: test

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Initialize env
.PHONY: install-go
install-go: ## Install go automatically. Might need sudo permission
	command -v go >/dev/null 2>&1 && ( echo >&2 "Go is installed. No need to install again"; exit 0; ) || \
	( echo >&2 "Installing go. Need sudo permission" && \
		curl -Lo go_installer https://get.golang.org/linux && \
		chmod +x go_installer && \
		sudo ./go_installer -version ${GO_VERSION} && rm go_installer )

##@ Install ginkgo
.PHONY: install-ginkgo
install-ginkgo:
	command -v ginkgo >/dev/null 2>&1 && echo "No need to install tesing tools again" || ( \
		cd ${SRC_FOLDER} && \
		go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@${GINKGO_VERSION}; \
		go get github.com/onsi/gomega/...; \
	)

	[ "v${INSTALLED_GINKGO_VERSION}" = "${GINKGO_VERSION}" ] || (cd ${SRC_FOLDER} && go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@${GINKGO_VERSION})


##@ Install kubeconform
.PHONY: install-kubeconform
install-kubeconform:
	[ "v${KUBECONFORM_VERSION}" = "${INSTALLED_KUBECONFORM_VERSION}" ] || ( \
			cd /tmp && curl -sL https://github.com/yannh/kubeconform/releases/download/v${KUBECONFORM_VERSION}/kubeconform-${PLATFORM}-${ARCH}.tar.gz -o kubeconform.tar.gz && \
    	tar -xvzf kubeconform.tar.gz && \
		sudo mv kubeconform /usr/local/bin/kubeconform && \
		sudo chmod +x /usr/local/bin/kubeconform \
	)

##@ Install golangci-lint
install-golang-cli:
	[ "${GOLANG_CI_LINT_VERSION}" = "${INSTALLED_GOLANG_CI_LINT_VERSION}" ] || (cd ${SRC_FOLDER} && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v${GOLANG_CI_LINT_VERSION})
	echo "Please make sure to put $$HOME/go/bin in your PATH"

##@ Install testing tools
.PHONY: install-testing-tools
install-testing-tools: install-ginkgo install-kubeconform install-golang-cli

.PHONY: init
## initialize the working environment
init: install-go install-testing-tools install-golang-cli
	cd ${SRC_FOLDER} && \
	go mod download && \
	go mod tidy


##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	cd ${SRC_FOLDER} && \
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	cd ${SRC_FOLDER} && \
	go vet ./...

.PHONY: lint
lint: ## Run go vet against code.
	cd ${SRC_FOLDER} && \
	./bin/golangci-lint run

.PHONY: test-chart
test-chart: install-kubeconform ## Run tests for helm chart
	@[ -d "${CHART_FOLDER}" ] && chart_folders=$$(find ${CHART_FOLDER} -name Chart.yaml -exec dirname {} \;); \
	for chart_folder in $$chart_folders; do \
		cd $$chart_folder && \
		helm dependencies build && helm dependency update && helm lint . && \
		helm template . | kubeconform --strict -ignore-missing-schemas -summary && \
		helm unittest . --color; \
	done

.PHONY: accept-changes-chart
accept-changes-chart: ## Approve the changes in the unit tests of the helm chart
	cd ${CHART_FOLDER}/chart && \
	helm unittest -u . --color

.PHONY: test-application
test-application: ## Run tests for the application
	cd ${SRC_FOLDER} && \
	ginkgo --json-report ./ginkgo.report -r --race --randomize-all -coverprofile=coverage.out --junit-report=report.xml && \
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test
test: init fmt vet lint test-application test-chart

##@ Build

.PHONY: build
build: fmt vet lint ## Build binary.
	cd ${SRC_FOLDER} && \
	go build -o bin/${APPNAME}

.PHONY: run
run: fmt vet lint ## Run a controller from your host.
	cd ${SRC_FOLDER} && \
	go run ./main.go

## Build docker image with the manager.
## This image will be used on the cluster
.PHONY: docker-build
docker-build: # Build a docker image to upload to the artifactory and run inside the cluster
echo "Building image ${IMG}:latest"
	$(DOCKER) build \
		--target production \
		-t ${IMG}:latest \
		-f Dockerfile \
		--build-arg GIT_USERNAME=${GIT_USERNAME} \
  		--build-arg GIT_PASSWORD=`python3 -c "import urllib.parse;print(urllib.parse.quote('${GIT_PASSWORD}'))"` \
		--build-arg GO_VERSION=${GO_VERSION} \
		.

.PHONY: docker-build-local
## build the LOCAL container image with your current KUBECONFIG (assume that it is '~/.kube/config')
## This image will be used in the local machine for developent purpose
docker-build-local: # Build a docker image to use the controller on the local machine
	echo "Building image ${IMG}:latest"
	$(DOCKER) build \
		--build-arg KUBECONFIG="$$(cat ~/.kube/config)"  \
		--target "production" \
		-t ${IMG}:latest \
		-f Dockerfile \
		--build-arg GIT_USERNAME=${GIT_USERNAME} \
  		--build-arg GIT_PASSWORD=`python3 -c "import urllib.parse;print(urllib.parse.quote('${GIT_PASSWORD}'))"` \
		--build-arg GO_VERSION=${GO_VERSION} \
		.

# this target need to run with: "make -j2" to execute two child targets in parallel
.PHONY: docker-run
docker-run: ## Run the built docker image
	[ -f "./config.test.yaml" ] || cp ./config.test.yaml.sample ./config.test.yaml
	$(DOCKER) run \
		-v `pwd`/config.test.yaml:/opt/workspace/configs/config.yaml \
		--env KUBECONFIG="/home/nonroot/.kube_config"  \
		-p 3030:3030 \
		${IMG}:latest

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(DOCKER) tag ${IMG}:latest ${REGISTRY}/${IMG}:${TAG}
	$(DOCKER) push ${REGISTRY}/${IMG}:${TAG}
	echo "Pushed docker image to: ${REGISTRY}/${IMG}:${TAG}"
