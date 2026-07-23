# Convocate v2

Agentic software-engineering platform. Monorepo with three primary services + infrastructure.

## Source Map
- `src/api/` — Go REST API server (module: `github.com/asymmetric-effort/convocate`)
- `src/ui/` — Bun + SpecifyJS SPA, served by Go static file server (`src/ui/cmd/serve/`)
- `src/saml-scim-agent/` — Go SAML IdP + SCIM provisioner (standalone module, ed25519+RSA signing)
- `src/agent/` — Go agent (standalone module)
- `infrastructure/` — Ansible, Helm, Dockerfiles, CI/CD

## Infrastructure Layout
- `infrastructure/docker/` — All Dockerfiles (multi-stage, ubuntu:26.04 → distroless)
- `infrastructure/charts/convocate/` — Helm umbrella chart (api, ui, postgresql, redis sub-charts)
- `infrastructure/k8s/` — Ansible roles for K8s provisioning (kubeadm, Cilium, Traefik, ESO, Fluent Bit)
- `infrastructure/bootstrap/` — Ansible roles for svr00 (OpenBao, Grafana, Prometheus, VictoriaLogs, saml-scim-agent, cloudflared)
- `infrastructure/saml-scim-agent/` — Deploy playbook + PDV tests for SAML/SCIM agent
- `.github/workflows/` — CI/CD pipelines

## Key Invariants
- **HARD RULE**: Zero third-party deps without explicit approval. See `mem:tech_stack`.
- **HARD RULE**: No recursion in Go — use loops and work-list patterns.
- **HARD RULE**: All secrets in OpenBao, never in env vars or config files. Tokens in files 0400.
- **HARD RULE**: All services on tcp/443. All SSH keys ed25519-only.
- **HARD RULE**: All Convocate auth via SAML through saml-scim-agent (Grafana exception: OIDC).
- **HARD RULE**: No pre-built app containers. Only ubuntu/alpine/distroless bases.
- **HARD RULE**: cluster-a canary (wipe OK), cluster-b production (zero-downtime only).
- **HARD RULE**: Not done unless both A and B PDV tests pass. CI/CD green.
- Authoritative docs: `openapi.yaml` (API contract), `SPECIFICATION.md` (requirements).

## Deployment
- Two K8s clusters: cluster-a (192.168.3.170-175), cluster-b (192.168.3.180-185)
- svr00 (192.168.3.159): hypervisor, Docker services, self-hosted runner at .90
- Traefik ingress on node-*-1 (hostPort 443), path-based routing for app.{domain}
- Auth flow: SPA → API → saml-scim-agent → OpenBao → SAMLResponse → JWT
- CI/CD: Fetch Deps → Docker Images → {K8s, Deploy Apps, service workflows}
- DNS: Cloudflare public DNS → LAN IPs, accessed via ZTNA tunnel

For commands see `mem:suggested_commands`. For conventions see `mem:conventions`.
