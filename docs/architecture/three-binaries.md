# The three binaries

convocate ships three cooperating Go binaries, each with a distinct role
and a distinct host where it runs.

## `convocate` — the operator's TUI

**Where it runs:** the operator's laptop or the shell host (`ssh
convocate@shell-host` then it's already your login shell).

**What it does:**

- Renders the session menu (a `tcell`-based TUI; see
  [Using the TUI](../guides/using-the-tui.md))
- Holds one persistent SSH connection to each registered agent
  (`tcp/222`) and uses it for both the CRUD subsystem
  (`convocate-agent-rpc`) and PTY relay (`convocate-agent-attach`)
- Talks to the local `convocate-status` listener (`tcp/223`) so it
  picks up agent → shell events in real time
- Does **not** run containers itself. v2.0.0 made the shell a pure
  client; sessions live on agents

**What it doesn't do:**

- Hold any session state of its own. Everything authoritative lives on
  the agent that hosts a given session

## `convocate-agent` — the worker

**Where it runs:** every host that will actually host session
containers. One agent per machine.

**What it does:**

- Listens on `tcp/222` for SSH connections from the shell host
  (ed25519 keys; pubkey auth only)
- Accepts only two subsystems: `convocate-agent-rpc` (JSON-RPC for
  CRUD ops on sessions) and `convocate-agent-attach` (raw byte relay
  to a session's tmux PTY). Shell, exec, port forwarding, and any
  other SSH channel type are refused
- Runs `docker run` for new sessions, `docker exec` for attaches,
  `docker stop` for kills
- Enrolls every session container under
  `convocate-sessions.slice` so the kernel enforces the 90%
  aggregate CPU + memory cap (see
  [Capacity and isolation](capacity-and-isolation.md))
- Pushes status events back to the shell over a persistent SSH
  connection to the shell's `tcp/223` listener (the
  `convocate-status` subsystem)
- Forwards container logs to the shell over rsyslog/TLS on `tcp/514`

**Lifecycle:** systemd unit `convocate-agent.service`. Started by
`convocate-host init-agent`.

## `convocate-host` — the deploy tool

**Where it runs:** the operator's laptop or the shell host. It's a
one-shot CLI, not a service.

**What it does:** turns vanilla Ubuntu boxes into shell-host or
agent-host roles, and rolls updates across the cluster. Subcommands:

| Subcommand | Purpose |
|---|---|
| `install` | Prep a fresh Ubuntu host: apt packages, Docker, dnsmasq, `convocate` user, ufw, timezone, dist-upgrade, reboot if remote |
| `init-shell` | Deploy + configure the shell side: `convocate-status` systemd unit on `tcp/223`, rsyslog CA + server cert, ufw rule |
| `init-agent` | Deploy + configure an agent: `convocate-agent` binary, systemd unit, `convocate-sessions.slice`, image-prune cron, ed25519 peering keypairs (both directions), TLS client cert for syslog, transfer the current `convocate:<v>` image |
| `update` | Roll a new binary + image to one host. Existing containers keep running their original tag until `(R)estart` |
| `migrate-session` | Move a pre-v2 orphan session directory from the shell host to an agent |
| `create-vm` | Provision a vanilla Ubuntu host as a KVM hypervisor and bootstrap a new Ubuntu VM under it |

See [the CLI reference](../reference/cli/convocate-host.md) for full
flag listings.

## How they fit together

```
   ┌──────────────────────────┐
   │   operator's laptop       │
   │   ┌──────────────────┐   │
   │   │   convocate (TUI)│   │
   │   │     listens 223  │   │      ssh tcp/222   ┌───────────────────┐
   │   └──────────────────┘   │ ───────────────►   │   agent host A     │
   │                          │                    │   convocate-agent  │
   │   convocate-host CLI     │ ◄─── ssh tcp/223 ──│                    │
   │   (provisioning, updates)│                    │   docker run ...   │
   └──────────────────────────┘                    │   docker exec ...  │
                                                   └───────────────────┘
                                                   ┌───────────────────┐
                                                   │   agent host B     │
                                                   │   convocate-agent  │
                                                   └───────────────────┘
```

- **Solid arrow (shell → agent, `tcp/222`):** SSH connection the shell
  holds open per agent. CRUD calls and attach traffic both flow over
  this. Shell is the client.
- **Reverse arrow (agent → shell, `tcp/223`):** SSH connection the
  agent holds open back to the shell, used for status event push.
  Agent is the client.
- Not shown: rsyslog TLS on `tcp/514` for container-log forwarding.

The shell never directly addresses a container; it always asks the
agent to do something. This means an agent that goes offline takes
its sessions offline, but no agent's failure can affect another
agent's sessions.
