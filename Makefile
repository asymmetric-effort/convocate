VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := build
GO := go
GOFLAGS := -trimpath
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

BINARIES := claude-shell claude-host claude-agent

.PHONY: all generate build build-claude-shell build-claude-host build-claude-agent install clean lint lint-go lint-yaml lint-json test test-unit test-integration test-e2e release release/major release/minor

all: lint test build

generate:
	@echo "Generating embedded assets..."
	$(GO) generate ./internal/assets/
	@echo "Assets generated."

build: build-claude-shell build-claude-host build-claude-agent
	@echo "Build complete: $(BINARIES:%=$(BUILD_DIR)/%)"

build-claude-shell: generate
	@echo "Building claude-shell $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/claude-shell ./cmd/claude-shell/

build-claude-host:
	@echo "Building claude-host $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/claude-host ./cmd/claude-host/

build-claude-agent:
	@echo "Building claude-agent $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/claude-agent ./cmd/claude-agent/

install: build
	@echo "Installing claude-shell, claude-host, claude-agent to /usr/local/bin..."
	sudo install -m 0755 $(BUILD_DIR)/claude-shell  /usr/local/bin/claude-shell
	sudo install -m 0755 $(BUILD_DIR)/claude-host   /usr/local/bin/claude-host
	sudo install -m 0755 $(BUILD_DIR)/claude-agent  /usr/local/bin/claude-agent
	@echo "Running 'claude-shell install' to finish setup..."
	sudo /usr/local/bin/claude-shell install

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

release:
	@LATEST=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f2); \
	PATCH=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f3); \
	NEXT="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
	echo "Bumping $$LATEST -> $$NEXT"; \
	git tag "$$NEXT" && git push --tags && \
	echo "Released $$NEXT"

release/minor:
	@LATEST=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f2); \
	NEXT="v$$MAJOR.$$((MINOR + 1)).0"; \
	echo "Bumping $$LATEST -> $$NEXT"; \
	git tag "$$NEXT" && git push --tags && \
	echo "Released $$NEXT"

release/major:
	@LATEST=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	NEXT="v$$((MAJOR + 1)).0.0"; \
	echo "Bumping $$LATEST -> $$NEXT"; \
	git tag "$$NEXT" && git push --tags && \
	echo "Released $$NEXT"
