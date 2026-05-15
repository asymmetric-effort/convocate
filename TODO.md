# convocate v0.2.0 — Implementation Plan

This is the implementation roadmap for the convocate MVP as defined in
README.md. Every item traces back to the architecture and conventions in
README.md and CLAUDE.md. Nothing is invented beyond what those documents
specify.

---

## Phase 0 — Project Skeleton and Build System

- [x] **0.1** Reset `go.mod` to `github.com/asymmetric-effort/convocate` with
      Go 1.26+, zero third-party dependencies (add only as needed and only
      MIT-compatible).
- [x] **0.2** Create directory layout:
      ```
      cmd/
        convocate-router/       # Router API + Web UI + convocate-cli
        convocate-dispatch/     # Per-host dispatch executor
        convocate-secrets-broker/ # Per-container OpenBao socket multiplexer
        convocate-agent-wrapper/  # Agent Container entrypoint
      internal/
        router/                 # Router API business logic
        dispatch/               # Dispatch Service business logic
        broker/                 # Secrets Broker logic
        wrapper/                # Agent wrapper logic
        redis/                  # Redis client (both namespaces)
        openbao/                # OpenBao client integration
        mtls/                   # mTLS certificate utilities
        protocol/               # Shared JSON request/response types
        cli/                    # convocate-cli subcommands
        webui/                  # Web UI SPA (specifyjs framework)
        mock/                   # mock_claude and test doubles
      test/
        e2e/                    # End-to-end tests (mock_claude)
        integration/            # Integration tests
      deploy/
        control-plane/          # Docker Compose + .env.example
        agent-host/             # Docker Compose + .env.example
      .github/
        workflows/
          ci.yml                # CI pipeline
          convocate-dispatch.yml  # Reusable dispatch workflow
          convocate-feedback.yml  # Reusable feedback workflow
      ```
- [x] **0.3** Rewrite `Makefile` with all targets from README.md § Build:
      `clean`, `lint`, `test`, `images`, `build`, `local/start`,
      `local/logs`, `local/stop`, `local/reset`, `release`,
      `release/minor`, `release/major`. `VERSION` set via
      `-ldflags "-X main.Version=$(VERSION)"`.
- [x] **0.4** Create `docker-compose.dev.yml` (repo root) combining
      control-plane and agent-host stacks on a single Compose network with
      local-only defaults (self-signed CA, `.dev/secrets/`,
      localhost-bound ports `8443`/`8444`).
- [x] **0.5** Create `.gitignore`, `.editorconfig`, `.golangci-lint.yml`,
      `.yamllint.yml`, `.markdownlint.json` for the new layout.
- [x] **0.6** Retain `docs/img/*` (architecture diagram). Retain and
      update for v0.2.0: `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`,
      `SECURITY.md` (required by org coding standards Definition of
      Done). Remove stale v1 artifacts: old `Dockerfile`,
      `entrypoint.sh`, `site/`, and any other files not actively used
      by the new version. The branch must be clean and well-organized
      at completion — no orphaned configs, dead code, or leftover v1
      scaffolding.

---

## Phase 1 — Shared Types and Protocol Layer (`internal/protocol/`)

- [x] **1.1** Define JSON request/response structs for all Router API
      endpoints:
      - `POST /v1/jobs` — job submission (from GitHub Action)
      - `GET /v1/dispatch?host=<id>` — long-poll/SSE for dispatch events
      - `POST /v1/status` — per-job status transitions (from Dispatch)
      - `POST /v1/heartbeat` — host health (from Dispatch, every 15s)
      - `GET /v1/health` — Router API health check
      - Web UI management endpoints (projects CRUD, ad-hoc submit,
        cluster auth, container upgrade)
- [x] **1.2** Define state enums:
      - Container states: `provisioning`, `running`, `stopped`,
        `provisioning_failed`, `failed_dispatch`.
      - Job lifecycle states: `claimed`, `running`, `complete`, `failed`,
        `clarifying`, `terminated`.
- [x] **1.3** Define the idempotency key type: `(repository, issue_number,
      run_id)`. `run_attempt` excluded per README.
- [x] **1.4** Define `POST /v1/status` payload: `host_id`, `container_id`,
      `job_id`, `from_state`, `to_state`, `timestamp`, optional `reason`
      and `pr_url`.
- [x] **1.5** Unit tests for serialization round-trips and state-transition
      validation. 90%+ coverage.

---

## Phase 2 — Redis Client (`internal/redis/`)

