# Security posture

This page maps the trust boundaries and what's authenticated where.
It's the ground truth for "who can do what" in a convocate cluster.

## Trust model

| Principal | Trusted to | NOT trusted to |
|---|---|---|
| Operator's `convocate` user (on the shell host) | Talk to any registered agent over SSH; create / kill / attach to sessions | SSH into agent hosts as a regular user; modify `/etc/convocate/*` (root-only) |
| Each `convocate-agent` process | Run docker locally on its own host; respond to requests on `tcp/222` | Talk to other agents; modify `/etc/convocate-agent/*` after install |
| `convocate` user inside a session container | Run sudo within the container; bind-mount the operator's `~/.claude/`, `~/.ssh/`, `~/.gitconfig` (read-only) | Touch the host's filesystem outside the session's bind mounts; reach other containers; sudo on the host |

## Cryptographic primitives

- **SSH host + client keys: ed25519 only.** Project-wide invariant —
  no RSA, no ECDSA. `convocate-host init-agent` mints two ed25519
  pairs per agent (shell→agent, agent→shell).
- **TLS for rsyslog: ECDSA P-256.** Issued by a CA minted under
  `/etc/convocate/rsyslog-ca/` during `init-shell`. Each agent gets
  its own client cert during `init-agent`.
- **No shared secrets across agents.** A compromise of one agent's
  keys never escalates to another agent.

## Authentication on every channel

| Channel | Server | Client | What's checked |
|---|---|---|---|
| Shell → Agent SSH (`tcp/222`) | Agent | Shell | Pubkey-only; client must present an ed25519 key whose pubkey is in the agent's `authorized_keys`. The agent's host key is pinned on the shell side at provisioning |
| Agent → Shell SSH (`tcp/223`) | Shell | Agent | Pubkey-only; agent presents an ed25519 key whose pubkey is in the shell's `status_authorized_keys`. The shell's host key is pinned on the agent side |
| Agent → Shell rsyslog (`tcp/514`) | Shell | Agent | Mutual TLS — client cert must chain to the per-cluster rsyslog CA; server cert is signed by the same CA |

## Channel hardening

`convocate-agent`'s SSH server is intentionally narrow:

- No password auth
- No keyboard-interactive auth
- No `none` auth method
- Only ed25519 host + client keys
- Only the `convocate-agent-rpc` and `convocate-agent-attach` SSH
  subsystems are accepted; **shell**, **exec**, **direct-tcpip**,
  **forwarded-tcpip**, **env**, **pty-req**, and any other channel
  request is refused with a protocol-level rejection
- Channel close drops anything left in flight

Same posture on the shell's `tcp/223` listener: only the
`convocate-status` subsystem is accepted; everything else is refused.

## Container privilege model

Inside a session container:

- `convocate` user runs everything; UID/GID match the host's `claude`
  user (typically 1337 on convocate hosts) so file ownership stays
  consistent across the bind mount
- `claude` has NOPASSWD sudo **inside the container only**
- Container does **not** run with `--privileged`
- No bind mounts go up to the host root or to `/etc`, `/var`, etc.
- The Docker socket (`/var/run/docker.sock`) is bind-mounted *only*
  to enable Docker-in-Docker for development (e.g. `docker compose`
  inside a session); a malicious session that uses it could escape
  to the host. **This is documented as a known trust boundary.** If
  you're running untrusted code, drop the docker-socket mount in
  `internal/container/container.go`.

## What the operator can do that bypasses the controls

- Run arbitrary commands as `claude` on the agent host via direct
  SSH (assuming the operator has agent-side SSH access). convocate
  doesn't lock down host SSH.
- Bypass the Layer 1 admission cap by `docker run`-ing directly. The
  Layer 2 cgroup cap still applies, so the kernel enforces the
  aggregate 90%, but the new container won't show up in the TUI.
- Read agent-side log files directly, bypassing the rsyslog forwarder.

These are deliberate. The operator is trusted; controls are aimed at
limiting what *Claude itself* can do from inside a session.

## What's NOT secured

- **No network policy between sessions.** Two sessions on the same
  agent can reach each other on the bridge network. If you want
  isolation at the L3 level, run those sessions on different agents.
- **No quota on disk usage** beyond Layer 1's coarse pre-flight
  check. A runaway session can still fill `/home/convocate` on the
  agent. Monitor `/var/log/convocate-agent/<id>.log` for warnings.
- **No sandboxing of the Claude CLI itself.** Once Claude has a
  shell, it has full container privileges (as `claude`). Per-action
  policy belongs to Claude Code's tool permissions, not convocate.

## Threat model

convocate is **not** designed to defend against:

- A compromised operator workstation (full game over — they hold
  every shell-side key)
- A compromised agent host (the agent's keys + image cache are
  enough to impersonate that agent until you rotate)
- Malicious code running as the `convocate` user in a session
  container, escaping via the docker-socket bind mount (see above)

It **is** designed to defend against:

- An eavesdropper on the wire (TLS + SSH on every channel)
- A compromised non-agent peer trying to impersonate an agent (per-agent
  ed25519 keys, host-key pinning)
- An accidental runaway session starving the agent (Layer 1 + Layer 2
  caps)
- Subsystem-namespace abuse over SSH (only the named subsystems are
  accepted; everything else is rejected at the protocol level)
