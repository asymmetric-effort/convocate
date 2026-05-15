# convocate

[![CI][ci-badge]][ci-link]
[![Coverage][cov-badge]][ci-link]

[ci-badge]: https://github.com/asymmetric-effort/convocate/actions/workflows/ci.yml/badge.svg
[ci-link]: https://github.com/asymmetric-effort/convocate/actions/workflows/ci.yml
[cov-badge]: https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/asymmetric-effort/convocate/badges/.badges/coverage.json

An automated software development platform that receives GitHub Issues and delivers working code.
Apply the `automated-development` label to an issue and convocate dispatches an isolated,
containerized Claude Code agent to implement the solution, run the CI/CD pipeline, and open a
pull request — without human intervention.

📖 **Full documentation: <https://convocate.asymmetric-effort.com>**

---

## How It Works

1. An engineer creates a GitHub Issue describing a feature or bug.
2. They affix a 'automated-development' label to the issue.
3. A GitHub Action fires and sends the issue payload to the convocate Router API.
4. The Router API checks the idempotency ledger: a retried submission with a matching
   `(repo, issue, run_id)` key is short-circuited and the original job ID is returned (HTTP
   200, no second dispatch). The Router API then resolves the project's bound Agent Container.
   **If no project has been created via the Web UI for this repository, the Router API responds
   HTTP 404 — projects are never auto-provisioned from a GitHub Actions submission.** Otherwise
   it records the job and returns a new job ID.
5. On 200 OK, the GitHub Action applies the `dispatched` label to the issue and exits. From that
   point, convocate owns the work. The `dispatched` label stays on the issue only while convocate
   is actively driving it: the **agent** removes the label on PR open, on clarification (issue
   handed back to the original author), and on failure. On `failed_dispatch` no agent ever ran,
   so the **Router API** removes `dispatched` and applies `failed_dispatch` itself, using the
   project's per-project PAT. If convocate isn't currently working the issue, the label is off.
6. The Dispatch Service delivers the prompt to the bound Agent Container as a background task —
   the container may already be working on other issues for the same project and starts this
   one in parallel. The agent — running a Go wrapper, Claude Code, and the `gh` CLI — clones the
   target repository (or reuses its existing worktree), creates a per-job feature branch,
   implements the solution, runs tests, and opens a pull request. If the issue is ambiguous,
   the agent posts a clarification comment, removes the `automated-development` label, and
   assigns the issue back to the original author. When the PR is merged, the CI/CD pipeline
   updates the issue status automatically.

Engineers interact with GitHub Issues the way they always have. Convocate handles everything
between the label and the pull request.

---

## Architecture

![convocate diagram](docs/img/convocate%20diagram.png)

### Components

#### Router API

This is the control plane and routing authority. It runs as a single container in the control
plane stack (alongside Redis and the OpenBao server) — one instance per cluster, no HA in
MVP. A Router API restart picks up state from Redis on boot; in-flight Dispatch long-polls
reconnect automatically. High-availability (active-passive or active-active behind a load
balancer) is post-MVP. It receives job submissions from GitHub Actions and serves ad-hoc
submissions, realtime status queries, and system-management calls from the Web UI. The Router
API exposes a single RESTful HTTPS surface — there is no gRPC, no SSH control plane, no
per-service custom wire protocol. For every accepted submission it picks the `(host,
container)` pair that will run the work, dispatches the prompt to the Dispatch Service on that
host over a long-lived mTLS HTTPS channel, and returns a job ID to the caller.

The Router API owns four pieces of state in Redis:

1. **Container map** — every Agent Container that exists, the host it lives on, its assigned
   project, and its current run state. The canonical state set is `provisioning`, `running`,
   `stopped`, `provisioning_failed`, `failed_dispatch`. The Router API is the single writer;
   per-host Dispatch Services report state transitions back through the API.
2. **Project routing table** — a strict project → `(host, container)` binding established by
   the Web UI's Create Project flow. Once a project is bound to a container on a host, every
   subsequent job for that project routes to that exact container on that exact host. Tasks
   for a given project are never sent to a different container or a different host, so build
   context and any sensitive data the agent has touched stay inside the boundary they were
   loaded into. If the bound container is missing or unhealthy at dispatch time, the local
   Dispatch Service attempts an automatic replacement using the project's stored secrets (see
   Dispatch Service item 4). On success, the Router API updates the container map and the
   project remains routable. On failure, the Router API marks the project `failed_dispatch`,
   applies the `failed_dispatch` label to the originating issue (using the project's
   per-project PAT), and surfaces the failure in the Web UI for operator intervention.
3. **Repository allowlist** — the set of repository full names eligible to submit jobs.
   Populated only by Create Project; consulted on every `/jobs` request; misses produce
   HTTP 404.
4. **Job-submission ledger** — every accepted submission is recorded with an idempotency key
   `(repository, issue number, workflow run_id)`. `run_attempt` is deliberately excluded, so
   workflow-internal retries (same run, higher attempt number) dedupe to the original job
   ID and never produce a second dispatch. Re-applying the `automated-development` label
   generates a fresh run_id and a fresh job — that is the intended retry path.

A bound container may have several jobs in flight at once. The Router API does not serialize
submissions per container — it hands each accepted prompt to the Dispatch Service as a background
task, and the container's wrapper runs them concurrently. Concurrency is bounded only by the
container's own resource limits and the host slice's CPU/memory cap.

The Router API is the single writer for job metadata — issue number, repository, branch name, PR
URL, current status, and timestamps. No other component writes to the Router API's Redis namespace
directly.

#### Dispatch Service

1. The on-host container executor. **A Dispatch Service runs on every agent host**, alongside the
   containers it manages.
2. It consumes dispatch events targeted at its host from the Router API and delivers the prompt
   to the local Agent Container the Router API has already selected.
3. Delivery is non-blocking—the wrapper accepts the prompt then passes it via stdin to Claude as
   a background task and immediately acknowledges, so the same container can be handed multiple
   prompts in rapid succession without serializing.
