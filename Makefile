VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := build
GO := $(shell command -v go 2>/dev/null || echo /usr/local/go/bin/go)
GOFLAGS := -trimpath
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"
COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

BINARIES := convocate-router convocate-dispatch convocate-secrets-broker convocate-agent-wrapper convocate-cli mock-claude

.PHONY: all dev auth build clean lint lint-go lint-yaml lint-vuln test test-unit test-integration test-e2e test-coverage \
        images image-router image-dispatch image-secrets-broker image-agent image-redis image-openbao \
        local/start local/logs local/stop local/reset local/test local/pdv hooks verify \
        release release/minor release/major

all: lint test build

# --- Dev Environment Setup ---

dev:
	@echo "Installing development dependencies..."
	@command -v $(GO) >/dev/null 2>&1 || { echo "ERROR: Go not found. Install Go 1.26+ from https://go.dev/dl/"; exit 1; }
	@echo "  Go: $$($(GO) version)"
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "  golangci-lint: installed"
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "  govulncheck: installed"
	@command -v yamllint >/dev/null 2>&1 || pip install --break-system-packages yamllint 2>/dev/null || pip install yamllint
	@echo "  yamllint: $$(yamllint --version)"
	@command -v node >/dev/null 2>&1 || { echo "WARNING: Node.js not found. Install Node 24+ for Web UI builds."; }
	@if command -v node >/dev/null 2>&1; then \
		echo "  node: $$(node --version)"; \
		cd internal/webui && npm install; \
		echo "  webui deps: installed"; \
	fi
	@if [ ! -f .git/hooks/pre-commit ]; then \
		$(MAKE) hooks; \
	else \
		echo "  git hooks: already installed"; \
	fi
	@COMPOSE_CMD=""; \
	if docker compose version >/dev/null 2>&1; then \
		COMPOSE_CMD="docker compose"; \
	elif command -v docker-compose >/dev/null 2>&1; then \
		COMPOSE_CMD="docker-compose"; \
	else \
		echo "WARNING: Docker Compose not found. Install for local dev stack."; \
	fi; \
	if [ -n "$$COMPOSE_CMD" ]; then \
		echo "  compose: $$COMPOSE_CMD ($$($$COMPOSE_CMD version 2>/dev/null || echo unknown))"; \
	fi
	@echo "Development environment ready."

# --- GitHub OAuth Setup ---

auth:
	@echo "=== GitHub OAuth Setup ==="
	@echo ""
	@echo "Create an OAuth App at: https://github.com/settings/developers"
	@echo "  Homepage URL:  https://localhost:8443"
	@echo "  Callback URL:  https://localhost:8443/auth/callback"
	@echo ""
	@printf "Enter GitHub Client ID: " && read CLIENT_ID && \
	printf "Enter GitHub Client Secret: " && read CLIENT_SECRET && \
	mkdir -p .dev && \
	printf "GITHUB_CLIENT_ID=%s\nGITHUB_CLIENT_SECRET=%s\nCONVOCATE_AUTH_ORG=asymmetric-effort\n" \
		"$$CLIENT_ID" "$$CLIENT_SECRET" > .dev/auth.env && \
	echo "" && \
	echo "Credentials saved to .dev/auth.env" && \
	echo "Run 'make local/reset' to apply."

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

images: image-tls-init image-router image-dispatch image-secrets-broker image-agent image-redis image-openbao
	@echo "All images built."