- [x] **2.1** Redis TLS client (TLS v1.3+, `tcp/6379`). Minimal
      wrapper — no third-party Redis library unless explicitly approved;
      use raw RESP protocol or the Go stdlib.
- [x] **2.2** Router API namespace operations:
      - Container map CRUD (keyed by container ID)
      - Project routing table CRUD (project → `(host, container)`)
      - Repository allowlist (set membership)
      - Job ledger — write with idempotency key, lookup by key, lookup
        by job ID
      - Job metadata — issue number, repo, branch, PR URL, status,
        timestamps
- [x] **2.3** Dispatch namespace operations (keyed by host ID):
      - Host queue read/write
      - In-flight lifecycle state per job
- [x] **2.4** Namespace isolation enforced: Dispatch code cannot read/write
      Router namespace and vice versa.
- [x] **2.5** Unit + integration tests (against real Redis in dev stack).
      90%+ coverage on business-logic paths.

---

## Phase 3 — mTLS Infrastructure (`internal/mtls/`)

- [x] **3.1** Private CA generation at control-plane first start.
- [x] **3.2** `convocate-cli ca print-bundle` — emit trust bundle PEM.
- [x] **3.3** `convocate-cli host issue-cert <host-id>` — issue per-host
      mTLS client cert.
- [x] **3.4** TLS server config (Router API `tcp/8443`): require client cert
      on Dispatch routes, optional on Web UI routes.
- [x] **3.5** TLS client config (Dispatch Service): present host cert,
      verify Router API server cert against the private CA.
- [x] **3.6** Unit tests for cert generation, signing, verification.

---

## Phase 4 — OpenBao Integration (`internal/openbao/`)

- [ ] **4.1** OpenBao client for the Router API: store/read/revoke
      per-project secrets (SSH key, PAT, custom secrets) and shared
      service credentials (Anthropic API key or Claude.ai session token).
- [ ] **4.2** Policy management: when binding project X to container Y on
      host H, push an OpenBao policy authorizing H's AppRole to read
      project-X secrets. Revoking reverses.
- [ ] **4.3** `convocate-cli openbao init` — generate the sealed bootstrap
      key file (mode `0400`).
- [ ] **4.4** Unit tests with a mock OpenBao or the dev-stack OpenBao.

---

## Phase 5 — Secrets Broker (`cmd/convocate-secrets-broker/`, `internal/broker/`)

- [ ] **5.1** Per-container Unix socket multiplexer: one socket per Agent
      Container under `/run/convocate/secrets/`, bind-mounted into
      containers at `/run/convocate/secrets.sock`.
- [ ] **5.2** Map each socket to its bound project via the Router API's
      container map.
- [ ] **5.3** Fetch project secrets from the local OpenBao Agent on read,
      return to caller.
- [ ] **5.4** Socket lifecycle: create on container provision, remove on
      container teardown.
- [ ] **5.5** Unit + integration tests. The broker is the only consumer of
      OpenBao Agent's HTTP API on the host.

---

## Phase 6 — Router API (`cmd/convocate-router/`, `internal/router/`)

- [ ] **6.1** HTTP server on `tcp/443` (GitHub Actions `/v1/jobs`) and
      `tcp/8443` (Web UI + Dispatch mTLS), path-multiplexed.
- [ ] **6.2** `POST /v1/jobs` — bearer-token auth (`CONVOCATE_API_TOKEN`
      per repo), validate repo against allowlist (404 if missing),
      idempotency check, resolve project → container binding, record job,
      dispatch to Dispatch Service, return job ID.
- [ ] **6.3** `GET /v1/dispatch?host=<id>` — long-poll or SSE stream of
      dispatch events targeted at the requesting host. mTLS required.
- [ ] **6.4** `POST /v1/status` — accept per-job state transitions from
      Dispatch Services. Update job metadata in Redis. mTLS required.
- [ ] **6.5** `POST /v1/heartbeat` — accept host health from Dispatch
      Services (container count, CPU%, memory%). Cache latest per host.
      mTLS required.
- [ ] **6.6** `GET /v1/health` — health check endpoint.
- [ ] **6.7** Job submission idempotency: `(repo, issue, run_id)` key.
      Duplicate returns original job ID (HTTP 200). Fresh `run_id` =
      new job.
- [ ] **6.8** `CONVOCATE_API_TOKEN` minting per repo at Create Project time.
      Token only authorizes `/v1/jobs` for its bound repo.
- [ ] **6.9** Container-replacement failure handling: mark project
      `failed_dispatch`, apply `failed_dispatch` label to issue using
      the project's PAT from OpenBao, remove `dispatched` label.
