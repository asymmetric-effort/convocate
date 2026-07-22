# Convocate v2 — Project Instructions

## Overview

Convocate is an agentic software-engineering platform. This is a complete v2
rewrite. The repository is a monorepo with two primary services:

- **Web UI** — TypeScript + Bun + @asymmetric-effort/specifyjs SPA
- **API** — Go 1.26+ REST server

## Architecture

Five containers orchestrated via Docker Compose:

| Container  | Language/Runtime | Build Stage    | Runtime Stage |
|------------|-----------------|----------------|---------------|
| Web UI     | Bun + SpecifyJS | ubuntu:26.04   | distroless    |
| API        | Go 1.26+        | ubuntu:26.04   | distroless    |
| Redis      | —               | ubuntu:26.04   | distroless    |
| PostgreSQL | —               | ubuntu:26.04   | distroless    |
| OpenBao    | —               | ubuntu:26.04   | distroless    |

Storage tiering: file-based JSON first, Redis for key-value/sessions,
PostgreSQL only when relational queries are truly necessary.

## Authoritative Documents

- **API contract**: `openapi.yaml` — the controlling authority for all API
  endpoints, schemas, and RBAC roles.
- **Specification**: `SPECIFICATION.md` — product requirements and domain model.
## Project Layout

```
src/api/          — Go API server (module: github.com/asymmetric-effort/convocate)
src/agent/        — Go agent
src/ui/           — Bun + SpecifyJS SPA
  src/ui/img/     — icon assets
infrastructure/
  charts/         — Helm charts
  deploy/         — deployment scripts
  docker/         — Dockerfiles (ui, api, redis, pg, openbao, etc.)
  k8s/            — K8s manifests
  inventory/      — Ansible inventory
  playbooks/      — Ansible playbooks
  roles/          — Ansible roles
test/pdv/         — PDV tests
build/            — build artifacts (gitignored)
```

## Build & Run

```bash
# Local development (no containers needed)
cd src/ui && bun install && bun run dev    # UI on :8080
cd src/api && go run ./...                 # API on :8443

# Docker Compose (full stack)
docker compose up --build
```

## Make Targets

| Target       | Description |
|--------------|-------------|
| `make clean` | Remove all container images and built artifacts in `build/`, recreate `build/` |
| `make lint`  | Run all linters (Go, TypeScript, SQL, Markdown, Makefiles, JS/CSS/HTML, YAML, JSON, Dockerfiles) |
| `make test`  | Run all unit, integration and e2e tests including Playwright browser tests |
| `make build` | Build all container images and GitHub Pages website artifacts |
| `make cover` | Run code coverage and fail if below 98% |

## Coding Standards

**Authoritative reference:** <http://coding-standards.asymmetric-effort.com/>

### Go

- No recursion. Go has no tail-call optimization; use loops and explicit
  work-list patterns.
- ed25519-only SSH keys. Every key the project generates must be ed25519.
  No RSA, no ECDSA.
- Use `gofmt` for formatting.
- Tests go in `_test.go` files alongside the code they test.

### TypeScript

- Use Bun as the runtime and bundler.
- All UI components use @asymmetric-effort/specifyjs.

## Dependency Policy

**HARD RULE: Zero third-party dependencies unless explicitly approved.**

### Approved Dependencies

| Package                              | Language | Purpose            |
|--------------------------------------|----------|--------------------|
| `@asymmetric-effort/specifyjs`       | TS       | UI framework       |
| `@asymmetric-effort/nogginlessdom`   | TS       | DOM library        |
| `@asymmetric-effort/yamllint`        | TS       | YAML linter        |
| `@asymmetric-effort/jsonlint`        | TS       | JSON linter        |
| `redis/go-redis`                     | Go       | Redis client       |
| `jackc/pgx`                          | Go       | PostgreSQL driver  |
| `openbao/openbao`                    | Go       | Secret store client |
| `k8s.io/client-go`                  | Go       | Kubernetes API client |

Everything else must use language standard libraries. Do not add any
dependency — including test frameworks, linters, or utility packages —
without explicit approval.

## API Conventions

- All endpoints: `/api/v1/<applet_shortname>/...`
- Applet shortnames: `nmgr`, `amgr`, `pb`, `ide`, `repo`, `ac`, `sup`, `auth`
- Event channels: `/api/v1/events/{applet}/{channel}` (WebSocket)
- Agent shell: `/api/v1/amgr/agent/{agentId}/shell` (WebSocket)
- Bearer JWT auth; RBAC per operation; `admin` role implies all.
- JSON request/response bodies.
- List endpoints use cursor/offset pagination (`Page<T>`).
- Timestamps are RFC 3339 UTC.

<!-- code-review-graph MCP tools -->
## MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

### When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes_tool` or `query_graph_tool` instead of Grep
- **Understanding impact**: `get_impact_radius_tool` instead of manually tracing imports
- **Code review**: `detect_changes_tool` + `get_review_context_tool` instead of reading entire files
- **Finding relationships**: `query_graph_tool` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview_tool` + `list_communities_tool`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

### Key Tools

| Tool | Use when |
| ------ | ---------- |
| `detect_changes_tool` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context_tool` | Need source snippets for review — token-efficient |
| `get_impact_radius_tool` | Understanding blast radius of a change |
| `get_affected_flows_tool` | Finding which execution paths are impacted |
| `query_graph_tool` | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes_tool` | Finding functions/classes by name or keyword |
| `get_architecture_overview_tool` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

### Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes_tool` for code review.
3. Use `get_affected_flows_tool` to understand impact.
4. Use `query_graph_tool` pattern="tests_for" to check coverage.
