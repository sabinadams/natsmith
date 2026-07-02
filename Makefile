GO ?= go
BIN_DIR := bin

.PHONY: build clean install test test-integration

build: ## Build bin/natsmith
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/natsmith ./cmd/natsmith

install: ## Install natsmith to $(GOBIN)
	$(GO) install ./cmd/natsmith

test: ## Run unit tests
	$(GO) test -race -count=1 ./...

test-integration: ## Run cross-cluster integration tests (requires Docker)
	$(GO) test -tags=integration -count=1 -timeout=10m ./internal/integration/ ./cmd/migrate/

clean: ## Remove built binaries and test artifacts
	rm -rf $(BIN_DIR)
	rm -f natsmith coverage.out
