# Ralph Makefile
# Development commands for building, running, testing, and maintaining Ralph.

.DEFAULT_GOAL := help
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# Configuration
BIN_DIR ?= bin
BINARY_NAME ?= ralph
BINARY ?= $(BIN_DIR)/$(BINARY_NAME)
CMD_DIR ?= ./cmd/ralph
PKG ?= ./...
GO ?= go
GOFLAGS ?=
COVERAGE_FILE ?= coverage.out

.PHONY: all help build run install deps install-deps tidy deps-verify test test-race test-cover fmt fmt-check vet lint validate clean

all: build ## Build binary (default)

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

build: ## Build the ralph CLI binary
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BINARY) $(CMD_DIR)

run: ## Run ralph via go run (pass arguments with ARGS="...")
	$(GO) run $(CMD_DIR) $(ARGS)

install: ## Install ralph binary into $$GOPATH/bin
	$(GO) install $(GOFLAGS) $(CMD_DIR)

deps: ## Download Go module dependencies
	$(GO) mod download

install-deps: deps ## Alias for deps

tidy: ## Prune and tidy Go module dependencies
	$(GO) mod tidy

deps-verify: ## Verify Go module dependencies
	$(GO) mod verify

test: ## Run unit tests
	$(GO) test -v $(PKG)

test-race: ## Run unit tests with race detection
	$(GO) test -v -race $(PKG)

test-cover: ## Run unit tests with coverage profile
	$(GO) test -v -coverprofile=$(COVERAGE_FILE) $(PKG)
	$(GO) tool cover -func=$(COVERAGE_FILE)

fmt: ## Format Go source files using gofmt
	gofmt -s -w .

fmt-check: ## Check Go code formatting without modifying files
	@test -z "$$(gofmt -s -l .)" || (echo "Unformatted files found. Run 'make fmt'." && exit 1)

vet: ## Run go vet code analysis
	$(GO) vet $(PKG)

lint: fmt-check vet ## Run formatting and vet checks

validate: ## Run Ralph pre-flight readiness checks
	$(GO) run $(CMD_DIR) validate

clean: ## Clean build outputs and temporary coverage files
	rm -rf $(BIN_DIR) $(COVERAGE_FILE)