- [ ] **6.10** State recovery on restart: rebuild from Redis.
- [ ] **6.11** Web UI management API endpoints:
      - Create Project (allowlist + OpenBao secrets + mint token + select
        host by lowest container count + provision container)
      - Delete Project (drain → force-terminate option → teardown →
        revoke token → delete secrets → remove routing)
      - Cluster Authentication (Anthropic API key or Claude.ai session
        token → store in OpenBao as shared credential)
      - Ad-hoc job submission (synthetic job ID, no issue number)
      - List projects, job status, agent-fleet health
      - Container upgrade (stop old, provision new image, re-bind)
      - Upgrade All Idle
- [ ] **6.12** Unit tests for every endpoint, idempotency logic, routing
      logic, state transitions. 90%+ coverage.

---

## Phase 7 — Dispatch Service (`cmd/convocate-dispatch/`, `internal/dispatch/`)

- [ ] **7.1** mTLS HTTPS client: long-poll `GET /v1/dispatch?host=<id>` at
      boot, reconnect on drop.
- [ ] **7.2** `POST /v1/status` — report per-job lifecycle transitions
      (`claimed`, `running`, `complete`, `failed`, `clarifying`,
      `terminated`).
- [ ] **7.3** `POST /v1/heartbeat` — every 15 seconds with container count,
      CPU%, memory% from Docker daemon.
- [ ] **7.4** Container provisioning: launch Agent Containers as sibling
      Docker containers, enroll in `convocate-sessions.slice` (90% CPU
      and memory cap). Requires systemd cgroup driver — refuse to start
      without it.
- [ ] **7.5** Prompt delivery: non-blocking delivery to wrapper via stdin.
      Same container handles multiple concurrent prompts.
- [ ] **7.6** Automatic container replacement on dispatch to
      missing/unhealthy container: fetch secrets from broker, provision
      fresh container, register with Router API, retry dispatch. On
      failure → reject back to Router API.
- [ ] **7.7** Per-host Dispatch Redis namespace: queue and in-flight
      lifecycle state, keyed by host ID.
- [ ] **7.8** Force-terminate: signal wrapper to abort a specific background
      task by job ID. Job status → `terminated`.
- [ ] **7.9** Container stop/remove on Delete Project or image upgrade.
- [ ] **7.10** Unit + integration tests. 90%+ coverage on business logic.

---

## Phase 8 — Agent Wrapper (`cmd/convocate-agent-wrapper/`, `internal/wrapper/`)

- [ ] **8.1** Container entrypoint. Read credentials from OpenBao via
      `/run/convocate/secrets.sock`. Write SSH key to `~/.ssh/id_ed25519`
      (mode `0600`) + `known_hosts` for `github.com`.
- [ ] **8.2** Clone or reuse `/workspace/.git`. Per-job worktree at
      `/workspace/jobs/<job-id>` via `git worktree add`.
- [ ] **8.3** Background task acceptance: receive prompts via stdin, prepend
      `Background task: `, pass to long-running Claude Code process.
      Multiple concurrent tasks.
- [ ] **8.4** Per-job feature branch: `fix/issue-<N>` or
      `feature/issue-<N>`. Push over SSH.
- [ ] **8.5** PR creation via `gh` CLI using per-project PAT (`GH_TOKEN`
      env var per subprocess, never written to disk).
- [ ] **8.6** Clarification protocol: post comment with questions, remove
      `automated-development` + `dispatched` labels, reassign to original
      author.
- [ ] **8.7** Success protocol: open PR with `Closes #N`, remove
      `dispatched` label, reassign to author.
- [ ] **8.8** Failure protocol: post failure reason, remove
      `automated-development` + `dispatched` labels, reassign to author.
- [ ] **8.9** Issue comment cadence: one comment per state transition
      (`running`, `clarifying`, `complete`, `failed`, `terminated`).
      No time-based progress pings.
- [ ] **8.10** Branch cleanup on PR merge: delete remote + local branch,
      prune worktree. Keep branches for non-merged PRs.
- [ ] **8.11** Pipeline failure self-healing: detect CI failure via `gh`,
      read logs, apply fix, commit. Max 3 retries, 30-minute wall-clock
      cap. Post diagnostic on exhaustion.
- [ ] **8.12** Liveness heartbeat to local Dispatch Service every 30s.
      Three missed → unhealthy.
- [ ] **8.13** Claude authentication: read shared credential from OpenBao.
      If API key → export `ANTHROPIC_API_KEY`. If session token → run
      refresh loop.
