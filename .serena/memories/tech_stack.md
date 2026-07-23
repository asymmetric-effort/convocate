# Tech Stack

## Languages & Runtimes
- **Go 1.26+** — API, agent, saml-scim-agent, UI server
- **TypeScript + Bun** — UI SPA (SpecifyJS framework)
- **Python 3** — Ansible, build scripts
- **Bash** — CI/CD, bootstrap scripts

## Approved Dependencies (HARD RULE: nothing else without approval)

| Package | Language | Purpose |
|---------|----------|---------|
| `@asymmetric-effort/specifyjs` | TS | UI framework |
| `@asymmetric-effort/nogginlessdom` | TS | DOM library |
| `@asymmetric-effort/yamllint` | TS | YAML linter |
| `@asymmetric-effort/jsonlint` | TS | JSON linter |
| `redis/go-redis` | Go | Redis client |
| `jackc/pgx` | Go | PostgreSQL driver |
| `openbao/openbao` | Go | Secret store client |
| `k8s.io/client-go` | Go | Kubernetes API client |

## Container Images
- Base images: ubuntu:26.04, alpine, distroless ONLY
- Third-party binaries (Traefik, cloudflared) mirrored by extracting into ubuntu-base
- All images built via multi-stage Dockerfiles in `infrastructure/docker/`

## Infrastructure
- Ansible — VM provisioning, svr00 service deployment
- Helm 3 — K8s app deployment (umbrella chart)
- Cilium 1.16.6 — CNI, WireGuard encryption, replaces kube-proxy
- Traefik v3.7 (chart v41.0.2) — Ingress controller, TLS termination
- External Secrets Operator — OpenBao integration
- Playwright — PDV/smoke tests
- Cloudflare ZTNA — tunnel via cloudflared on svr00
