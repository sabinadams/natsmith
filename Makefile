GO ?= go
BIN_DIR := bin

.PHONY: build clean install

build: ## Build bin/migrate-nats-kv and bin/migrate-nats-objects
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/migrate-nats-kv ./cmd/migrate-nats-kv
	$(GO) build -o $(BIN_DIR)/migrate-nats-objects ./cmd/migrate-nats-objects

install: ## Install migrate-nats-kv and migrate-nats-objects to $(GOBIN)
	$(GO) install ./cmd/migrate-nats-kv ./cmd/migrate-nats-objects

clean: ## Remove built binaries
	rm -rf $(BIN_DIR)
