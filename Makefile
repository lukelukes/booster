MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
.SUFFIXES:

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
TESTFLAGS ?=

.PHONY: all
all: build

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

##@ Build

.PHONY: build
build: | $(BUILD_DIR) ## Build the binary
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BINARY) $(PKG)

##@ Test

.PHONY: test
test: ## Run unit tests
	go test -race $(TESTFLAGS) ./internal/... ./cmd/...

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -race $(TESTFLAGS) ./tests/integration/...

.PHONY: test-all
test-all: test test-integration ## Run all tests

.PHONY: bench
bench: ## Run benchmarks
	go test -bench=. -benchmem -run=^$$ ./internal/... ./cmd/...

.PHONY: fuzz
fuzz: ## Run fuzz tests (30s)
	go test -fuzz=Fuzz -fuzztime=30s ./internal/...

.PHONY: fuzz-long
fuzz-long: ## Run fuzz tests (5m)
	go test -fuzz=Fuzz -fuzztime=5m ./internal/...

.PHONY: coverage
coverage: | $(BUILD_DIR) ## Generate coverage report
	go test -coverprofile=$(COVER_OUT) ./internal/... ./cmd/...
	go tool cover -func=$(COVER_OUT)

.PHONY: coverage-html
coverage-html: coverage ## Open coverage in browser
	go tool cover -html=$(COVER_OUT)

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	go clean -cache

.PHONY: install
install: build ## Install to ~/.local/bin/
	cp $(BINARY) ~/.local/bin/

##@ Mutation Testing

.PHONY: mutation
mutation: ## Run mutation testing on internal packages
	gremlins unleash \
		-E "_mock.*\.go$$" \
		-E "test_helpers\.go$$"

.PHONY: mutation-diff
mutation-diff: ## Run mutation testing only on changed files
	gremlins unleash --diff "origin/master" \
		--threshold-efficacy 80 \
		--threshold-mcover 80

.PHONY: mutation-report
mutation-report: | $(BUILD_DIR) ## Generate JSON mutation report
	gremlins unleash \
		-E "_mock.*\.go$$" \
		-E "test_helpers\.go$$" \
		--output $(BUILD_DIR)/mutation-report.json

##@ Lint & QA

.PHONY: fix
fix: ## Apply go fix modernizers
	@go fix ./...

.PHONY: install-hooks
install-hooks: ## Installs lefthook git hooks
	lefthook install

.PHONY: lint
lint: ## Run golangci-lint per .golangci.yml
	@golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with --fix (applies autofixes incl. formatters)
	@golangci-lint run --fix

.PHONY: fmt
fmt: ## Auto-fix formatting/imports
	@golangci-lint fmt

.PHONY: fmt-check
fmt-check: ## Check formatting/imports (fails if unformatted)
	@test -z "$$(golangci-lint fmt --diff)" || (echo "Run 'make fmt' to fix formatting" && exit 1)

.PHONY: vet
vet: ## Run go vet
	@go vet ./...

.PHONY: vuln
vuln: ## Run govulncheck ./... (Go vulnerability scanner)
	@$(GOBIN)/govulncheck ./...

.PHONY: tidy
tidy: ## Run go mod tidy -v
	@go mod tidy -v

.PHONY: tidy-check
tidy-check: ## Check that go.mod is tidy (fails if not, for CI)
	@go mod tidy -diff

.PHONY: mod-verify
mod-verify: ## Verify dependencies haven't been tampered with
	@go mod verify

.PHONY: verify-dev
verify-dev: build ## Run all quality checks - in local dev
	@$(MAKE) -j4 lint fmt-check vet vuln tidy mod-verify
	@$(MAKE) test-all

.PHONY: verify-ci
verify-ci: build ## Run all quality checks - in CI
	@$(MAKE) -j4 vet vuln tidy-check mod-verify
	@$(MAKE) test-all TESTFLAGS="-count=1"

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image with booster
	docker build -t booster-sandbox .

.PHONY: docker-run
docker-run: docker-build ## Run booster in Docker (interactive, tmux)
	docker run -it --rm \
		-w /workspace \
		-e HOME=/workspace \
		-e XDG_DATA_HOME=/workspace/.local/share \
		booster-sandbox

DOCKER_E2E_CONTAINER := booster-e2e

.PHONY: docker-e2e
docker-e2e: docker-build ## Start detached container for E2E testing
	@docker rm -f $(DOCKER_E2E_CONTAINER) 2>/dev/null || true
	docker run -d --name $(DOCKER_E2E_CONTAINER) \
		-w /workspace \
		-e HOME=/workspace \
		-e XDG_DATA_HOME=/workspace/.local/share \
		booster-sandbox
	@echo "Container ready. Send commands with:"
	@echo "  docker exec $(DOCKER_E2E_CONTAINER) tmux send-keys 'booster apply' Enter"
	@echo "  docker exec $(DOCKER_E2E_CONTAINER) tmux capture-pane -p"

.PHONY: docker-e2e-stop
docker-e2e-stop: ## Stop E2E testing container
	docker rm -f $(DOCKER_E2E_CONTAINER)

##@ Release

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	@git diff --quiet HEAD -- || (echo "Error: uncommitted changes in working tree" && exit 1)

.PHONY: release-dry
release-dry: ## Test goreleaser locally (no publish)
	goreleaser release --snapshot --clean

.PHONY: release
release: confirm no-dirty ## Create a release (requires GITHUB_TOKEN)
	goreleaser release --clean

##@ Utility

.PHONY: init-toolchain
init-toolchain: ## Installs all mise managed tools
	mise install
	lefthook install

.PHONY: upgrade-deps
upgrade-deps: ## Upgrades to the latest dependencies
	go get -u ./...
	make tidy

.PHONY: help
help: ## Show this help (auto-generated)
	@awk 'BEGIN {FS = ":.*##"; ORS="";} \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0,5); next } \
		/^[a-zA-Z0-9_.-]+:.*##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } \
		END { if (NR==0) print "No help available.\n" }' $(MAKEFILE_LIST)

##@ Aliases

.PHONY: b t ta l f v c i cov
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
