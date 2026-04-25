# ──────────────────────────────────────────────────────────────────────────────
# Variables
# ──────────────────────────────────────────────────────────────────────────────
BINARY_DIR      := ./build
GENTOOL         := $(BINARY_DIR)/gentool
COMPOSE_FILE    := tests/integration/docker-compose.yml
SMOKE_FILE      := tests/smoke/docker-compose.yml

GO              := go
CGO_ENABLED     ?= 0
GOFLAGS         := CGO_ENABLED=$(CGO_ENABLED)
VERSION         := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_FLAGS     := -trimpath -ldflags="-s -w -X 'github.com/ny4rl4th0t3p/cosmos-genesis-tool/cmd/gentool/cmd.Version=$(VERSION)'"

# Packages used by the various test targets
UNIT_PKGS       := ./internal/... ./cmd/...

# ──────────────────────────────────────────────────────────────────────────────
# Default target
# ──────────────────────────────────────────────────────────────────────────────
.DEFAULT_GOAL := help

# ──────────────────────────────────────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2}'

# ──────────────────────────────────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: build
build: ## Build the gentool binary → build/gentool
	@mkdir -p $(BINARY_DIR)
	$(GOFLAGS) $(GO) build $(BUILD_FLAGS) -o $(GENTOOL) ./cmd/gentool

.PHONY: install
install: ## Install gentool to $(GOPATH)/bin (or ~/go/bin)
	$(GOFLAGS) $(GO) install $(BUILD_FLAGS) ./cmd/gentool

# ──────────────────────────────────────────────────────────────────────────────
# Test
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: test
test: ## Run unit tests
	$(GO) test -count=1 $(UNIT_PKGS)

.PHONY: test-verbose
test-verbose: ## Run unit tests with verbose output
	$(GO) test -v -count=1 $(UNIT_PKGS)

.PHONY: test-race
test-race: ## Run unit tests with the race detector
	$(GO) test -race -count=1 $(UNIT_PKGS)

.PHONY: test-cover
test-cover: ## Run unit tests and open an HTML coverage report
	$(GO) test -count=1 -coverprofile=coverage.out $(UNIT_PKGS)
	$(GO) tool cover -html=coverage.out

.PHONY: test-integration
test-integration: ## Run the full Docker integration test (builds image + runs scenario)
	docker compose -f $(COMPOSE_FILE) up --build --abort-on-container-exit --exit-code-from integration-test
	docker compose -f $(COMPOSE_FILE) down --remove-orphans

.PHONY: integration-build
integration-build: ## Build the integration test Docker image without running
	docker compose -f $(COMPOSE_FILE) build

.PHONY: integration-clean
integration-clean: ## Remove integration test containers and images
	docker compose -f $(COMPOSE_FILE) down --rmi local --remove-orphans

.PHONY: test-smoke
test-smoke: ## Run the smoke test (2-validator network boots + produces a block)
	docker compose -f $(SMOKE_FILE) up --build --abort-on-container-exit --exit-code-from smoke-test
	docker compose -f $(SMOKE_FILE) down --remove-orphans

.PHONY: smoke-clean
smoke-clean: ## Remove smoke test containers and images
	docker compose -f $(SMOKE_FILE) down --rmi local --remove-orphans

# ──────────────────────────────────────────────────────────────────────────────
# Code quality
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: fmt
fmt: ## Format all Go source files
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Check formatting without modifying files (exits non-zero if changes needed)
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	$(GO) mod tidy

.PHONY: tidy-check
tidy-check: ## Check that go.mod and go.sum are tidy (exits non-zero if not)
	$(GO) mod tidy && git diff --exit-code go.mod go.sum

.PHONY: lint
lint: ## Run golangci-lint (report only — used by check/CI)
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint and auto-fix what it can (misspell, gofmt, goimports, whitespace, …)
	golangci-lint run --fix

.PHONY: check
check: fmt-check vet tidy-check lint test ## Run all checks (CI entry point: fmt + vet + tidy + lint + unit tests)

# ──────────────────────────────────────────────────────────────────────────────
# Clean
# ──────────────────────────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove build artifacts and coverage files
	rm -rf $(BINARY_DIR) coverage.out