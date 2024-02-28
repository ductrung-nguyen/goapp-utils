
GO_VERSION := 1.21.7
GOLANG_CI_LINT_VERSION := 1.55.2

# ginkgo version
GINKGO_VERSION := $(shell cat go.mod | grep ginkgo/v2 | cut -d" " -f2)
INSTALLED_GINKGO_VERSION =  $(shell (ginkgo version | cut -d ' ' -f3) || '')
INSTALLED_GOLANG_CI_LINT_VERSION =  $(shell (./bin/golangci-lint --version | grep -o "version \S*" | cut -d" " -f2) || '')

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

.PHONY: install-testing-tools
install-testing-tools: ## Install testing tools
	command -v ginkgo >/dev/null 2>&1 && echo "No need to install tesing tools again" || ( \
		go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@${GINKGO_VERSION}; \
		go get github.com/onsi/gomega/...; \
	)

	[ "v${INSTALLED_GINKGO_VERSION}" = "${GINKGO_VERSION}" ] || go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@${GINKGO_VERSION}

install-golang-cli:
	[ "${GOLANG_CI_LINT_VERSION}" = "${INSTALLED_GOLANG_CI_LINT_VERSION}" ] || (curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v${GOLANG_CI_LINT_VERSION})
	echo "Please make sure to put $$HOME/go/bin in your PATH"

.PHONY: init
## initialize the working environment
init: install-go install-testing-tools install-golang-cli
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

.PHONY: test
test: init fmt vet lint ## Run tests.
	ginkgo --json-report ./ginkgo.report -r --race --randomize-all -coverprofile=coverage.out --junit-report=report.xml
	go tool cover -html=coverage.out -o coverage.html
