
# Image URL to use all building/pushing image targets
IMG ?= fluentd_issue_detector

# tag of the docker image
TAG := $(shell date --iso=seconds)

# Registry
REGISTRY := dockerhub.rnd.amadeus.net:5002/splunk/fluend-issue-detector

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Initialize env
.PHONY: install-go
install-go:
	command -v go >/dev/null 2>&1 && ( echo >&2 "Go is installed. No need to install again"; exit 0; ) || \
	( echo >&2 "Installing go. Need sudo permission" && sudo ./scripts/install_go.sh )

.PHONY: install-linters
install-linters:
	command -v ginkgo >/dev/null 2>&1 && echo "No need to install linters again" || ( \
		wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.46.2; \
		go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@v2.4.0; \
		go get github.com/onsi/gomega/...; \
	)

install-golang-cli:
	[ -f "./bin/golangci-lint" ] && echo "No need to install golang-cli again" ||  (curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.46.2)

.PHONY: init
## initialize the working environment
init: install-go install-linters install-golang-cli
	go mod download
	go mod tidy

##@ Development	

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: ## Run go vet against code.
	./bin/golangci-lint run

ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

.PHONY: test
test: init fmt vet lint envtest ## Run tests.
	ginkgo --json-report ./ginkgo.report -r -cover -coverprofile=coverage.out --junit-report=report.xml
	go tool cover -html=coverage.out -o coverage.html

##@ Build

.PHONY: build
build: fmt vet lint ## Build manager binary.
	go build -o bin/manager

.PHONY: run
run: fmt vet lint ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG}:latest .

.PHONY: docker-build-local
## build the LOCAL container image with your current KUBECONFIG (assume that it is '~/.kube/config')
docker-build-local: test
	docker build -t ${IMG}:latest  --build-arg KUBECONFIG="$(cat ~/.kube/config)" .

.PHONY: docker-run
docker-run: ## Run docker docker container with the LOCAL container image.
	[ -f "./config.test.yaml" ] || cp ./config.test.yaml.sample ./config.test.yaml
	docker run -v `pwd`/config.test.yaml:/home/nonroot/config.yaml --env KUBECONFIG="/home/nonroot/workspace/.kube_config"  fluentd_issue_detector:latest

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker tag ${IMG}:latest ${REGISTRY}/${IMG}:${TAG}
	docker push ${REGISTRY}/${IMG}:${TAG}
	echo "Pushed docker image to: ${REGISTRY}/${IMG}:${TAG}"
