VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := build
GO := $(shell command -v go 2>/dev/null || echo /usr/local/go/bin/go)
GOFLAGS := -trimpath
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

BINARIES := convocate-router convocate-dispatch convocate-secrets-broker convocate-agent-wrapper convocate-cli mock-claude

.PHONY: all build clean lint lint-go lint-yaml lint-vuln test test-unit test-integration test-e2e test-coverage \
        images image-router image-dispatch image-secrets-broker image-agent image-redis image-openbao \
        local/start local/logs local/stop local/reset hooks verify \
        release release/minor release/major

all: lint test build

# --- Build ---

build: $(BINARIES:%=build-%)
	@echo "Build complete: $(BINARIES:%=$(BUILD_DIR)/%)"

build-%:
	@echo "Building $* $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$* ./cmd/$*/

# --- Clean ---

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@mkdir -p $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@$(GO) clean -testcache
	@echo "Removing convocate Docker containers and images..."
	@docker ps -aq --filter "name=convocate-" 2>/dev/null | xargs -r docker rm -f 2>/dev/null || true
	@docker images -q "convocate-*" 2>/dev/null | xargs -r docker rmi -f 2>/dev/null || true
	@echo "Clean complete."

# --- Lint ---

lint: lint-go lint-yaml lint-vuln
	@echo "All linters passed."

lint-go:
	@echo "Running Go linter..."
	@command -v golangci-lint >/dev/null 2>&1 || $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@PATH="$$($(GO) env GOBIN):$$($(GO) env GOPATH)/bin:$$PATH" golangci-lint run --config .golangci-lint.yml ./...

lint-yaml:
	@echo "Running YAML linter..."
	@find . -name '*.yml' -o -name '*.yaml' | grep -v vendor | grep -v node_modules | grep -v .dev | xargs yamllint -s

lint-vuln:
	@echo "Running govulncheck against vuln.go.dev..."
	@command -v govulncheck >/dev/null 2>&1 || $(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	@PATH="$$($(GO) env GOBIN):$$($(GO) env GOPATH)/bin:$$PATH" govulncheck ./... || \
		echo "WARNING: govulncheck found vulnerabilities (may be stdlib — upgrade Go to fix)"

# --- Test ---

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

# --- OCI Images ---

images: image-router image-dispatch image-secrets-broker image-agent image-redis image-openbao
	@echo "All images built."

image-router:
	@echo "Building convocate-router image $(VERSION)..."
	docker build -f deploy/control-plane/Dockerfile.router \
		--build-arg VERSION=$(VERSION) \
		-t convocate-router:$(VERSION) \
		-t convocate-router:latest .

image-dispatch:
	@echo "Building convocate-dispatch image $(VERSION)..."
	docker build -f deploy/agent-host/Dockerfile.dispatch \
		--build-arg VERSION=$(VERSION) \
		-t convocate-dispatch:$(VERSION) \
		-t convocate-dispatch:latest .

image-secrets-broker:
	@echo "Building convocate-secrets-broker image $(VERSION)..."
	docker build -f deploy/agent-host/Dockerfile.secrets-broker \
		--build-arg VERSION=$(VERSION) \
		-t convocate-secrets-broker:$(VERSION) \
		-t convocate-secrets-broker:latest .

image-agent:
	@echo "Building agent binaries (linux/amd64, static)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(LDFLAGS) \
		-o $(BUILD_DIR)/convocate-agent-wrapper ./cmd/convocate-agent-wrapper/
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(LDFLAGS) \
		-o $(BUILD_DIR)/mock-claude ./cmd/mock-claude/
	@echo "Building convocate-agent image $(VERSION)..."
	docker build -f deploy/agent-host/Dockerfile.agent \
		--platform linux/amd64 \
		--build-arg VERSION=$(VERSION) \
		$(if $(CONVOCATE_DEV_MOCK_CLAUDE),--build-arg DEV_MOCK_CLAUDE=1,) \
		-t convocate-agent:$(VERSION) \
		-t convocate-agent:latest .

image-redis:
	@echo "Building convocate-redis image..."
	docker build -f deploy/control-plane/Dockerfile.redis \
		-t convocate-redis:latest .

image-openbao:
	@echo "Building convocate-openbao image..."
	docker build -f deploy/control-plane/Dockerfile.openbao \
		-t convocate-openbao:latest .

# --- Local Dev Environment ---

local/start: images
	@echo "Starting local dev environment..."
	docker compose -f docker-compose.dev.yml up -d
	@echo "Waiting for Router API to become healthy..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do \
		if curl -fsSk https://localhost:8443/v1/health >/dev/null 2>&1; then \
			echo "Router API healthy: $$(curl -sk https://localhost:8443/v1/health)"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	echo "ERROR: Router API not healthy after 40s"; \
	docker compose -f docker-compose.dev.yml logs --tail=20 router; \
	exit 1

local/logs:
	docker compose -f docker-compose.dev.yml logs -f

local/stop:
	@echo "Stopping local dev environment (volumes preserved)..."
	docker compose -f docker-compose.dev.yml stop

local/reset:
	@echo "Tearing down local dev environment (volumes removed)..."
	docker compose -f docker-compose.dev.yml down -v
	@rm -rf .dev/secrets
	@echo "Regenerating local CA on next start."
	@$(MAKE) local/start

# --- Git Hooks ---

hooks:
	@echo "Installing git hooks..."
	@cp scripts/pre-commit .git/hooks/pre-commit
	@cp scripts/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push
	@echo "Git hooks installed."

# --- Post-deployment Verification ---

verify:
	@scripts/post-deploy-verify $(CONVOCATE_PUBLIC_URL)

# --- Release ---

release:
	@LATEST=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f2); \
	PATCH=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f3); \
	NEXT="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
	echo "Bumping $$LATEST -> $$NEXT"; \
	git tag "$$NEXT" && git push origin "$$NEXT" && \
	echo "Released $$NEXT"

release/minor:
	@LATEST=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f2); \
	NEXT="v$$MAJOR.$$((MINOR + 1)).0"; \
	echo "Bumping $$LATEST -> $$NEXT"; \
	git tag "$$NEXT" && git push origin "$$NEXT" && \
	echo "Released $$NEXT"

release/major:
	@LATEST=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	NEXT="v$$((MAJOR + 1)).0.0"; \
	echo "Bumping $$LATEST -> $$NEXT"; \
	git tag "$$NEXT" && git push origin "$$NEXT" && \
	echo "Released $$NEXT"