4. When a dispatch targets a container that has gone missing or unhealthy, the local Dispatch
   Service attempts a single automatic replacement: it fetches the project's secrets via the
   Secrets Broker, provisions a fresh container on the same host, registers it with the
   Router API, and retries the dispatch against the new container. **If the replacement
   provisioning itself fails (resource exhaustion, image pull error, OpenBao unavailable,
   etc.), the Dispatch Service rejects the dispatch back to the Router API**, which then
   marks the project `failed_dispatch`, applies the `failed_dispatch` label to the issue, and
   stops accepting new submissions for that project until the operator intervenes via the Web
   UI. Container replacement on dispatch is the only automatic provisioning convocate performs
   — projects themselves are still never auto-created.
5. A Dispatch Service never makes routing decisions, never starts containers on other hosts, and
   never sees jobs for projects bound to a different host — it acts only on the routing
   decisions the Router API has already made for its own host. It manages in-container job
   lifecycle transitions — `claimed`, `running`, `complete`, `failed`, `clarifying`,
   `terminated` — per background task, and persists its host's queue and lifecycle state to a
   dedicated Dispatch namespace on the central Redis (keyed by host ID, never overlapping the
   Router API namespace). Durable job metadata still lives only in the Router API namespace.
   Agent Containers communicate exclusively through their local Dispatch Service.
6. The Dispatch Service uses the same RESTful HTTPS surface as the rest of convocate: it
   long-polls (or SSE-streams) `GET /v1/dispatch?host=<id>` on the Router API for new dispatch
   events, `POST /v1/status` for per-job status transitions, and `POST /v1/heartbeat` every 15
   seconds with the host's current container count, CPU%, and memory% sampled from the Docker
   daemon. The Router API caches the latest heartbeat per host and the Web UI renders it as
   agent-fleet health. All three endpoints sit on the Router API's mTLS-protected `tcp/8443`
   listener. `POST /v1/status` payloads carry, at minimum, `host_id`, `container_id`,
   `job_id`, `from_state`, `to_state`, `timestamp`, and an optional `reason` or `pr_url`
   field. The full JSON schema for every Router API endpoint will land in `docs/api/v1.md`
   alongside the v1 implementation.

#### Redis

A single Redis container runs in the control plane stack. It holds two separate keyspaces:

- The **Router API's authoritative namespace** — container registry, project routing table,
  repository allowlist, job ledger, and job metadata. The Router API is the single writer.
- The **Dispatch Services' namespace** — host-local queue and in-flight lifecycle state, keyed
  by host ID. Each per-host Dispatch Service is the single writer for its own host's keys and
  never reads or writes the Router API namespace.

Redis is reachable from agent hosts (for Dispatch) and from the control plane (for the Router
API), but not from Agent Containers or any external system. Traffic must be encrypted in
flight using TLS v1.3+; Redis is exposed on `tcp/6379` with native TLS.

#### Secrets Manager (OpenBao)