image-tls-init:
	@echo "Building convocate-tls-init image..."
	docker build -f deploy/control-plane/Dockerfile.tls-init \
		-t convocate-tls-init:latest .

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
	@if [ -f .dev/auth.env ]; then set -a; . ./.dev/auth.env; set +a; fi; \
	$(COMPOSE) -f docker-compose.dev.yml up -d
	@echo "Waiting for Router API to become healthy..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do \
		if curl -fsSk https://localhost:8443/v1/health >/dev/null 2>&1; then \
			echo "Router API healthy: $$(curl -sk https://localhost:8443/v1/health)"; \
			exit 0; \
		fi; \
		if docker run --rm --network convocate_convocate-dev --entrypoint sh convocate-tls-init:latest \
			-c 'curl -fsSk https://convocate-router:443/v1/health' 2>/dev/null; then \
			echo ""; \
			echo "Router API healthy (via docker network)"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	echo "ERROR: Router API not healthy after 40s"; \
	$(COMPOSE) -f docker-compose.dev.yml logs --tail=20 router; \
	exit 1

local/logs:
	$(COMPOSE) -f docker-compose.dev.yml logs -f

local/stop:
	@echo "Stopping local dev environment (volumes preserved)..."
	$(COMPOSE) -f docker-compose.dev.yml stop

local/reset:
	@echo "Tearing down local dev environment (volumes removed)..."
	$(COMPOSE) -f docker-compose.dev.yml down -v
	@rm -rf .dev/secrets
	@echo "Regenerating local CA on next start."
	@$(MAKE) local/start

# DCURL runs curl against the router inside the Docker network.
# Falls back to localhost for environments where port mapping works.
DCURL = docker run --rm --network convocate_convocate-dev --entrypoint sh \
	convocate-tls-init:latest -c

local/test:
	@echo "=== Post-deployment verification tests ==="
	@echo "--- Health check ---"
	@$(DCURL) 'curl -fsSk https://convocate-router:443/v1/health' || \
		{ echo "FAIL: /v1/health unreachable"; exit 1; }
	@echo ""
	@echo "--- Health alias ---"
	@$(DCURL) 'curl -fsSk https://convocate-router:443/health' || \
		{ echo "FAIL: /health unreachable"; exit 1; }
	@echo ""
	@echo "--- Root serves Web UI or auth redirect ---"
	@HTTP=$$($(DCURL) 'curl -sk -o /dev/null -w "%{http_code}" https://convocate-router:443/'); \
	if [ "$$HTTP" = "200" ] || [ "$$HTTP" = "302" ]; then \
		echo "OK: / returns $$HTTP"; \
	else \
		echo "FAIL: / returned $$HTTP (expected 200 or 302)"; exit 1; \
	fi
	@echo "--- Auth enforcement ---"
	@HTTP_CODE=$$($(DCURL) 'curl -sk -o /dev/null -w "%{http_code}" \
		-X POST https://convocate-router:443/v1/jobs \
		-H "Content-Type: application/json" \
		-d "{\"repository\":\"test/repo\",\"run_id\":1}"'); \
	if [ "$$HTTP_CODE" != "401" ]; then \
		echo "FAIL: /v1/jobs without token returned $$HTTP_CODE, want 401"; exit 1; \
	fi; \
	echo "OK: /v1/jobs returns 401 without token"
	@echo "--- Allowlist enforcement ---"
	@HTTP_CODE=$$($(DCURL) 'curl -sk -o /dev/null -w "%{http_code}" \
		-X POST https://convocate-router:443/v1/jobs \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer bad-token" \
		-d "{\"repository\":\"unknown/repo\",\"run_id\":1}"'); \
	if [ "$$HTTP_CODE" != "404" ]; then \
		echo "FAIL: unknown repo returned $$HTTP_CODE, want 404"; exit 1; \
	fi; \
	echo "OK: unknown repo returns 404"
	@echo "--- Web UI API (responds, may require auth) ---"
	@HTTP=$$($(DCURL) 'curl -sk -o /dev/null -w "%{http_code}" https://convocate-router:443/ui/api/projects'); \
	if [ "$$HTTP" = "200" ] || [ "$$HTTP" = "401" ] || [ "$$HTTP" = "302" ]; then \
		echo "OK: /ui/api/projects returns $$HTTP"; \
	else \
		echo "FAIL: /ui/api/projects returned $$HTTP"; exit 1; \
	fi
	@HTTP=$$($(DCURL) 'curl -sk -o /dev/null -w "%{http_code}" https://convocate-router:443/ui/api/jobs'); \
	if [ "$$HTTP" = "200" ] || [ "$$HTTP" = "401" ] || [ "$$HTTP" = "302" ]; then \
		echo "OK: /ui/api/jobs returns $$HTTP"; \
	else \
		echo "FAIL: /ui/api/jobs returned $$HTTP"; exit 1; \
	fi
	@HTTP=$$($(DCURL) 'curl -sk -o /dev/null -w "%{http_code}" https://convocate-router:443/ui/api/hosts'); \
	if [ "$$HTTP" = "200" ] || [ "$$HTTP" = "401" ] || [ "$$HTTP" = "302" ]; then \
		echo "OK: /ui/api/hosts returns $$HTTP"; \
	else \
		echo "FAIL: /ui/api/hosts returned $$HTTP"; exit 1; \
	fi
	@echo "--- Internal port (8443) ---"
	@$(DCURL) 'curl -fsSk https://convocate-router:8443/v1/health' > /dev/null || \
		{ echo "FAIL: port 8443 /v1/health unreachable"; exit 1; }
	@echo "OK: port 8443 serves /v1/health"
	@echo "=== All local verification tests passed ==="

local/pdv:
	@echo "=== Running Playwright PDV tests against https://localhost:8443/ ==="
	cd internal/webui && npx playwright install --with-deps chromium && \
		npx playwright test --reporter=list
	@echo "=== Playwright PDV tests passed ==="

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
