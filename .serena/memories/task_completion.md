# Task Completion Checklist

## Before committing
```bash
cd src/api && gofmt -l .           # Go format check (no output = pass)
cd src/api && go vet ./...         # Go static analysis
cd src/api && go test ./...        # Go unit tests
make lint                          # All linters
make cover                         # Coverage >=98%
```

## Definition of Done
- 100% code complete — all specified capabilities built in first pass
- >=98% test coverage — happy + sad paths
- PDV tests pass after deploy
- CI/CD green on both clusters
- No secrets in committed code (leakdetector runs in pre-commit hook)

## CI/CD Pipeline Order
1. `Fetch Dependencies` — downloads binaries, creates GitHub release
2. `Docker Images` — builds all container images, promotes to :latest
3. `Kubernetes Clusters` — provisions K8s infrastructure (Ansible)
4. `Deploy Applications` — Helm deploy API+PG+Redis to both clusters
5. Service workflows (OpenBao, Grafana, VictoriaLogs, saml-scim-agent) — triggered by Docker Images
