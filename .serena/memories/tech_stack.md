# Tech Stack

## Languages & Runtimes
- **Go 1.26+** — API, agent, saml-scim-agent, UI server
- **TypeScript + Bun** — UI SPA (SpecifyJS framework)
- **Python 3** — Ansible, build scripts
- **Bash** — CI/CD, bootstrap scripts

## Approved Dependencies (HARD RULE: nothing else without explicit approval)

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
- Base images: ubuntu:26.04, alpine, distroless ONLY — no pre-built app containers
- All images built via multi-stage Dockerfiles in `infrastructure/docker/`
- Third-party binaries (Traefik, cloudflared) mirrored by extracting binary into ubuntu-base

## Infrastructure Tools
- Ansible — VM provisioning, svr00 service deployment
- Helm 3 — K8s app deployment (umbrella chart with sub-charts)
- Cilium 1.16.6 — K8s CNI (replaces kube-proxy, WireGuard encryption)
- Traefik v3.7 — Ingress controller (Helm chart v41.0.2)
- External Secrets Operator — OpenBao integration
- Playwright — PDV/smoke tests
