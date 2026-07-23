# Convocate v2 — Project Instructions

## Overview

Convocate is an agentic software-engineering platform. This is a complete v2
rewrite. The repository is a monorepo with three primary services:

- **Web UI** — TypeScript + Bun + @asymmetric-effort/specifyjs SPA
- **API** — Go 1.26+ REST server
- **SAML/SCIM Agent** — Go SAML IdP + SCIM provisioner backed by OpenBao

## Architecture

### Application Containers (K8s)

| Container  | Language/Runtime | Build Stage    | Runtime Stage |
|------------|-----------------|----------------|---------------|
| Web UI     | Bun + SpecifyJS | ubuntu:26.04   | distroless    |
| API        | Go 1.26+        | ubuntu:26.04   | distroless    |
| Redis      | —               | ubuntu:26.04   | distroless    |
| PostgreSQL | —               | ubuntu:26.04   | distroless    |

### Infrastructure Services (svr00 Docker)

| Service           | IP               | Purpose                              |
|-------------------|------------------|--------------------------------------|
| OpenBao secrets-a | 192.168.3.160    | Secret store (dev/canary)            |
| OpenBao secrets-b | 192.168.3.161    | Secret store (production)            |
| Grafana-a         | 192.168.3.162    | Monitoring dashboards (dev)          |
| Grafana-b         | 192.168.3.163    | Monitoring dashboards (prod)         |
| Prometheus-a      | 192.168.3.164    | Metrics collection (dev)             |
| Prometheus-b      | 192.168.3.165    | Metrics collection (prod)            |
| VictoriaLogs-a    | 192.168.3.166    | Log aggregation (dev)                |
| VictoriaLogs-b    | 192.168.3.167    | Log aggregation (prod)               |
| saml-scim-agent-a | 192.168.3.168    | SAML/SCIM auth proxy (dev)           |
| saml-scim-agent-b | 192.168.3.169    | SAML/SCIM auth proxy (prod)          |

### K8s Clusters

| Cluster   | Nodes              | IPs                  | Role       |
|-----------|--------------------|----------------------|------------|
| cluster-a | node-a-0 through 5 | 192.168.3.170–175    | Canary     |
| cluster-b | node-b-0 through 5 | 192.168.3.180–185    | Production |

- **Traefik** ingress controller on node-*-1 (hostPort 443), hostname-based routing
- **Cilium** CNI with WireGuard encryption, replaces kube-proxy
- **External Secrets Operator** for OpenBao integration

### Authentication Flow

```
Browser → SPA login form → POST /api/v1/auth/login {username, password}
  → API calls saml-scim-agent /saml/login (SAML backend proxy)
  → saml-scim-agent authenticates against OpenBao userpass
  → Returns SAMLResponse with assertions (ed25519 or RSA signed)
  → API extracts identity, mints JWT (ES256)
  → Returns JWT to SPA
  → SPA stores in localStorage, sends as Bearer token on all requests
```

Falls back to direct OpenBao userpass if `SAML_SCIM_AGENT_URL` not set.

### Routing (Traefik)

```
app.dev.convocate.net/           → UI service (SPA)
app.dev.convocate.net/api/v1/*   → API service (priority routing)
api.dev.convocate.net/*          → API service (direct)
```

Same pattern on cluster-b with `prod.convocate.net`.

### DNS Layout

| FQDN                            | IP              | Service                |
|----------------------------------|-----------------|------------------------|
| api.dev.convocate.net            | 192.168.3.171   | API (cluster-a Traefik)|
| app.dev.convocate.net            | 192.168.3.171   | UI (cluster-a Traefik) |
| api.prod.convocate.net           | 192.168.3.181   | API (cluster-b Traefik)|
| app.prod.convocate.net           | 192.168.3.181   | UI (cluster-b Traefik) |
| auth.asymmetric-effort.com       | 192.168.3.161   | OpenBao secrets-b      |
| dev-auth.asymmetric-effort.com   | 192.168.3.160   | OpenBao secrets-a      |
| grafana.asymmetric-effort.com    | 192.168.3.163   | Grafana-b              |
| dev.grafana.asymmetric-effort.com| 192.168.3.162   | Grafana-a              |
| saml.asymmetric-effort.com       | 192.168.3.169   | saml-scim-agent-b      |
| dev.saml.asymmetric-effort.com   | 192.168.3.168   | saml-scim-agent-a      |

All DNS is Cloudflare public DNS pointing to internal LAN IPs, accessed via
Cloudflare ZTNA tunnel through cloudflared on svr00.

Storage tiering: file-based JSON first, Redis for key-value/sessions,
PostgreSQL only when relational queries are truly necessary.

## Authoritative Documents

- **API contract**: `openapi.yaml` — the controlling authority for all API
  endpoints, schemas, and RBAC roles.
- **Specification**: `SPECIFICATION.md` — product requirements and domain model.

## Project Layout

