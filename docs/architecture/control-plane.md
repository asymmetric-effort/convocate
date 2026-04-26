# Control plane

The control plane is what flows between the shell host and each agent
host. There are three channels and three TCP ports.

## Summary

| Direction | Port | Transport | Subsystem | Purpose |
|---|---|---|---|---|
| Shell → Agent | `tcp/222` | SSH (ed25519) | `convocate-agent-rpc` | JSON-RPC: CRUD ops on sessions |
| Shell → Agent | `tcp/222` | SSH (ed25519) | `convocate-agent-attach` | Raw byte relay to tmux PTY inside session container |
| Agent → Shell | `tcp/223` | SSH (ed25519) | `convocate-status` | Newline-delimited JSON event stream |
| Agent → Shell | `tcp/514` | rsyslog over TLS (ECDSA P-256) | n/a | Container log forwarding |

Each direction is a **persistent connection** — the shell holds one
SSH session open to each agent on `:222`, and each agent holds one SSH
session open back to the shell on `:223`. Reconnect with exponential
backoff (capped at 30s) on disconnect.

## Why two ports for SSH?

The agent's role is "let the shell drive me," so it listens on `:222`.
The shell's role is "let agents push me events," so it listens on a
separate port `:223` to keep the listeners distinct on a host that
runs both roles (single-box deployments are common).

A single host running both shell and agent has two SSH listeners
running — one on `:222`, one on `:223` — plus the host's regular SSH
on `:22` used by humans. They don't collide.

## Authentication

Every channel is mutually authenticated using **ed25519** keypairs
that `convocate-host init-agent` mints at provisioning time:

```
shell-side keypair        ──── used for shell → agent SSH (tcp/222)
agent-side keypair        ──── used for agent → shell SSH (tcp/223)
agent client TLS cert     ──── used for agent → shell rsyslog (tcp/514)
                              signed by the shell-side rsyslog CA
```

There is **no shared key** across the cluster. If one agent is
compromised, its keys don't open up any other agent.

The shell side stores agent metadata under
`/etc/convocate/agent-keys/<agent-id>/`. Each subdirectory holds:

- `agent-host` — the agent's SSH host (a string)
- `agent_to_shell_ed25519_key.pub` — the agent's public key, used to
  authenticate the agent when it dials our `tcp/223` listener
- `shell_to_agent_ed25519_key` (private) and `.pub` — the shell's
  side of the shell→agent SSH channel
- `host_key.pub` — the agent's SSH host key, pinned

## SSH server invariants

The agent's SSH listener (`internal/agentserver`) is locked down
hard:

- **Only ed25519 keys** for both client and host
- **Only the two named subsystems** (`convocate-agent-rpc`,
  `convocate-agent-attach`) are accepted
- **No shell, no exec, no env**, no port forwarding, no X11 — every
  other channel type is refused with an SSH protocol-level rejection
- **No password auth, ever** — pubkey only

The same posture applies to the shell's `tcp/223` listener
(`internal/shellserver`): only the `convocate-status` subsystem is
accepted, no other channel types.

## Subsystem framing

### `convocate-agent-rpc` (JSON-RPC)

Each request is a single JSON object on a line, response is a single
JSON object on a line. The connection is closed after one request/
response pair (the SSH connection itself is persistent, but each RPC
opens a fresh subsystem channel).

```json
// request
{"op":"list","params":null}

// response (success)
{"ok":true,"result":[{ ...session metadata... }]}

// response (error)
{"ok":false,"error":"agent op \"create\": port 53/udp already in use"}
```

Op names are listed in [RPC ops](../reference/protocol/rpc-ops.md).

### `convocate-agent-attach` (raw PTY)

After the subsystem is requested, the client writes a single
JSON header line:

```json
{"id":"<session-uuid>","cols":120,"rows":40}
```

…then the channel becomes a raw byte pipe between the SSH session and
`docker exec -it <container> sudo -u claude -- tmux attach-session -t claude`.
Window-change events on the SSH channel are forwarded to the PTY.

### `convocate-status` (newline-JSON event stream)

Agent → Shell. After the subsystem is opened, the agent writes one
JSON event per line. No reply expected; the shell's listener drains
the stream and dispatches each event to its handler.

Event types are listed in [Status events](../reference/protocol/status-events.md).

## Reconnect behavior

Both directions implement the same pattern:

1. Dial. If success, run.
2. On any error (read EOF, SSH disconnect, write fail) → close,
   sleep `backoff`, redial.
3. `backoff` doubles on each failure (capped at the configured max,
   default 30s) and resets to the initial value (default 1s) on
   any successful connection.

This means a transient network blip shows up as a brief gap in events
+ failed CRUD calls, and recovers automatically. A *durable* break
(e.g. wrong key, wrong port, firewall) just keeps logging
"redial failed" until you fix the underlying problem.