- [ ] **8.14** Ad-hoc job handling: PR body = prompt, no `Closes #N`,
      status streams to Web UI instead of GitHub comments.
- [ ] **8.15** Unit + integration tests. 90%+ coverage on business logic.

---

## Phase 9 — convocate-cli (`internal/cli/`)

- [ ] **9.1** `convocate-cli ca print-bundle` — emit private CA trust
      bundle PEM.
- [ ] **9.2** `convocate-cli host issue-cert <host-id>` — issue mTLS
      client cert for an agent host.
- [ ] **9.3** `convocate-cli openbao init` — generate sealed bootstrap key
      file (mode `0400`).
- [ ] **9.4** CLI is shipped inside the `convocate-router` image, invoked
      via `docker compose exec router convocate-cli ...`.
- [ ] **9.5** Unit tests for each subcommand.

---

## Phase 10 — Web UI (`internal/webui/`)

- [ ] **10.1** SPA using the specifyjs.asymmetric-effort.com UI framework.
      Served by Router API on `tcp/8443` under `/ui/...` prefix.
- [ ] **10.2** Create Project form:
      - Input: repo full name, ed25519 SSH private key, fine-grained PAT,
        custom secrets.
      - On submit: Router API provisions (allowlist + OpenBao + mint token
        + select host + provision container).
      - Display `CONVOCATE_API_TOKEN` once for operator to copy.
      - Display workflow file snippets and config values for the operator.
      - Show `provisioning_failed` state with failed step if provision
        fails.
- [ ] **10.3** Delete Project flow:
      - Drain in-flight jobs (with force-terminate option per job).
      - Teardown container, revoke token, delete secrets, remove routing.
      - Prompt operator for GitHub-side cleanup steps.
- [ ] **10.4** Cluster Authentication form:
      - Anthropic API key mode (paste key).
      - Claude.ai subscription mode (OAuth dance, capture session token).
      - Switch modes forces containers to pick up new credential on next
        task.
- [ ] **10.5** Ad-hoc job submission form: pick project, type prompt,
      submit. Status streams to job-detail view.
- [ ] **10.6** Dashboard views:
      - Project list with container status.
      - Job list with status and links to PRs/issues.
      - Job detail view (state transitions, clarification Qs, failure
        reasons, PR link).
      - Agent-fleet health (heartbeat data: container count, CPU%,
        memory% per host).
      - "Image upgrade available" indicator per project. "Upgrade
        Container" and "Upgrade All Idle" actions.
- [ ] **10.7** No first-party auth — operator fronts with authenticating
      reverse proxy. Router API trusts proxied identity headers from
      configured upstream.
- [ ] **10.8** Playwright tests for all UI flows (Create Project, Delete
      Project, Cluster Auth, ad-hoc submit, dashboard views).

---

## Phase 11 — Agent Container Image

- [ ] **11.1** `Dockerfile.agent` per README.md § Agent Container Image:
      `ubuntu:latest` base (agent containers always use ubuntu:latest
      because they need a full userland — git, curl, openssh-client, gh
      CLI, npm/Claude Code). Install deps, copy
      `convocate-agent-wrapper` binary as entrypoint.
- [ ] **11.2** `CONVOCATE_DEV_MOCK_CLAUDE=1` build-time flag: when set,
      bake `mock_claude` binary into the image instead of real Claude
      Code.
- [ ] **11.3** Volume mount: `convocate-project-<id>` at `/workspace`.
- [ ] **11.4** Secrets socket mount: `/run/convocate/secrets.sock`.

---

## Phase 11.5 — Service Container Images (non-agent)

All non-agent containers use distroless base images (`gcr.io/distroless/static-debian12`
or `/base-debian12` where a libc is needed) for minimal attack surface. Agent containers
are the sole exception — they use `ubuntu:latest` because they require a full userland.

- [ ] **11.5.1** `Dockerfile.router` — multi-stage build: Go build stage
      (`golang:1.26`) → distroless final stage. Copy `convocate-router`
      and `convocate-cli` binaries. Embed Web UI static assets.
- [ ] **11.5.2** `Dockerfile.dispatch` — multi-stage build → distroless.
      Copy `convocate-dispatch` binary.
- [ ] **11.5.3** `Dockerfile.secrets-broker` — multi-stage build →
      distroless. Copy `convocate-secrets-broker` binary.
- [ ] **11.5.4** Verify all three service images run correctly from
      distroless (no shell, no package manager, static or CGO-free
      binaries).

---

## Phase 12 — Docker Compose Stacks

