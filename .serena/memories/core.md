# Convocate v2

Agentic software-engineering platform. Monorepo with two primary services + infrastructure.

## Source Map
- `src/api/` — Go REST API server (module: `github.com/asymmetric-effort/convocate`)
- `src/ui/` — Bun + SpecifyJS SPA, served by Go static file server (`src/ui/cmd/serve/`)
- `src/saml-scim-agent/` — Go SAML IdP + SCIM provisioner (standalone module)
- `src/agent/` — Go agent (standalone module)
- `infrastructure/` — Ansible, Helm, Dockerfiles, CI/CD

## Infrastructure Layout
- `infrastructure/docker/` — All Dockerfiles (multi-stage, ubuntu-base → distroless)
- `infrastructure/charts/convocate/` — Helm umbrella chart with sub-charts (api, ui, postgresql, redis, etc.)
- `infrastructure/k8s/` — Ansible roles/playbooks for K8s cluster provisioning (kubeadm, Cilium, Traefik, ESO)
- `infrastructure/bootstrap/` — Ansible roles for svr00 host (OpenBao, Grafana, VictoriaLogs, saml-scim-agent)
- `.github/workflows/` — CI/CD pipelines

## Key Invariants
- **HARD RULE**: Zero third-party deps without explicit approval. See `mem:tech_stack` for approved list.
- **HARD RULE**: No recursion in Go — use loops and work-list patterns.
- **HARD RULE**: All secrets in OpenBao, never in env vars or config files.
- **HARD RULE**: All services on tcp/443, all SSH keys ed25519-only.
- **HARD RULE**: All auth via SAML against OpenBao through saml-scim-agent.
- Authoritative docs: `openapi.yaml` (API contract), `SPECIFICATION.md` (product requirements).

## Deployment
- Two K8s clusters: cluster-a (canary, 192.168.3.170-175), cluster-b (production, 192.168.3.180-185)
- svr00 (192.168.3.159): hypervisor host running Docker services (OpenBao, Grafana, Prometheus, VictoriaLogs, saml-scim-agent)
- Traefik ingress controller on both clusters (hostPort 443 on node-*-1)
- API exposed via Traefik IngressRoute, Traefik terminates TLS
- CI/CD chain: Fetch Dependencies → Docker Images → {K8s Clusters, Deploy Applications, service workflows}

For build/test commands see `mem:suggested_commands`. For code conventions see `mem:conventions`.
