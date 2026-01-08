BUILD_DIR := out
BINARY := $(BUILD_DIR)/booster
PKG := ./cmd/cli
COVER_OUT := $(BUILD_DIR)/coverage.out
GOBIN := $(shell go env GOPATH)/bin

# Version info for ldflags
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.Date=$(DATE)"

.PHONY: all build test test-integration test-all coverage clean install help
.PHONY: mutation mutation-dry mutation-diff mutation-report
.PHONY: release release-dry
.PHONY: b t ta l f v c i cov

all: build

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

##@ Build
build: | $(BUILD_DIR) ## Build the binary
	go build -trimpath $(LDFLAGS) -o $(BINARY) $(PKG)

##@ Test
test: ## Run unit tests
	go test -race ./internal/...

test-integration: ## Run integration tests
	go test -race ./tests/integration/...

test-all: test test-integration

coverage: | $(BUILD_DIR) ## Generate coverage report
	go test -coverprofile=$(COVER_OUT) ./internal/... ./cmd/...
	go tool cover -func=$(COVER_OUT)

coverage-html: coverage ## Open coverage in browser
	go tool cover -html=$(COVER_OUT)

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

install: build ## Install to ~/.local/bin/
	cp $(BINARY) ~/.local/bin/

##@ Mutation Testing
mutation: ## Run mutation testing on internal packages
	gremlins unleash \
		-E "_mock.*\.go$$" \
		-E "test_helpers\.go$$"

mutation-diff: ## Run mutation testing only on changed files
	gremlins unleash --diff "origin/master" \
		--threshold-efficacy 80 \
		--threshold-mcover 80

mutation-report: | $(BUILD_DIR) ## Generate JSON mutation report
	gremlins unleash \
		-E "_mock.*\.go$$" \
		-E "test_helpers\.go$$" \
		--output $(BUILD_DIR)/mutation-report.json

##@ Lint & QA
install-hooks: ## Installs lefthook git hooks
	lefthook install

lint: ## Run golangci-lint per .golangci.yml
	@golangci-lint run

lint-fix: ## Run golangci-lint with --fix (applies autofixes incl. formatters)
	@golangci-lint run --fix

fmt: ## Auto-fix formatting/imports
	@golangci-lint fmt

fmt-check: ## Check formatting/imports
	@golangci-lint fmt

vet: ## Run go vet
	@go vet ./...

vuln: ## Run govulncheck ./... (Go vulnerability scanner)
	@$(GOBIN)/govulncheck ./...

tidy: ## Run go mod tidy -v
	@go mod tidy -v

verify-dev: build ## Run all quality checks - in local dev
	@$(MAKE) -j4 lint fmt-check vet vuln tidy
	@$(MAKE) test-all

verify-ci: build ## Run all quality checks - in CI
	@$(MAKE) -j4 vet vuln tidy
	@$(MAKE) test-all

##@ Release
release-dry: ## Test goreleaser locally (no publish)
	goreleaser release --snapshot --clean

release: ## Create a release (requires GITHUB_TOKEN)
	goreleaser release --clean

##@ Utility
init-toolchain: ## Installs all mise managed tools
	mise install

help: ## Show this help (auto-generated)
	@awk 'BEGIN {FS = ":.*##"; ORS="";} \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0,5); next } \
		/^[a-zA-Z0-9_.-]+:.*##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } \
		END { if (NR==0) print "No help available.\n" }' $(MAKEFILE_LIST)

##@ Aliases
b: build    ## build
t: test     ## test
ta: test-all ## test-all
l: lint     ## lint
f: fmt      ## fmt
v: verify-dev ## verify-dev
c: clean    ## clean
i: install  ## install
cov: coverage ## coverage

.DEFAULT_GOAL := help
