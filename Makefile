BINARY_NAME := claude-shell
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := build
GO := go
GOFLAGS := -trimpath
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

.PHONY: all generate build clean lint lint-go lint-yaml lint-json test test-unit test-integration test-e2e

all: lint test build

generate:
	@echo "Generating embedded assets..."
	$(GO) generate ./internal/assets/
	@echo "Assets generated."

build: generate
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/claude-shell/
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@$(GO) clean -testcache
	@echo "Clean complete."

lint: lint-go lint-yaml
	@echo "All linters passed."

lint-go:
	@echo "Running Go linter..."
	go vet -v ./...

lint-yaml:
	@echo "Running YAML linter..."
	@find . -name '*.yml' -o -name '*.yaml' | grep -v vendor | xargs yamllint -s

test: test-unit test-integration
	@echo "All tests passed."

test-unit:
	@echo "Running unit tests..."
	$(GO) test -v -race -count=1 -coverprofile=coverage.out ./internal/...
	@$(GO) tool cover -func=coverage.out | tail -1
	@echo "Unit tests complete."

test-integration:
	@echo "Running integration tests..."
	$(GO) test -v -race -count=1 -tags=integration ./test/integration/...
	@echo "Integration tests complete."

test-e2e:
	@echo "Running end-to-end tests..."
	$(GO) test -v -count=1 -tags=e2e -timeout=300s ./test/e2e/...
	@echo "End-to-end tests complete."

test-coverage: test-unit
	@echo "Generating coverage report..."
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
