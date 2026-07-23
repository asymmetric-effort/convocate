# Task Completion Checklist

## Before committing
```bash
cd src/api && gofmt -l .           # Go format check (no output = pass)
cd src/api && go vet ./...         # Go static analysis
cd src/api && go test ./...        # Go unit tests
cd src/saml-scim-agent && go test ./...  # SAML agent tests (if changed)
make lint                          # All linters
make cover                         # Coverage >=98%
```

## Definition of Done (HARD RULES)
- 100% code complete — all specified capabilities built in first pass
- >=98% test coverage — happy + sad paths
- PDV tests pass after deploy on BOTH clusters
- CI/CD green across all affected pipelines
- No secrets in committed code (leakdetector pre-commit hook)
- Both cluster-a (canary) and cluster-b (production) verified

## CI/CD Pipeline Order
1. `Fetch Dependencies` — downloads binaries, creates GitHub release
2. `Docker Images` — builds all container images, promotes to :latest, triggers downstream
3. `Kubernetes Clusters` — provisions K8s infrastructure via Ansible (~30-90 min)
4. `Deploy Applications` — Helm deploy API+UI+PG+Redis to both clusters (~5 min)
5. Service workflows — OpenBao, Grafana, VictoriaLogs, SAML/SCIM Agent, Bootstrap
6. All deploy workflows verify Docker Images passed before proceeding

## Pipeline Monitoring
- Single self-hosted runner — workflows execute sequentially
- K8s Clusters is the longest pipeline (30-90 min)
- Docker Images triggers all downstream via trigger-deploys job
- Duplicate runs occur (push + trigger) — concurrency control on Deploy Apps only
