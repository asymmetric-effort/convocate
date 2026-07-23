# Suggested Commands

## Development
```bash
cd src/ui && bun install && bun run dev    # UI on :8080
cd src/api && go run ./...                 # API on :8443
docker compose up --build                  # Full stack
```

## Build & Test
```bash
make clean   # Remove all containers + build artifacts
make lint    # All linters (Go, TS, SQL, MD, YAML, JSON, Dockerfiles)
make test    # All unit, integration, e2e tests + Playwright
make build   # All container images
make cover   # Coverage check (must be >=98%)
```

## Go-specific
```bash
cd src/api && go test ./...               # API unit tests
cd src/saml-scim-agent && go test ./...   # SAML agent unit tests
cd src/api && go vet ./...                # Static analysis
cd src/api && gofmt -l .                  # Format check
govulncheck ./...                         # Vulnerability scan
```

## CI/CD Status
```bash
curl -s "https://api.github.com/repos/asymmetric-effort/convocate/actions/runs?per_page=5" | jq '.workflow_runs[:5] | .[] | {name: .name, status, conclusion}'
# Or via Cloudflare DNS-over-HTTPS (when dig unavailable):
curl -s "https://1.1.1.1/dns-query?name=HOST&type=A" -H "accept: application/dns-json" | python3 -c "import sys,json; print(json.load(sys.stdin).get('Answer',[{}])[0].get('data','NOT SET'))"
```

## Cluster Access (from svr00)
```bash
ssh samcaldwell@192.168.3.159                              # svr00
kubectl --kubeconfig=/home/samcaldwell/.kube/a get pods -A  # cluster-a
kubectl --kubeconfig=/home/samcaldwell/.kube/b get pods -A  # cluster-b
/tmp/linux-amd64/helm list --kubeconfig=/home/samcaldwell/.kube/a -A  # Helm releases
```

## Service Health
```bash
# API (via Traefik)
curl -sfk --resolve api.dev.convocate.net:443:192.168.3.171 https://api.dev.convocate.net/api/v1/status
# UI (via Traefik)
curl -sfk --resolve app.dev.convocate.net:443:192.168.3.171 https://app.dev.convocate.net/healthz
# saml-scim-agent
curl -sfk https://192.168.3.168:443/health
# OpenBao
curl -sfk https://192.168.3.161:443/v1/sys/seal-status
```
