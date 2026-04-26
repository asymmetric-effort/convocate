# Architecture

## Three binaries

As of v2.0.0 convocate ships three cooperating binaries.

- **`convocate`** — the interactive TUI. Runs as user `claude`. Lists
  sessions across all registered agents, creates sessions on a chosen
  agent, attaches to a session's container over SSH. Doesn't run
  containers itself.
- **`convocate-agent`** — the worker. One per host that actually hosts
  session containers. Listens on `tcp/222` over SSH. Runs `docker run`
  for sessions. Enrolls every container under `convocate-sessions.slice`
  for a 90% aggregate cgroup cap.
- **`convocate-host`** — the deploy tool. Provisions vanilla Ubuntu
  hosts, copies binaries over SSH, wires up SSH peering + TLS-encrypted
  log forwarding, distributes the container image.

## Session isolation

Each session has:

- a unique UUIDv4
- its own home directory at `/home/claude/<uuid>/` on the **agent** host
- a dedicated Docker container named `convocate-session-<uuid>`,
  enrolled in `convocate-sessions.slice` for kernel-enforced 90%
  aggregate CPU + memory cap

## Shared resources (read-only bind mounts, on the agent)

- Claude CLI binary (`/usr/local/bin/claude`)
- SSH keys (`~/.ssh/`)
- Git configuration (`~/.gitconfig`)
- Claude settings (`~/.claude/` mounted as `~/.claude-shared/`)

## Control plane

- **Shell → Agent (`tcp/222`)** — SSH subsystems
  `convocate-agent-rpc` (CRUD JSON RPC) and `convocate-agent-attach`
  (raw pty relay). The shell holds a persistent connection per agent.
- **Agent → Shell (`tcp/223`)** — SSH subsystem
  `convocate-status` — newline-JSON event stream
  (`agent.started`, `agent.heartbeat`, `container.created`,
  `container.started`, `container.stopped`, `container.deleted`, etc.).
  Agent holds a persistent connection.
- **Agent → Shell (`tcp/514`)** — rsyslog TLS-encrypted log forwarding,
  authenticated with per-agent client certs under the CA minted by
  `init-shell`.

## Container image distribution

The shell host builds `convocate:<semver>` during `convocate install`.
`init-agent` and `update` ship that tarball to each agent via
`docker save | gzip | ssh | docker load`, verifying SHA-256 on both
ends. A daily cron on each agent deletes image tags no container is
still referencing.

## Security posture

- All control-plane traffic over SSH (ed25519 host + client keys) or
  TLS (ECDSA P-256 for the rsyslog channel).
- Each agent gets its own pair of keypairs at provisioning time —
  there is no shared key.
- `convocate-agent` accepts only the `convocate-agent-rpc` and
  `convocate-agent-attach` SSH subsystems. Shell access, exec, port
  forwarding, and any other channel type are refused.
- The `claude` user inside containers has NOPASSWD sudo only inside
  its container, never on the host.
- 90% cgroup cap on `convocate-sessions.slice` plus 90% admission
  control at create time means a runaway session can't starve the
  agent of resources.

## Why "convocate"?

*Convocate* is an archaic English verb meaning "to call together;
to convoke." It comes from Latin *com-* "together" + *vocare* "to
call." It captures what the system actually does: the TUI calls
together AI sessions running across many agent hosts, gathering them
into one operator pane.

The name is also clean across software namespaces — no GitHub,
PyPI, npm, Docker Hub, or trademark collisions; reusable as a
brand without inheriting the AI-product naming saturation that
forced the project's rename from `claude-shell`.