```
src/api/              — Go API server (module: github.com/asymmetric-effort/convocate)
src/agent/            — Go agent (standalone module)
src/saml-scim-agent/  — Go SAML IdP + SCIM provisioner (standalone module)
src/ui/               — Bun + SpecifyJS SPA
  src/ui/cmd/serve/   — Go static file server for SPA
  src/ui/img/         — icon assets
infrastructure/
  bootstrap/          — Ansible roles for svr00 host services
  charts/             — Helm umbrella chart with sub-charts (api, ui, pg, redis, etc.)
  docker/             — Dockerfiles (all services)
  grafana/            — Grafana deploy playbook + provisioning
  k8s/                — Ansible roles/playbooks for K8s cluster provisioning
  prometheus/         — Prometheus deploy playbook
  saml-scim-agent/    — SAML/SCIM agent deploy playbook + PDV tests
  secrets/            — OpenBao deploy playbook + PDV tests
  victorialogs/       — VictoriaLogs deploy playbook + PDV tests
.github/workflows/    — CI/CD pipelines
test/pdv/             — PDV tests
build/                — build artifacts (gitignored)
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

## Hard Rules

These rules are non-negotiable. Do not weaken, bypass, or work around them.

### Dependencies
- **Zero third-party dependencies** unless explicitly approved (see Approved Dependencies below).
- **No pre-built app containers** — only ubuntu/alpine/distroless base images. Third-party binaries (Traefik, cloudflared) must be extracted into approved base images.

### Security
- **All secrets in OpenBao** — never in env vars, config files, or repo.
- **Tokens in files with 0400 perms** — never in environment variables.
- **No secrets in repo** — leakdetector runs in CI + pre-commit hook. All secrets generated programmatically.
- **All tests use userpass auth** like real users — no root tokens in tests.
- **ed25519-only SSH keys** — all generated SSH keys must be ed25519. No RSA, no ECDSA. (SAML signing supports both ed25519 and RSA for interoperability.)
- **All auth via SAML** through saml-scim-agent against OpenBao — no direct OIDC, no bypasses (exception: Grafana uses OIDC directly against OpenBao).
- **All VMs get owner's GitHub SSH keys** (sam-caldwell.keys) + CI key.

### Deployment
- **All services on tcp/443** — standard HTTPS port, no non-standard ports.
- **Cluster-a is canary** (wipe OK), **cluster-b is production** (zero-downtime only).
- **Zero-downtime CI deploys only** — no destructive ops in deploy workflows.
- **Not done unless both A and B PDV tests pass** — deployment is incomplete until verified on both clusters.
- **All fixes via CI/CD** — no manual SSH, no manual pipeline triggers unless user explicitly asks.
- **Every feature needs Playwright PDV** after deploy — done = deploy + PDV pass.
- **All container deploy workflows require Docker Images prereq** — deployment cannot proceed unless latest Docker Images pipeline passed.

### Code Quality
- **100% code complete** — build ALL specified capabilities in first pass, don't iterate.
- **>=98% test coverage** — happy + sad paths.
- **No recursion in Go** — use loops and explicit work-list patterns.
- **CIS benchmark hardening** required on svr00 bootstrap.

### UI
- Use published npm @asymmetric-effort/specifyjs — never clone/modify/vendor.

## Coding Standards

**Authoritative reference:** <http://coding-standards.asymmetric-effort.com/>

### Go

- No recursion — use loops and explicit work-list patterns.
- ed25519-only SSH keys.
- Use `gofmt` for formatting.
- Tests go in `_test.go` files alongside the code they test.

### TypeScript

- Use Bun as the runtime and bundler.
- All UI components use @asymmetric-effort/specifyjs.

## Approved Dependencies

| Package                              | Language | Purpose            |
|--------------------------------------|----------|--------------------|
| `@asymmetric-effort/specifyjs`       | TS       | UI framework       |
| `@asymmetric-effort/nogginlessdom`   | TS       | DOM library        |
| `@asymmetric-effort/yamllint`        | TS       | YAML linter        |
| `@asymmetric-effort/jsonlint`        | TS       | JSON linter        |
| `redis/go-redis`                     | Go       | Redis client       |
| `jackc/pgx`                          | Go       | PostgreSQL driver  |
| `openbao/openbao`                    | Go       | Secret store client|
| `k8s.io/client-go`                   | Go       | Kubernetes API     |

Everything else must use language standard libraries. Do not add any
dependency — including test frameworks, linters, or utility packages —
without explicit approval.

## API Conventions

- All endpoints: `/api/v1/<applet_shortname>/...`
- Applet shortnames: `nmgr`, `amgr`, `pb`, `ide`, `repo`, `ac`, `sup`, `auth`
- Event channels: `/api/v1/events/{applet}/{channel}` (WebSocket)
- Agent shell: `/api/v1/amgr/agent/{agentId}/shell` (WebSocket)
- Bearer JWT auth (ES256); RBAC per operation; `admin` role implies all.
- JSON request/response bodies.
- List endpoints use cursor/offset pagination (`Page<T>`).
- Timestamps are RFC 3339 UTC.

## CI/CD Pipeline Architecture

```
Fetch Dependencies → Docker Images → {
  K8s Clusters (infrastructure)
  Deploy Applications (API + UI + PG + Redis via Helm)
  OpenBao (svr00 Docker)
  Grafana (svr00 Docker)
  VictoriaLogs (svr00 Docker)
  SAML/SCIM Agent (svr00 Docker)
  Bootstrap svr00 (full host config)
}
```

Docker Images `trigger-deploys` job detects which Dockerfiles changed and
dispatches only the relevant downstream workflows. All downstream workflows
verify Docker Images passed before deploying.

Deploy pattern: deploy-a (canary) → PDV → deploy-b (production) → smoke.

## Infrastructure

- **svr00** (192.168.3.159): hypervisor host, Docker services, self-hosted runner
- **Self-hosted runner** (192.168.3.90): GitHub Actions runner in Docker container
- **Cloudflared tunnel** on svr00: Cloudflare ZTNA access to all services
- **Ansible**: VM provisioning, K8s bootstrap, svr00 service deployment
- **Helm 3**: K8s application deployment (umbrella chart with sub-charts)
- **Cilium 1.16.6**: K8s CNI, WireGuard encryption, host firewall
- **Traefik v3.7**: Ingress controller (Helm chart v41.0.2)
- **External Secrets Operator**: OpenBao integration for K8s secrets
