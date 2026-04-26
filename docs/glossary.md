# Glossary

Quick reference for terms used throughout the docs.

## Roles

**Operator** — A human running the `convocate` TUI or `convocate-host`
CLI. Trusted; holds shell-side keys.

**Shell host** — The machine running the `convocate` TUI service and
the `convocate-status` listener. Operators SSH here as the `claude`
user to enter the TUI. Also runs the rsyslog server and (optionally)
dnsmasq for cluster DNS.

**Agent host** — A machine that runs session containers. Has
`convocate-agent.service` listening on `tcp/222`. There can be one
or many.

**Session** — A unit of work managed by convocate. Comprises a UUID,
a home directory on an agent (`/home/claude/<uuid>/`), and a Docker
container (`convocate-session-<uuid>`) that runs Claude CLI inside
tmux.

## Identifiers

**UUID** — UUIDv4 stamped on every session at create time. The
primary key for everything.

**Agent ID** — 8-character random string (e.g. `abc12345`) generated
once per agent at `convocate-agent install` time. Never changes for
that machine.

**Session name** — Human-readable label set by the operator. Editable
via the TUI's `(E)dit` action. Display-only; not used for routing.

**DNS name** — Optional per-session label that resolves to the agent
host's IP via the cluster's dnsmasq. `<dns-name>.<domain>` is the
queryable record.

## Network

**`tcp/222`** — Agent SSH listener. Subsystems: `convocate-agent-rpc`
(JSON-RPC) and `convocate-agent-attach` (PTY relay). Shell is the
client.

**`tcp/223`** — Shell SSH listener. Subsystem: `convocate-status`
(event push). Agent is the client.

**`tcp/514`** — Shell rsyslog TLS listener. Receives forwarded
container logs from each agent.

**`udp/53` + `tcp/53`** — Shell dnsmasq, when used. Cluster DNS
authority for session DNS names.

## Subsystems

**`convocate-agent-rpc`** — Newline-JSON request/response RPC. CRUD
ops on sessions: list, get, create, edit, clone, delete, kill,
background, override, restart, ping, settings-get, settings-set.

**`convocate-agent-attach`** — Raw byte relay. Client writes a
JSON header line with the session UUID, then the channel becomes a
pipe to `tmux attach-session` inside the container.

**`convocate-status`** — Newline-JSON event stream from agent to
shell. Lifecycle events for the agent and its containers.

## States and indicators

| Char | Name | What it means |
|---|---|---|
| `-` | Stopped | No container running, no operator attached |
| `L` | Locked | Lock file exists with a live PID, but no container is up |
| `R` | Running | Container is up; no operator attached right now |
| `C` | Connected | Container is up; ≥1 operator currently has a PTY attached |
| `O` | Orphan | Session directory exists locally but isn't claimed by any registered agent (only seen during pre-v2 → v2 migration) |

## Containers and isolation

**`convocate-session-<uuid>`** — The container name pattern. Each
session gets one container.

**`convocate-sessions.slice`** — The systemd slice (cgroup parent)
that every session container is enrolled in. Aggregate cap of 90%
host CPU + memory.

**Cgroup cap** — The systemd slice's `CPUQuota` + `MemoryMax`
limits, kernel-enforced. Layer 2 of the two-layer capacity model.

**Admission control** — The pre-flight 90% check the agent runs in
its `Create` handler before doing `docker run`. Layer 1.

**`claude` user** — UID 1337, GID 1337. The user account on shell
and agent hosts that owns convocate state, runs the TUI as a login
shell, and runs Claude CLI inside containers (with the same UID via
`CLAUDE_UID` / `CLAUDE_GID` env passed to entrypoint).

**Skel** — Files seeded into a freshly-created session's home
directory. Currently includes a project-level `CLAUDE.md` so Claude
Code knows the conventions. Source: `internal/assets/data/CLAUDE.md.gz`.

## Cryptography

**ed25519** — The only SSH key type convocate generates or accepts.
Project-wide invariant — no RSA, no ECDSA. Used for host keys,
peering keys, status push.

**ECDSA P-256** — TLS for the rsyslog log-forwarding channel.
Per-cluster CA + per-agent client certs.

**Host key pinning** — Both directions. The shell pins the agent's
SSH host key at provisioning time; the agent pins the shell's host
key for status push.

## Commands by role

### Operator (interactive)

- `convocate` — boot the TUI

### Operator (provisioning / one-shot)

- `convocate-host install` — prep a fresh Ubuntu host
- `convocate-host init-shell` — set up the shell side
- `convocate-host init-agent` — register a new agent
- `convocate-host update` — push a new binary/image to an agent
- `convocate-host migrate-session` — move a v1 orphan to an agent
- `convocate-host create-vm` — provision a fresh KVM VM as an agent

### Service (no manual invocation)

- `convocate status-serve` — driven by `convocate-status.service`
- `convocate-agent serve` — driven by `convocate-agent.service`

## Misc

**Image tag** — `convocate:vX.Y.Z`. Built by `convocate install`
on the shell host; distributed to agents by `init-agent` and
`update`.

**`current-image`** — The file at `/etc/convocate-agent/current-image`
that records the active image tag for new session creates and
restarts.

**Status emitter** — The agent's persistent connection back to the
shell on `tcp/223`. Reconnects with backoff (default 1s → 30s).

**Heartbeat** — `agent.heartbeat` event the emitter publishes on a
configurable cadence (default 30s). Used to detect silent agents.

**Skel directory** — `/home/claude/.skel/` on a session-creating
agent; contains the per-session starter content (currently just
`CLAUDE.md`).

**TUI** — `tcell`-based terminal UI. The `convocate` binary's
default mode. See [Using the TUI](guides/using-the-tui.md).
