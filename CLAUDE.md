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
| Web UI     | Bun + SpecifyJS | ubuntu:24.04   | distroless    |
| API        | Go 1.26+        | ubuntu:24.04   | distroless    |
| Redis      | —               | ubuntu:24.04   | distroless    |
| PostgreSQL | —               | ubuntu:24.04   | distroless    |
| OpenBao    | —               | ubuntu:24.04   | distroless    |

Storage tiering: file-based JSON first, Redis for key-value/sessions,
PostgreSQL only when relational queries are truly necessary.

## Authoritative Documents

- **API contract**: `openapi.yaml` — the controlling authority for all API
  endpoints, schemas, and RBAC roles.
- **Specification**: `SPECIFICATION.md` — product requirements and domain model.
## Project Layout

```
api/         — Go API server (module: github.com/asymmetric-effort/convocate)
ui/          — Bun + SpecifyJS SPA
docker/      — Dockerfiles (ui, api, redis, pg, openbao)
build/       — build artifacts (gitignored)
img/         — icon assets
```

## Build & Run

```bash
# Local development (no containers needed)
cd ui && bun install && bun run dev    # UI on :8080
cd api && go run ./...                 # API on :8443

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
