# Code Conventions

## Go
- `gofmt` for formatting, tests in `_test.go` alongside source
- No recursion — use loops and explicit work-list patterns
- ed25519-only SSH keys — no RSA, no ECDSA
- No mock stores — services fail hard (`log.Fatalf`) if backends unavailable
- API endpoints: `/api/v1/<applet_shortname>/...`
- Applet shortnames: `nmgr`, `amgr`, `pb`, `ide`, `repo`, `ac`, `sup`, `auth`
- Bearer JWT auth, RBAC per operation, `admin` implies all
- JSON request/response, RFC 3339 UTC timestamps, cursor/offset pagination

## TypeScript
- Bun runtime and bundler
- All UI components use @asymmetric-effort/specifyjs
- No other UI frameworks or libraries

## Docker
- Multi-stage builds: ubuntu:26.04 build stage → distroless runtime
- Run as user 65534 (nobody), read-only root filesystem
- Drop ALL capabilities, add only what's needed
- Third-party binaries: extract from upstream image into ubuntu-base (not use directly)

## CI/CD
- All container deploy workflows require Docker Images prereq check
- A/B deploy pattern: deploy-a (canary) → PDV → deploy-b (production) → smoke
- Cluster-a is canary (wipe OK), cluster-b is production (zero-downtime only)
- PDV tests use Playwright, test both happy and sad paths
- Definition of done: 100% code complete, >=98% coverage, PDV pass, CI/CD green

## Secrets
- All secrets in OpenBao, never in env vars or config files
- Tokens in files with 0400 perms, never in env vars
- No secrets in repo — leakdetector in CI + pre-commit hook
