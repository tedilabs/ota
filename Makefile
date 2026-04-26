# ota Makefile
# See docs/PROJECT_STRUCTURE.md §9 for target contracts.

GO          ?= go
BIN_DIR     ?= bin
BIN         ?= $(BIN_DIR)/ota
PKG         ?= ./...
VERSION_PKG := github.com/tedilabs/ota/internal/version
LDFLAGS     := -X $(VERSION_PKG).Tag=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
               -X $(VERSION_PKG).Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
               -X $(VERSION_PKG).BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: all build test test-short test-race test-integration test-e2e lint vuln ci run fmt tidy clean

all: build

build: ## Build the ota binary.
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/ota

test: ## Run unit + adapter integration + tui component tests.
	$(GO) test -count=1 $(PKG)

test-short: ## Run the fast subset (skips long-running tests via -short).
	$(GO) test -short -count=1 $(PKG)

test-race: ## Run tests with the race detector.
	$(GO) test -race -count=1 $(PKG)

test-integration: ## Run integration-tagged tests (httptest-backed).
	$(GO) test -tags=integration -count=1 $(PKG)

test-e2e: ## Run the live-tenant tests. Requires OKTA_ORG_URL and OKTA_API_TOKEN.
	@[ -n "$$OKTA_ORG_URL" ] && [ -n "$$OKTA_API_TOKEN" ] || (echo "set OKTA_ORG_URL + OKTA_API_TOKEN" && exit 1)
	$(GO) test -tags=e2e -count=1 $(PKG)

lint: ## gofumpt + golangci-lint.
	gofumpt -l -d .
	golangci-lint run $(PKG)

vuln: ## govulncheck.
	govulncheck $(PKG)

ci: lint vuln test test-race ## Full CI gate.

run: ## Run from source (requires OKTA_* envs).
	$(GO) run ./cmd/ota

fmt: ## Apply gofumpt.
	gofumpt -w .

tidy: ## go mod tidy.
	$(GO) mod tidy

clean: ## Remove build artifacts.
	rm -rf $(BIN_DIR) coverage.out coverage.html