- [ ] **12.1** `deploy/control-plane/docker-compose.yml`:
      `convocate-router` (`tcp/443` + `tcp/8443`), `convocate-redis`
      (`tcp/6379` TLS), `convocate-openbao` (`tcp/8200`). `.env.example`
      with `CONVOCATE_PUBLIC_URL` and `CONVOCATE_OPENBAO_UNSEAL`.
- [ ] **12.2** `deploy/agent-host/docker-compose.yml`:
      `convocate-dispatch`, `convocate-openbao-agent`,
      `convocate-secrets-broker`. `.env.example` with
      `CONVOCATE_CONTROL_URL`, `CONVOCATE_CONTROL_CA`,
      `CONVOCATE_HOST_CERT`, `CONVOCATE_HOST_ID`.
- [ ] **12.3** `docker-compose.dev.yml` (repo root): combine both stacks on
      single Compose network. Self-signed CA on first start,
      `.dev/secrets/`, localhost-bound `8443`/`8444`. Support
      `CONVOCATE_DEV_MOCK_CLAUDE=1` in `.env`.
- [ ] **12.4** Verify targets from README.md § Verify work against dev stack.

---

## Phase 13 — GitHub Actions (Reusable Workflows)

- [ ] **13.1** `.github/workflows/ci.yml` — CI pipeline for convocate
      itself (lint, test, build).
- [ ] **13.2** `.github/workflows/convocate-dispatch.yml` — reusable
      workflow: on `issues.labeled` == `automated-development`, POST
      issue payload to Router API, apply `dispatched` label on 200 OK.
- [ ] **13.3** `.github/workflows/convocate-feedback.yml` — reusable
      workflow: on push to `fix/**` or `feature/**` branches, gate on
      `github.actor == vars.CONVOCATE_BOT_ACCOUNT`, run project test
      suite, report results, trigger ticket updater.
- [ ] **13.4** Unit/integration tests for workflow logic where possible.

---

## Phase 14 — Mock Claude and E2E Tests

- [ ] **14.1** `test/e2e/mock_claude` binary: deterministic responses for
      e2e pipeline testing without Anthropic API credits.
- [ ] **14.2** E2E test suite (`test/e2e/`): exercise the full local dev
      stack end-to-end — submit a job, verify dispatch, verify agent
      creates branch + PR, verify status transitions, verify
      clarification and failure paths.
- [ ] **14.3** Integration test suite (`test/integration/`): test
      inter-component interactions (Router ↔ Redis, Dispatch ↔ Router,
      Broker ↔ OpenBao Agent, Wrapper ↔ Dispatch).
- [ ] **14.4** Playwright post-deployment verification tests for Web UI
      and API endpoints per CLAUDE.md § Rigorous testing.
- [ ] **14.5** Coverage targets: 90%+ on business-logic packages, 98%+
      overall per CLAUDE.md conventions.

---

## Phase 15 — Documentation and Cleanup

- [ ] **15.1** `docs/api/v1.md` — full JSON schema for every Router API
      endpoint (referenced in README.md § Dispatch Service item 6).
- [ ] **15.2** Verify `CLAUDE.md` package map matches the new layout.
- [ ] **15.3** Verify all Makefile targets work end-to-end.
- [ ] **15.4** Final lint pass (`make lint`), full test pass (`make test`),
      image build (`make images`).
- [ ] **15.5** Final artifact audit: walk the repo tree and confirm every
      file is actively used by the new version. Remove any orphaned v1
      artifacts, dead configs, unused dependencies in `go.mod`/`go.sum`,
      stale workflow files, and leftover documentation that no longer
      applies. The branch must ship clean — no file present without a
      purpose.

---

## Implementation Order

Phases are roughly sequential with the following parallelism opportunities:

- **Phase 0** must complete first (skeleton + build system).
- **Phases 1-4** can proceed in parallel (protocol types, Redis, mTLS,
  OpenBao are independent).
- **Phase 5** (Secrets Broker) depends on Phase 4 (OpenBao).
- **Phases 6-8** (Router, Dispatch, Wrapper) depend on Phases 1-5 and
  should be built in order since each layer depends on the one below.
- **Phase 9** (CLI) can proceed alongside Phase 6 (it ships in the
  Router image).
- **Phase 10** (Web UI) depends on Phase 6 (Router API endpoints).
- **Phases 11-12** (images + Compose) depend on Phases 6-8.
- **Phase 13** (GitHub Actions) can proceed alongside Phases 6-8.
- **Phase 14** (E2E) depends on everything above.
- **Phase 15** (cleanup) is last.
