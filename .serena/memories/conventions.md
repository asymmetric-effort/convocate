# Code Conventions

## Go
- `gofmt` for formatting, tests in `_test.go` alongside source
- No recursion — use loops and explicit work-list patterns
- ed25519-only SSH keys — no RSA, no ECDSA (exception: SAML signing supports both)
- No mock stores — services fail hard (`log.Fatalf`) if backends unavailable
- API endpoints: `/api/v1/<applet_shortname>/...`
- Applet shortnames: `nmgr`, `amgr`, `pb`, `ide`, `repo`, `ac`, `sup`, `auth`
- Bearer JWT auth (ES256), RBAC per operation, `admin` implies all
- JSON request/response, RFC 3339 UTC timestamps, cursor/offset pagination

## TypeScript
- Bun runtime and bundler
- All UI components use @asymmetric-effort/specifyjs

## Docker
- Multi-stage builds: ubuntu:26.04 build → distroless runtime
- Run as user 65534 (nobody), read-only root filesystem (exceptions: svr00 services run as root for port 443)
- Drop ALL capabilities, add only what's needed
- Third-party binaries: extract from upstream into ubuntu-base (never use pre-built containers)

## Authentication
- API auth via SAML backend proxy: API → saml-scim-agent → OpenBao userpass
- SAML signing: ed25519 (default) or RSA, configurable via SAML_SCIM_AGENT_KEY_ALGORITHM
- JWT issued by API after SAML assertion extraction (ES256)
- Grafana: OIDC directly against OpenBao (exception to SAML rule)

## CI/CD
- All container deploy workflows require Docker Images prereq check
- A/B deploy pattern: deploy-a (canary) → PDV → deploy-b (production) → smoke
- Cluster-a is canary (wipe OK), cluster-b is production (zero-downtime only)
- PDV tests use Playwright, test both happy and sad paths
- Definition of done: 100% code complete, >=98% coverage, PDV pass, CI/CD green
- Docker Images trigger-deploys: maps changed images to downstream workflows

## Secrets
- All secrets in OpenBao, never in env vars or config files
- Tokens in files with 0400 perms, never in env vars
- No secrets in repo — leakdetector in CI + pre-commit hook
- Tests use userpass auth like real users, no root tokens