The credential broker. Convocate uses **OpenBao** ([openbao.org](https://openbao.org)) — the
MPL-2.0 Linux Foundation fork of HashiCorp Vault — as its secret store, shipped **unmodified**
(no source-disclosure obligation; only the upstream LICENSE/NOTICE ride along).

- **OpenBao server** runs as a container in the control plane stack alongside the Router API.
  It holds:
    - **Per-project credentials**: a fine-grained GitHub PAT scoped to the project's repository,
      the project's ed25519 SSH keypair (matching its deploy key), and any custom secrets the
      operator loads. These are served only to the project's bound Agent Container.
    - **Shared service credentials**: the cluster's Claude authentication (either an Anthropic
      API key or a captured Claude.ai subscription session token), plus any other shared
      service keys the operator loads. Served to every Agent Container that needs them.

  Convocate stores no bot-account-wide GitHub PAT — the Router API never calls the GitHub API
  on the operator's behalf, so it has no need for cross-repo credentials. Storage uses
  integrated Raft persisted on a Docker volume mounted at `/var/lib/openbao`. Auto-unseal at
  boot uses a sealed bootstrap key on a local file (mode `0400`, owned by the OpenBao
  container's user). The operator generates this file once at install with `convocate-cli
  openbao init` and stores it out-of-band; the file is supplied to the container as a Docker
  secret. There is no automatic KMS integration in MVP — bootstrap-key safekeeping is the
  operator's responsibility.
- **OpenBao Agent** runs on every agent host as a container in the host's local stack
  (upstream binary, unmodified). It authenticates to the central OpenBao via an AppRole bound
  to that host's identity and caches scoped tokens for the host's containers.
- **Secrets Broker** is a small convocate-built sidecar that sits in front of OpenBao Agent on
  every agent host. It owns one Unix socket per Agent Container (created at container
  provisioning and bind-mounted into the container at `/run/convocate/secrets.sock`), maps
  each socket back to its bound project via the Router API's container map, fetches that
  project's secrets from OpenBao Agent on read, and returns them to the caller. The broker is
  the only consumer of OpenBao Agent's HTTP API on the host; Agent Containers never reach
  OpenBao Agent or the central OpenBao directly.

The Router API treats its project routing table as the secrets ACL: when it binds project X to
container Y on host H, it pushes a policy update to OpenBao (authorizing H's AppRole to read
project-X secrets) and tells host H's Secrets Broker that socket Y maps to project X. Revoking
the binding reverses both. A container can never read secrets for a project it is not bound to
— the secrets boundary follows the routing boundary by construction.

#### Agent containers

1. Long-lived Docker containers, **one per project** — and a project is exactly one GitHub
   repository, always. The SSH keypair, repo-scoped PAT, routing entry, Agent Container, and
   workspace volume are all 1:1:1:1:1 with the repo. Multi-repo projects are not supported.
2. A container is provisioned by the Web UI's Create Project flow before any job submission and
   is reused for every subsequent job from that project — build caches, cloned worktrees, and
   accumulated Claude Code context all persist across jobs.
3. There is no shared state with any other project's container. Each container runs three components
   as a unit: a Go wrapper process (the entrypoint), Claude Code, and the `gh` CLI.
4. Each container mounts a named Docker volume `convocate-project-<id>` at `/workspace` for
   disk persistence. The volume holds one primary clone (`/workspace/.git`) plus a per-job
   worktree directory at `/workspace/jobs/<job-id>` created by `git worktree add` and pruned
   only when the container is retired. Volumes survive image-upgrade container replacements
   and are deleted only by Delete Project.
5. The Go wrapper accepts prompts as **background tasks**: when the Dispatch Service delivers a new
   prompt, the wrapper passes the prompt via stdin to its long-running claude process without blocking
   the container's other in-flight work. The wrapper prepends the literal string `Background task:`
   (followed by a single space) to each prompt — this is a convocate convention that signals Claude
   to route the message into its task-feature flow rather than treating it as a continuation of any
   in-progress reasoning.
6. Multiple issues for the same project run concurrently inside one container as independent
   background tasks within Claude's long-running process. Each task gets its own git worktree
   and feature branch, and the wrapper tracks a per-job status stream against it.
7. The same background-task grammar is also what Claude itself uses to delegate work to its
   sub-agents: when Claude invokes its task tool inside the container, the wrapper accepts the
   sub-agent prompt on the same entrypoint, identifies it by the parent job's ID, and runs it
   alongside the parent task in the shared Claude process.
8. For each background task the wrapper constructs the prompt from the issue payload, invokes Claude
   Code, handles the clarification protocol, pushes the branch, opens the PR, and reports per-job
   status to the Dispatch Service as it transitions. The wrapper never exits while any background
   task is in flight; the container itself is torn down only when the Router API retires it (image
   upgrade, project removed, unhealthy).
9. Each Agent Container reads the cluster's Claude authentication at start (see Web UI →
   Cluster Authentication): either an Anthropic API key it exports as `ANTHROPIC_API_KEY`,
   or a Claude.ai subscription session token. When using a session token, the wrapper runs a
   refresh loop to keep the session live; when using an API key, the loop is a no-op.
10. The wrapper POSTs a liveness heartbeat to its local Dispatch Service every 30 seconds. Three
    consecutive missed heartbeats flip the container to **unhealthy**; the Router API surfaces
    the project in the Web UI and Dispatch's next dispatch to this container triggers the
    replace-or-fail flow from Dispatch Service item 4.

**Provisioning.** A project's Agent Container is provisioned by the Router API at create-project
time (see the Web UI's project lifecycle flow), using the SSH key, repo-scoped PAT, and custom
secrets the operator uploaded; the container is warm before the first job. Convocate never
auto-provisions a project from an inbound job submission, and never silently re-provisions on
its own — container creation, replacement, and teardown are always operator-initiated through
the Web UI.

At container start the wrapper reads its credentials from OpenBao via the per-container secrets
socket and writes the SSH private key to `~/.ssh/id_ed25519` (mode `0600`) with a `known_hosts`
entry for `github.com`. Git operations (`clone`, `fetch`, `push`) run over SSH
(`git@github.com:org/repo.git`). The `gh` CLI uses the **project's repo-scoped PAT** (also
fetched from the secrets socket) for direct GitHub API calls — PR creation, issue labelling,
reassignment. The wrapper exposes the PAT via the `GH_TOKEN` environment variable on each
per-task subprocess invocation; the token is never written to `~/.config/gh/hosts.yml` or any
on-disk credential store. Agents talk to GitHub and the Anthropic API directly with
credentials from OpenBao; no GitHub or Anthropic traffic is proxied through the Router API,
so the control plane never sits on the data path that carries job payloads. (Status
transitions and heartbeats do flow through the Router API, but they are small JSON messages
on a separate channel.)

On container retirement (image upgrade, project removed, unhealthy), the Router API clears the
project's PAT and secrets from OpenBao. The operator removes the deploy key from GitHub manually
as part of project teardown — convocate never calls GitHub's admin API on the operator's behalf.

#### Web UI

Provides system management, live job status, agent health monitoring, and an ad hoc console for
manual job submission. It communicates exclusively with the Router API — it never touches Redis,
OpenBao, or the Dispatch Service directly.

**Project lifecycle (MVP).** The Web UI owns the operator-facing flow for spinning a project up
and tearing it down end-to-end. Two actions cover the lifecycle: **Create Project** and **Delete
Project**.

***Create Project.*** A single create-project action does the following:

1. **Operator input** — enter the repo full name and upload the project's credentials:
    - the project's ed25519 SSH **private** key (operator generates the keypair themselves and
      registers the matching public key as a deploy key on the GitHub repo),
    - a fine-grained PAT scoped to that one repo (contents/PR/issues write),
    - any custom per-project secrets the wrapper or Claude Code prompts need.
2. **Router API provisioning** — the Router API writes the repo to the allowlist, stores the
   submitted secrets in OpenBao under the project's namespace, mints `CONVOCATE_API_TOKEN` for
   that repo, picks the agent host with the **lowest count of currently-running Agent
   Containers** (ties broken by host ID), and instructs that host's Dispatch Service to
   provision the project's Agent Container immediately. The container is up and warm before
   the first GitHub Issue is labeled — no cold-start latency on the first dispatch.
3. **Operator GitHub-side setup** — the UI displays `CONVOCATE_API_TOKEN` once for the operator
   to copy into the repo's Actions secrets, alongside the dispatch and feedback workflow files
   to drop into `.github/workflows/`.

If any step in (2) fails (e.g. the chosen host's Dispatch Service rejects the provision call),
the Router API records the project in a `provisioning_failed` state with the failed step
highlighted in the Web UI. There is no automatic rollback — the operator can retry from the
failed step, or run Delete Project to unwind the partial state. Steps that already succeeded
(allowlist entry, OpenBao secrets, minted token) stay in place until one of those operator
actions runs.

***Delete Project.*** A single delete-project action reverses Create end-to-end:

1. **Drain** — the Router API stops accepting new submissions for the repo (subsequent calls
   get HTTP 404) and waits for any in-flight jobs to finish. The operator can **force-terminate**
   a running job from the UI: the Router API instructs the bound host's Dispatch Service,
   which signals the wrapper to abort the matching Claude background task. The Claude task
   stops mid-stream; the per-job branch and worktree are left as-is on the volume for later
   inspection, and the job's status flips to `terminated`. Other in-flight jobs in the same
   container are unaffected.
2. **Tear-down** — the Router API instructs the bound host's Dispatch Service to stop and
   remove the Agent Container, revokes `CONVOCATE_API_TOKEN` for the repo, deletes the
   project's secrets from OpenBao, and removes the routing entry and allowlist entry from
   Redis.
3. **Operator GitHub-side cleanup** — the UI prompts the operator to remove the deploy key from
   the GitHub repository's settings, revoke the per-project PAT in GitHub, and delete the two
   workflow files and three configuration values from the repo. Convocate never calls GitHub
   to mutate repo settings on the operator's behalf.

The form never echoes a stored secret back; rotating means re-uploading. Keypair generation and
deploy-key registration on GitHub are always operator actions — convocate never generates SSH
keys or registers deploy keys itself.

**Cluster Authentication (Claude).** A separate one-time setup form in the Web UI captures the
authentication every Agent Container uses to talk to Claude. The operator picks one of two
modes; convocate stores the result in OpenBao as a shared service credential:

- **Anthropic API key.** Paste a key from `console.anthropic.com`. Convocate stores it under
  `shared/anthropic_api_key` and Agent Containers export it as `ANTHROPIC_API_KEY` at
  start. Stateless; no refresh loop runs.
- **Claude.ai subscription.** The Web UI walks the operator through the Claude.ai OAuth
  dance: the operator authorizes convocate from a browser tied to a Claude.ai subscription,
  the Web UI captures the resulting session token, and convocate stores it under
  `shared/claudeai_session`. The wrapper inside each Agent Container runs a refresh loop to
  keep the session live; if the upstream session is invalidated server-side, the Web UI
  flags it for the operator to re-authenticate.

Either mode satisfies the wrapper's auth requirements. Switching modes via the Web UI forces
every Agent Container to pick up the new credential on its next task; in-flight tasks finish
on the credential they started with.

Claude authentication is **cluster-wide** — there is no per-project Claude credential. A
compromised Agent Container could exfiltrate the shared API key or session token and burn the
org's Claude quota; that risk is accepted in exchange for the simpler operational model.
Per-project Claude credentials (for orgs that want billing or quota isolation between
projects) are explicitly out of MVP scope.

**Ad-hoc job submission.** A separate Web UI form accepts a free-form prompt against an
existing project — no GitHub Issue required. The operator picks a project, types a prompt,
and submits; the Router API dispatches it to the bound Agent Container as a background task,
identified by a synthetic job ID with no issue number. Two things differ from the
label-triggered path:

- The agent opens a pull request with the prompt as the PR body. No `Closes #N` reference is
  added because there is no originating issue.
- Clarification questions and failure reasons stream to the Web UI's job-detail view rather
  than to a GitHub Issue comment. The operator monitors the run there and decides whether to
  give up or resubmit with a refined prompt.

Ad-hoc submissions share the same idempotency ledger, keyed by `(repository, "ad-hoc",
web-ui-submission-id)`, and the same per-container concurrency budget as label-triggered
jobs.

The Web UI is a SPA written using the specifyjs.asymmetric-effort.com UI framework.

#### GitHub Action (dispatch)

A thin HTTP client. It sends the issue payload to the Router API and exits. Job duration
is irrelevant because the runner does no execution work. The workflow is defined once in
the convocate monorepo as a reusable workflow and called from project repositories with a
single-line reference.

#### GitHub Action CI/CD Pipeline

Runs on every push to a `fix/**` or `feature/**` branch and gates execution on
`github.actor == vars.CONVOCATE_BOT_ACCOUNT`, so only pushes authored by the bot trigger the
pipeline. It executes the project's test suite, reports results, and triggers the ticket
updater on completion.

#### GitHub Action Ticket Updater

Posts status comments to the originating GitHub Issue, transitions issue state, and reassigns
the issue to the original author when work is complete or blocked.

### Agent Container Image

Each agent container is built from a single image defined in the convocate monorepo:

```dockerfile
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y git curl openssh-client && rm -rf /var/lib/apt/lists/*

# gh CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) \
      signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] \
      https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list && \
    apt-get update && apt-get install -y gh

# Claude Code
RUN npm install -g @anthropic-ai/claude-code

# Go wrapper — the container entrypoint
COPY convocate-agent-wrapper /usr/local/bin/convocate-agent-wrapper

ENTRYPOINT ["/usr/local/bin/convocate-agent-wrapper"]
```

The Go wrapper reads job context from two sources. Non-secret context — issue payload, target
repository URL, job ID — is injected as environment variables by the Dispatch Service per
background task. Credentials — project-scoped GitHub PAT, Anthropic API key, and the project's
ed25519 SSH private key — are read on demand from the host's OpenBao Agent over the Unix socket
bind-mounted at `/run/convocate/secrets.sock`. The SSH key is materialized at `~/.ssh/id_ed25519`
(mode `0600`) with a matching `known_hosts` entry for `github.com`; git operations use this key
exclusively. Credentials never appear in environment variables, image layers, or `docker inspect`
output.

### Service Communication

All inter-service traffic uses mutual TLS over HTTPS. There is no SSH control plane between
running services. Operator-driven SSH for installation and Day-2 work (copying the mTLS cert
to a fresh agent host, running `docker compose pull` across the fleet, etc.) is expected and
fine — the "no SSH" rule is about the service-level data path, not how the operator
administers their machines.

|       From       |        To        |     Port      | Purpose                                          |
|------------------|------------------|---------------|--------------------------------------------------|
| GitHub Action    | Router API       | `tcp/443`     | Job submission (HTTPS, bearer token)             |
| Web UI           | Router API       | `tcp/8443`    | Management, status, ad-hoc submission (HTTPS)    |
| Dispatch Service | Router API       | `tcp/8443`    | HTTPS long-poll/SSE — dispatch + status (mTLS)   |
| Dispatch Service | Redis            | `tcp/6379`    | Host queue/lifecycle namespace (Redis TLS)       |
| OpenBao Agent    | OpenBao server   | `tcp/8200`    | Secret lease/fetch (HTTPS mTLS, AppRole)         |
| Agent Container  | OpenBao Agent    | Unix socket   | Local secret fetch (per-container, host-local)   |
| Agent host       | Container logs   | operator-set  | Standard syslog/journald/Docker log driver       |

The Dispatch Service opens a long-lived mTLS HTTPS connection to the Router API at boot. It
long-polls (or SSE-subscribes) for new dispatch events targeted at its host and POSTs status
transitions back over the same mTLS channel. Agent hosts are never required to be reachable
inbound from the Router API; the control plane and agent hosts can sit on opposite sides of a
NAT.

The Web UI shares the `tcp/8443` listener but on a separate URL prefix (`/ui/...`) so the
Router API can serve operator HTTPS and Dispatch HTTPS on one port. mTLS is required for the
Dispatch routes; the Web UI routes accept either a session cookie issued by the Router API or
a client cert, depending on how the operator's reverse proxy is configured.

mTLS certificates are issued from a private CA generated at control-plane initialization.
`convocate-cli ca print-bundle` emits the trust bundle agent hosts need at install. Each
service holds its own keypair; there are no shared keys across the cluster.

### Job Lifecycle

```text
'automated-development' label applied
        │
        ▼
GitHub Action fires → POST /jobs → Router API
        │
        ▼
Router API: idempotency check → resolve project→container binding → returns job_id
        │                       (HTTP 404 if no project exists for this repo)
        ▼
GitHub Action applies 'dispatched' label and exits
        │
        ▼
Dispatch Service delivers prompt to the bound Agent Container
        │
        ├─[failed_dispatch]─► (missing/unhealthy container; replacement also failed)
        │                     Router API removes 'dispatched' label,
        │                     applies 'failed_dispatch' label, stops
        │
        ├─► Agent reuses or clones worktree, creates branch fix/issue-<N> or feature/issue-<N>
        │
        ├─► Claude Code implements solution, commits incrementally
        │
        ├─► Agent runs test suite
        │
        ├─[success]─► Agent opens PR "Closes #<N>", removes 'dispatched' label,
        │             reassigns issue to author
        │
        ├─[clarification needed]─► Agent posts comment with questions,
        │                          removes 'automated-development' + 'dispatched' labels,
        │                          reassigns to original author
        │
        └─[failure]─► Agent posts failure reason,
                      removes 'automated-development' + 'dispatched' labels,
                      reassigns to original author
```

### Issue Comment Cadence

The agent posts one comment per per-job state transition on the originating GitHub Issue:
entering `running` ("starting work"), `clarifying` (with the clarification questions),
`complete` (with the PR link), `failed` (with the diagnostic summary), `terminated` ("job
cancelled by operator"). There are no time-based progress pings — quiet between transitions
is the expected behavior. For finer-grained progress, operators watch the Web UI's job-detail
view. Ad-hoc jobs skip these comments entirely (they have no originating issue) and stream
the same transitions to the Web UI instead.

### Per-Job Branch Cleanup

Each job pushes to a per-job feature branch (`fix/issue-<N>` or `feature/issue-<N>`). On PR
merge the agent deletes the branch — both locally in the container's worktree and remotely on
GitHub via the project's PAT — and prunes the matching `git worktree`. Branches behind PRs
that close without merging, or behind jobs that ended in `failed` / `failed_dispatch` /
`terminated`, are **kept** so the operator (or a follow-up retry) can inspect or revive the
work; the operator deletes those manually when no longer needed.

### Pipeline Failure Self-Healing

When a CI/CD pipeline run fails on a bot-owned branch, the agent is expected to detect and resolve
the failure autonomously using the `gh` CLI. The agent queries pipeline status, reads failure
logs, applies fixes, and commits.

Self-healing is bounded: **up to 3 retry attempts, with a 30-minute wall-clock cap across all
attempts**. If 3 attempts fail or the timebox is exceeded, the agent posts a diagnostic
comment summarizing what it tried, removes both the `automated-development` and `dispatched`
labels, reassigns the issue to the original author, and exits cleanly. The agent does not loop
indefinitely and does not silently consume more Claude budget than the cap allows.

---

## Project Integration

Convocate is a platform. Project repositories integrate by referencing convocate's reusable GitHub
Actions — they own no dispatch logic themselves.

### Bot Account Setup

Create a dedicated GitHub machine user account (e.g. `claude-code-bot`). Add it as a collaborator
with **write** access to each project repository (write is enough — convocate never calls the
GitHub admin API on the bot's behalf, so admin is not required).

Convocate uses one fine-grained GitHub PAT per project, held only in OpenBao:

- **Per-project PAT** — fine-grained, scoped to exactly one project repository, with permissions
  for contents read/write, pull requests write, and issues write. Loaded into OpenBao under the
  project's namespace at Create Project time. The bound Agent Container reads it on demand
  through its per-container secrets socket and uses it for direct `gh`/GitHub API calls (PR
  creation, issue labelling, reassignment). The Router API also reads it directly from OpenBao
  for the narrow case where no container can act on the project's behalf — for example,
  applying the `failed_dispatch` label to an issue when container replacement on dispatch
  fails. A container's PAT can only touch its bound repository because the PAT is repo-scoped
  by GitHub.

There is no cluster-wide bot PAT. Operator-driven actions on GitHub (creating deploy keys,
revoking PATs, removing the bot as a collaborator) are performed by the operator directly in
GitHub's UI — convocate stores no credential that could authorize them automatically.

Agents talk to GitHub and the Anthropic API directly using credentials they pull from OpenBao;
no GitHub or Anthropic traffic is proxied through the Router API, so the control plane never
sits on the data path that carries job payloads.

The bot account name must never be hardcoded. It is defined as a GitHub Actions variable
(`CONVOCATE_BOT_ACCOUNT`) at the repository or organization level.

Separately, the Router API mints a dispatch-only bearer token (`CONVOCATE_API_TOKEN`) for each
allowlisted repository. This token has no GitHub permissions — it authenticates the project's
GitHub Action when it calls the Router API's `/jobs` endpoint, and the Router API rejects it for
any repository other than the one it was minted for. The Web UI displays the bearer once at
allowlist time; the operator copies it into the project repo's Actions secrets store as
`CONVOCATE_API_TOKEN`. Rotating or revoking one repo's bearer has no effect on any other repo.

### Per-Repository Configuration

Add two variables and one secret to each project repository under Settings → Secrets and Variables
→ Actions:

| Type     | Name                    | Value                                                              |
| -------- | ----------------------- | ------------------------------------------------------------------ |
| Variable | `CONVOCATE_BOT_ACCOUNT` | The bot account login name                                         |
| Variable | `CONVOCATE_ROUTER_URL`  | URL of the deployed Router API                                     |
| Secret   | `CONVOCATE_API_TOKEN`   | Router API dispatch bearer for this repo (issued at allowlist time)|

### Workflow Files

Add two workflow files to `.github/workflows/` in each project repository. These files contain no
logic — they are pure configuration delegating to the convocate monorepo.

**`.github/workflows/convocate-dispatch.yml`** — fires when the `automated-development` label
is applied to an issue:

```yaml
on:
  issues:
    types: [labeled]

jobs:
  convocate:
    if: github.event.label.name == 'automated-development'
    uses: asymmetric-effort/convocate/.github/workflows/convocate-dispatch.yml@main
    with:
      router_api_url: ${{ vars.CONVOCATE_ROUTER_URL }}
      bot_account: ${{ vars.CONVOCATE_BOT_ACCOUNT }}
    secrets:
      router_api_token: ${{ secrets.CONVOCATE_API_TOKEN }}
```

**`.github/workflows/convocate-feedback.yml`** — fires on push to bot-owned branches:

```yaml
on:
  push:
    branches:
      - 'fix/**'
      - 'feature/**'

jobs:
  feedback:
    uses: asymmetric-effort/convocate/.github/workflows/convocate-feedback.yml@main
    with:
      router_api_url: ${{ vars.CONVOCATE_ROUTER_URL }}
      bot_account: ${{ vars.CONVOCATE_BOT_ACCOUNT }}
    secrets:
      router_api_token: ${{ secrets.CONVOCATE_API_TOKEN }}
```

That is the complete per-repository footprint. All dispatch logic, agent management, and feedback
handling is defined in the convocate monorepo and versioned independently of the projects it
serves.

### Issue Quality

The quality of convocate's output is bounded by the quality of the issue. Issues should include a
clear description of the problem or feature, explicit acceptance criteria, relevant file paths or
component names, and reproduction steps for bugs. Vague issues produce clarification requests
rather than code. Well-specified issues produce pull requests.

If an issue is too ambiguous to act on, the agent posts specific questions as a comment, removes
the `automated-development` label, and assigns the issue back to the original author. The author
updates the issue and re-applies the `automated-development` label to retry — no other action is
required.

If an issue comes back with a `failed_dispatch` label instead, convocate's own infrastructure
couldn't reach an Agent Container for that project (typically: agent host out of resources or
the project's container is unhealthy and replacement also failed). This is an operator problem,
not an issue problem — the engineer's only action is to wait until the operator clears the
project's `failed_dispatch` state in the Web UI, then re-apply the `automated-development`
label to retry.

### Repository Allowlist

The Router API maintains an allowlist of authorized repository full names (e.g. `org/repo`).
Allowlist entries live in the Router API's Redis namespace alongside the routing table — the
allowlist is routing metadata, not a secret, so OpenBao isn't the right store for it. Job
submissions from repositories not on the allowlist are rejected with **HTTP 404** before entering
the queue. This prevents unauthorized repositories from consuming convocate resources if the API
token is inadvertently exposed. In MVP, the allowlist is populated only via the Web UI's Create
Project flow — there is no auto-provisioning code path, and there is no separate "just add to the
allowlist" action.

---

## Deployment

### Prerequisites

| Role                 | Requirements                                                                  |
|----------------------|-------------------------------------------------------------------------------|
| Operator workstation | `git`, `make`, Docker, Go 1.26+                                               |
| Control plane host   | Docker 25+, Docker Compose                                                    |
| Agent host           | Docker 25+ with **systemd cgroup driver**, Docker Compose, systemd cgroup v2  |

Every host needs outbound HTTPS to GitHub and the Anthropic API. Agent hosts additionally need
outbound access to the control plane on `tcp/6379` (Redis, TLS), `tcp/8200` (OpenBao), and
`tcp/8443` (Dispatch + Web UI). There are no operating-system requirements beyond a current
Docker — the "control plane host" can be a small VM, a dedicated server, an existing Kubernetes
node, or one of the agent hosts itself if you're consolidating.

The simplest topology runs the control-plane Compose stack and the agent-host Compose stack on
the same machine. The Router API listens on `tcp/443` (HTTPS for GitHub Actions `/jobs`) and
`tcp/8443` (Web UI HTTPS + mTLS HTTPS for Dispatch Services), and the OpenBao server listens on
`tcp/8200`.

### Build

```bash
git clone https://github.com/asymmetric-effort/convocate.git
cd convocate
make clean          # remove build/ directory and recreate it, remove project docker containers and container images.
make lint           # go vet + yaml lint
make test           # go test ./... for unit/integration tests and e2e tests against the entire local dev stack
make images         # builds convocate-router, convocate-dispatch, convocate-agent OCI images
make build          # builds all images and other software (calls 'make images' as well as other build steps)
make local/start    # start the local dev environment
make local/logs     # tail logs from every dev-stack service
make local/stop     # stop the local dev environment (volumes preserved)
make local/reset    # tear down stack and volumes, regenerate the local CA, start clean
make release        # bump the patch version then commit+tag+push
make release/major  # bump the major version and commit+tag+push
make release/minor  # bump the minor version and commit+tag+push
```

`make images` builds every component as an OCI container image, tagged `convocate-<svc>:<version>`,
and pushes to the configured registry. Operators pull from that registry; no per-host source
build is required.

### Deploy the Control Plane

```bash
cd deploy/control-plane
cp .env.example .env
# Set:
#   CONVOCATE_PUBLIC_URL=https://router.example.com
#   CONVOCATE_OPENBAO_UNSEAL=/run/secrets/openbao-unseal   # local sealed-file path
docker compose up -d
```

`CONVOCATE_PUBLIC_URL` is the hostname GitHub Actions and operators use to reach the Router
API. It serves two purposes: it becomes a Subject Alternative Name on the Router API's TLS
certificate when the stack generates the private CA at first start, and it is the base URL
the Web UI embeds in operator-facing links (e.g. the PR-link and `CONVOCATE_API_TOKEN`
display on the Create Project confirmation screen).

The Compose stack starts:

- `convocate-router` — Router API on `tcp/443` (HTTPS for GitHub Actions `/jobs`) and
  `tcp/8443` (Web UI HTTPS + mTLS HTTPS long-poll for Dispatch Services, path-multiplexed)
- `convocate-redis` — on `tcp/6379` with native TLS; reachable by the Router API on the
  Compose network and by per-host Dispatch Services from agent hosts
- `convocate-openbao` — OpenBao server on `tcp/8200`, with a Docker volume for Raft storage

At first start the stack generates a private CA used for the Router API ↔ Dispatch mTLS channel
and the OpenBao Agent ↔ OpenBao server channel. The `convocate-router` image also ships
**`convocate-cli`**, an operator admin tool used inline below for CA bundle export and per-host
cert issuance (`convocate-cli ca print-bundle`, `convocate-cli host issue-cert`); it is invoked
via `docker compose exec router convocate-cli ...` against the running router container. Run
`docker compose exec router convocate-cli ca print-bundle > control-ca.pem` to emit the trust
bundle agent hosts need.

### Deploy an Agent Host

Each agent host runs its own Compose stack. Before bringing it up, mint the host's mTLS client
cert on the control plane and copy it over (this is the only step that requires operator-driven
SSH or equivalent out-of-band transport — there is no automatic enrolment):

```bash
# On the control plane host
docker compose exec router convocate-cli host issue-cert <host-id> > host.pem
scp host.pem control-ca.pem agent-a:/etc/convocate/
```

Then on the agent host:

```bash
cd deploy/agent-host
cp .env.example .env
# Set:
#   CONVOCATE_CONTROL_URL=https://router.example.com:8443
#   CONVOCATE_CONTROL_CA=/etc/convocate/control-ca.pem
#   CONVOCATE_HOST_CERT=/etc/convocate/host.pem
#   CONVOCATE_HOST_ID=<unique identifier matching the cert>
docker compose up -d
```

The stack starts three services:

- `convocate-dispatch` — opens its mTLS HTTPS long-poll to the Router API at boot, registers
  the host, and waits for dispatch events. Mounts the host's Docker socket so it can launch
  Agent Containers as siblings, enrolling each in `convocate-sessions.slice` (90% CPU and
  memory cap). The systemd cgroup driver is a hard prerequisite (see Prerequisites); Dispatch
  refuses to start on hosts not configured for it.
- `convocate-openbao-agent` — upstream OpenBao Agent, authenticating to the central OpenBao
  with a host-bound AppRole.
- `convocate-secrets-broker` — the per-container socket multiplexer described in
  Architecture → Secrets Manager. Talks to `convocate-openbao-agent` on the local Compose
  network; exposes one Unix socket per Agent Container under `/run/convocate/secrets/`.

### Verify

```bash
# Control plane host
docker compose ps                                              # all services Up
curl -fsSk https://localhost/health                            # Router API health
docker compose exec redis redis-cli ping                       # PONG
docker compose exec openbao bao status                         # Unsealed: true

# Agent host
docker compose ps                                              # dispatch + openbao-agent Up
docker compose logs --tail=20 convocate-dispatch               # "registered with router"
docker images | grep convocate-agent
systemctl status convocate-sessions.slice                      # active
```

### Web UI Access

The Web UI is served by the Router API on `tcp/8443` by default. Its capabilities are
described in Architecture → Web UI; this section covers exposure and authentication only.

Access is restricted to the control plane host's network by default. Convocate ships no
first-party operator authentication — to expose the Web UI beyond the control plane host, the
operator must front it with an authenticating reverse proxy (Authelia, oauth2-proxy, Cloudflare
Access, etc.). The Router API trusts proxied identity headers from a configured upstream and
ignores them otherwise. SSO and password-based first-party auth are out of MVP scope.

---

## Day-2 Operations

**Roll out a new release across the fleet:**

```bash
# Build and push new images
cd convocate && git pull && make images

# Control plane
cd deploy/control-plane && docker compose pull && docker compose up -d

# Each agent host
ssh agent-a 'cd /opt/convocate/agent-host && docker compose pull && docker compose up -d'
```

Existing Agent Containers keep their original image tag indefinitely — the Router API never
retires a healthy container on its own. After `docker compose pull` brings the new image onto
each agent host, the Web UI surfaces an **"image upgrade available"** indicator on every
project whose bound container is running an older tag. The operator clicks **"Upgrade
Container"** on each project they want to roll forward (or **"Upgrade All Idle"** to act on
every project that currently has no in-flight jobs); the Router API then instructs the bound
host's Dispatch Service to stop the old container and provision a replacement on the new
image, then re-binds the project. Cutover is operator-initiated, project-by-project, and
never mid-job — convocate refuses to upgrade a container while it has in-flight tasks.

**Add a new project repository (MVP):**

1. Open the Web UI and run the **Create Project** flow (see Architecture → Web UI → Project
   lifecycle): enter the repo full name and upload the operator-generated ed25519 SSH private
   key, the repo-scoped fine-grained PAT, and any custom per-project secrets. The Router API
   writes the repo to the allowlist, stores the secrets in OpenBao, mints
   `CONVOCATE_API_TOKEN`, and provisions the Agent Container in one step.
2. Add the bot account as a collaborator in the GitHub repository settings, and register the
   matching ed25519 public key as a deploy key with write access.
3. Add the two workflow files and three configuration values (including the displayed
   `CONVOCATE_API_TOKEN`) to the repository.
4. Apply the `automated-development` label to a test issue to verify end-to-end.

Job submissions for repositories that haven't completed the Create Project flow are rejected
with HTTP 404 — projects are never auto-provisioned from a GitHub Actions submission.

**Remove a project repository (MVP):**

1. Open the Web UI and run the **Delete Project** flow on the project. The Router API drains
   in-flight jobs, tears down the Agent Container, revokes `CONVOCATE_API_TOKEN`, deletes the
   project's secrets from OpenBao, and removes the repo from the allowlist (HTTP 404 from then
   on).
2. In the GitHub repository: remove the deploy key, revoke the per-project PAT, delete the two
   workflow files and three configuration values, and remove the bot account from collaborators
   if it should no longer have access.

**Monitor job failures:**

Failed jobs appear in the Web UI with the failure reason and the last log lines from the agent
container. The originating GitHub Issue has a failure comment posted by the bot. Re-applying the
`automated-development` label after addressing the root cause retries the job from scratch on a
clean branch.

---

## Security Posture

**Network isolation.** Agent Containers have outbound internet access (for package registries,
documentation, and the Anthropic API) but accept no inbound connections and cannot reach Redis,
the OpenBao server, or the Router API's mTLS HTTPS listener (`tcp/8443`). The Router API's
public HTTPS endpoint on `tcp/443` is intentionally reachable (GitHub Actions and operators
submit jobs to it) and must sit behind an authenticating reverse proxy or load balancer.

**Credential scope.** Credentials — project-scoped GitHub PAT, Anthropic API key, the project's
ed25519 SSH keypair, and any per-project secrets — live in the OpenBao server in the control
plane stack. The host-local OpenBao Agent releases them to bound containers over a per-container
Unix socket, on-demand, with short-lived tokens. Credentials never appear in environment
variables, image layers, `docker inspect` output, the target repository, or git history. When
the Router API moves or revokes a project's binding (image upgrade, host migration, project
removed), the prior container's socket authorization is revoked atomically with the rebind. The
operator separately removes the corresponding deploy key from GitHub as part of the Delete
Project flow — convocate never calls GitHub's admin API itself.

**Git push isolation.** Each project's Agent Container has its own ed25519 SSH keypair registered
as a write-enabled deploy key on exactly one repository, and its own fine-grained PAT scoped to
that same repository for `gh`/GitHub API operations. Both credentials are repo-scoped by GitHub,
so a compromised container cannot push to or call the GitHub API against any other repository —
not even repositories the bot account otherwise has access to. Convocate stores no cluster-wide
bot PAT; there is no credential anywhere in the system that grants cross-repo write access.
Each project's GitHub Actions runner holds only `CONVOCATE_API_TOKEN`, a dispatch-only bearer
that authorizes calls to the Router API's `/jobs` endpoint for that one repository and grants
no GitHub permissions.

**Agent isolation.** Each project runs in a dedicated long-lived container enrolled in
`convocate-sessions.slice`, which enforces a 90% aggregate CPU and memory cap across all
containers on the host. The Router API's strict project → container binding guarantees that one
project's prompts, build artifacts, and credentials are never visible to another project's
container. A runaway agent cannot starve the host or other agents.

**Key management.** Every inter-service channel uses mutual TLS with certificates issued from a
private CA generated at control-plane initialization. Each service holds its own keypair;
there are no shared keys across the cluster, and a compromised agent host's certificate can be
revoked without affecting any other host.

**Repository allowlist.** The Router API rejects job submissions from repositories not explicitly
authorized. An exposed API token cannot be used to run jobs against arbitrary repositories.

**Image supply chain (MVP).** Convocate pulls every `convocate-*` image from a configured OCI
registry; operators trust that registry. There is no cosign/sigstore signing or signature
verification in MVP. Operators who need stronger supply-chain guarantees can run their own
registry with image-digest pinning. Signed-image enforcement is a Phase-2 hardening item.

---

## Development

The canonical Makefile target list lives in **Deployment → Build** above. This section covers
the local-development workflow and testing expectations only.

**Local ecosystem (Docker Compose).** Day-to-day development runs the full convocate ecosystem
on the developer's workstation via Docker Compose. The repo ships a repo-root
`docker-compose.dev.yml` (alongside `deploy/control-plane/` and `deploy/agent-host/`) that
combines the control-plane and agent-host stacks into a single Compose network with local-only
defaults: self-signed CA generated on first start, sealed OpenBao unseal-file in
`./.dev/secrets/`, no external registry (images built locally by `make images`),
`localhost`-bound listeners. There is no separate dev binary — the same container images that
deploy to production run here.

```bash
make local/start  # build images and bring up Router + Redis + OpenBao + Dispatch + broker + 1 agent
make local/logs   # tail logs from every service
make local/stop   # stop the stack (volumes preserved)
make local/reset  # tear down stack and volumes, regenerate the local CA, start clean
```

The dev stack uses non-privileged ports so it runs without `sudo` on a typical workstation. The
Router API is exposed on `https://localhost:8443/` (GitHub Actions endpoint, served on the
production-equivalent of `tcp/443`) and `https://localhost:8444/` (Web UI + Dispatch, served on
the production-equivalent of `tcp/8443`). Open the Web UI in a browser, run **Create Project**
against a test repository you control, and either point Cluster Authentication at a real
Anthropic API key or set `CONVOCATE_DEV_MOCK_CLAUDE=1` in `.env` before `make images` to bake
the bundled `mock_claude` binary into the Agent Container image instead. The flag is read at
image-build time only; flipping it back to `0` (or unsetting it) requires `make local/reset`
followed by `make local/start` to rebuild the Agent Container image without `mock_claude`.

Coverage targets for business-logic packages are 90%+. Provisioning paths that require root and
real network access are excused from automated coverage.

The `test/e2e/` directory contains a mock Claude binary (`mock_claude`) that returns deterministic
responses for end-to-end pipeline testing without consuming Anthropic API credits.

## Organization-wide coding standards

**Authoritative reference:** <http://coding-standards.asymmetric-effort.com/>

That site is the source of truth for how code is written across every
asymmetric-effort project. Read it before touching anything; the
project-specific Conventions section below augments it but does not
override it. When this file disagrees with the standards site, the
standards site wins — fix this file.

## Conventions

- **No recursion.** Go has no tail-call optimization, so every recursive
  call grows the goroutine stack and can panic on adversarial input
  (deep trees, cyclic data, hostile filesystem layouts). For an
  orchestrator that holds long-lived session state, that's
  unacceptable. Use loops, explicit stacks/queues, `filepath.WalkDir`,
  or work-list patterns instead. Mutual recursion (A → B → A) is also
  forbidden — same problem. If you find yourself reaching for a
  helper that calls itself, stop and rewrite as iteration.
  Reference implementation: `session.copyDir` (uses an explicit
  work-list slice).
- **No speculative abstractions.** Fix what's in front of you; the next
  caller will refactor when it actually shows up.
- **Don't write docstrings explaining what code already says.** Comment
  the *why* when it's non-obvious (hidden invariant, workaround, past
  incident).
- **Tests verify behavior, not implementation.** When you change a
  signature, update callers and the failing tests; don't relax assertions.
- **Coverage targets** — aim for 90%+ in business-logic packages (`session`,
  `container`, `capacity`, `dns`). Installer and cmd/main have
  interactive/root-only paths that can't reasonably be unit tested.
- **Variable/Function/Type names should be explanatory** limit the use of short variable
  names that do not explain what the variable is.
- **StrongTyping** We prefer strong typing and safety to create a secure and stable product.
- **Rigorous testing** We expect >=98% unit/integration and e2e test coverage of happy and sad paths;
  where testing should include playwrite post-deployment verification tests for all User interafaces
  and APIs.

---

## License

MIT License — see [LICENSE.txt](LICENSE.txt)
