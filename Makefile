
GO_VERSION := 1.20

# ginkgo version
GINKGO_VERSION := $(shell cat go.mod | grep ginkgo/v2 | cut -d" " -f2)

GOLANG_CI_LINT_VERSION := 1.52.2

.PHONY: all $(DIRS)
all: test

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Initialize env
.PHONY: install-go
install-go:
	command -v go >/dev/null 2>&1 && ( echo >&2 "Go is installed. No need to install again"; exit 0; ) || \
	( echo >&2 "Installing go. Need sudo permission" && \
		curl -Lo go_installer https://get.golang.org/linux && \
		chmod +x go_installer -version ${GO_VERSION} && \
		sudo ./go_installer && rm go_installer )

.PHONY: install-testing-package
install-testing-package:
	command -v ginkgo >/dev/null 2>&1 && echo "No need to install testing package again" || ( \
		go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@${GINKGO_VERSION}; \
		go get github.com/onsi/gomega/...; \
	)

install-golang-cli:
    # wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v${GOLANG_CI_LINT_VERSION}; \
	[ -f "./bin/golangci-lint" ] && echo "No need to install golang-cli again" ||  (curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v${GOLANG_CI_LINT_VERSION})

.PHONY: init
## initialize the working environment
init: install-go install-testing-package install-golang-cli
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
