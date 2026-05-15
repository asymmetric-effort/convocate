# Repo guidance for Claude Code

This file is read by Claude Code at the start of every session in this repo.
Keep it terse; the goal is to orient you, not to duplicate the source.

## Organization-wide coding standards

**Authoritative reference:** <http://coding-standards.asymmetric-effort.com/>

That site is the source of truth for how code is written across every
asymmetric-effort project. Read it before touching anything; the
project-specific Conventions section below augments it but does not
override it. When this file disagrees with the standards site, the
standards site wins — fix this file.

## What this project is

`convocate` is an automated software-development platform. An engineer
labels a GitHub Issue `automated-development`; convocate dispatches an
isolated, containerized Claude Code agent to implement the solution, run
the CI/CD pipeline, and open a pull request — without human intervention.

**README.md is the architectural source of truth.** Read it before any
non-trivial change to understand the current component model:

- **Router API** — single control-plane container; RESTful HTTPS only;
  owns container map, project routing table, repository allowlist, and
  the job-submission ledger in Redis.
- **Dispatch Service** — one per agent host; long-polls the Router API on
  mTLS HTTPS for new dispatch events; launches Agent Containers as
  sibling Docker containers; only automatic provisioning convocate
  performs is *replacement* of a missing/unhealthy container for an
  already-existing project.
- **Redis** — single container in the control plane; two namespaces
  (Router API authoritative + per-host Dispatch); TLS-only.
- **Secrets Manager (OpenBao)** — unmodified MPL-2.0 OpenBao server in the
  control plane; OpenBao Agent + convocate-built Secrets Broker on each
  agent host; per-container Unix sockets at `/run/convocate/secrets.sock`.
- **Agent Container** — one per project (1 project ≡ 1 repo); long-lived;
  one long-running Claude Code process inside; per-job tasks run as
  background tasks within Claude's task feature, each on its own
  `git worktree`.
- **Web UI** — SPA served by the Router API on `tcp/8443`; owns Create
  Project / Delete Project / Cluster Authentication (Claude) /
  ad-hoc job submission.

There is no auto-provisioning of projects, no SSH control plane, no
cluster-wide bot PAT. Operators drive the lifecycle from the Web UI;
agents talk directly to GitHub and Anthropic.

## Build, test, lint

```bash
make clean          # remove ./build/ AND wipe local dev containers + convocate-* images
make lint           # go vet + yaml lint
make test           # unit + integration + e2e (requires `make local/start` first; uses mock_claude)
make images         # build convocate-router, convocate-dispatch, convocate-agent OCI images
make build          # build everything (calls make images plus other build steps)
make local/start    # bring up the local dev ecosystem via Docker Compose
make local/logs     # tail logs from every dev-stack service
make local/stop     # stop the dev stack (volumes preserved)
make local/reset    # tear down stack + volumes, regenerate the local CA, start clean
make release        # patch bump (commit + tag + push)
make release/minor  # minor bump
make release/major  # major bump
```

- `make release` tags the next version and pushes the tag. `git describe
  --tags --abbrev=0` picks the first reachable tag, so if you've manually
  created tags on the same commit the bump logic can collide — check
  `git tag` if `make release` errors with "tag already exists".

## Package map

The v2 component implementation is in flight; the package layout will
solidify as code lands. Authoritative names for the binaries shipped as
OCI images:

- `convocate-router` — Router API + Web UI + `convocate-cli` admin tool
- `convocate-dispatch` — host-local dispatch executor
- `convocate-secrets-broker` — host-local per-container OpenBao socket
  multiplexer
- `convocate-agent-wrapper` — entrypoint inside each Agent Container

Service-to-service communication, Redis namespaces, secrets ACL, and the
state machines for container and job lifecycle are all defined in
README.md → Architecture. Read that before adding new wire calls or
state values.

## Conventions

- **No recursion.** Go has no tail-call optimization, so every recursive
  call grows the goroutine stack and can panic on adversarial input
  (deep trees, cyclic data, hostile filesystem layouts). For a long-lived
  control plane and long-running Agent Containers, that's unacceptable.
  Use loops, explicit stacks/queues, `filepath.WalkDir`, or work-list
  patterns instead. Mutual recursion (A → B → A) is also forbidden — same
  problem. If you find yourself reaching for a helper that calls itself,
  stop and rewrite as iteration.
- **No speculative abstractions.** Fix what's in front of you; the next
  caller will refactor when it actually shows up.
- **Don't write docstrings explaining what code already says.** Comment
  the *why* when it's non-obvious (hidden invariant, workaround, past
  incident).
- **Tests verify behavior, not implementation.** When you change a
  signature, update callers and the failing tests; don't relax assertions.
- **Coverage targets** — aim for 90%+ in business-logic packages.
  Provisioning paths that require root and real network access are
  excused from automated coverage.
- **No auto-provisioning.** Projects are only created via the Web UI's
  Create Project flow. The Router API never auto-creates a project from
  an inbound `/jobs` call (404 instead). The single exception is
  Dispatch's automatic *replacement* of a missing/unhealthy container
  for an already-existing project — and even that fails to
  `failed_dispatch` on its first error, not retried silently.
- **No cluster-wide GitHub PAT.** Every credential is repo-scoped via
  per-project fine-grained PATs and per-project ed25519 deploy keys.
  A compromised Agent Container can only touch its bound repo.
- **REST only.** No gRPC, no SSH control plane, no per-service custom
  wire protocols. Everything is HTTPS + JSON, with mTLS on internal
  channels.
- **Minimal third-party dependencies.** Prefer writing our own over
  pulling in third-party libraries; adopt a dependency only when
  explicitly approved. Any third-party dependency that ships in product
  code must be MIT or MIT-compatible (Apache-2.0, BSD, and MPL-2.0 for
  unmodified redistribution all qualify — OpenBao is the canonical
  MPL-2.0 example). The Go standard library and dev/build tooling
  (linters, formatters, test scaffolding) are out of scope.

## Things that have tripped us up

- **`make release` picks the first tag** — if a tag for the same commit
  already exists (e.g. from a prior manual tag), the bump can clash.
  Delete the stray tag or tag the next version by hand rather than
  fighting the Makefile.
- **`localhost:443` for the dev stack** — binding to privileged ports on
  a developer workstation requires sudo or rootless-Docker port
  forwarding. Check the dev-stack Compose file for the actual port
  bindings before assuming.
- **sed regex over-match** — wide refactors across tests have historically
  caught test function declarations. When doing bulk renames,
  spot-check the diff for test function signatures.

## Current version

See `git describe --tags` at read time. `Version` is set via
`-ldflags "-X main.Version=$(VERSION)"` in the Makefile and is baked into
the convocate OCI images by `make images`.

Release history (short):

- `v1.0.0` — multi-host orchestration arc (convocate-host + convocate-agent
  deployed; SSH peering; rsyslog TLS).
- `v2.0.0` — "shell is pure client" arc, eventually superseded by the
  control-plane redesign captured in this README.
- **Current (MVP)** — Router API + Redis + OpenBao control plane,
  per-host Dispatch + OpenBao Agent + Secrets Broker, per-project
  long-lived Agent Containers, Web-UI-driven project lifecycle. See
  README.md for the complete picture.
