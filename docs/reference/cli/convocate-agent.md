# `convocate-agent`

The agent worker binary. Runs as a systemd service on every host
that hosts session containers. Started by `convocate-host init-agent`,
managed by systemd thereafter.

## Synopsis

```
convocate-agent install
convocate-agent serve
convocate-agent version
convocate-agent help
```

## `install`

```
convocate-agent install
```

Bootstraps the local host as a convocate agent. Idempotent.
**You normally don't run this directly** — `convocate-host init-agent`
SSH's into the target and invokes it.

**What it does:**

- Verifies Docker is present and the daemon is reachable.
- Verifies the `convocate` user exists; creates it (UID 1337) if not.
- Creates `/etc/convocate-agent/`.
- Generates a stable agent ID (8-char random string) at
  `/etc/convocate-agent/agent-id` if absent.
- Generates an ed25519 SSH host key at
  `/etc/convocate-agent/ssh_host_ed25519_key` if absent.
- Writes the systemd unit at `/etc/systemd/system/convocate-agent.service`.
- Writes `/etc/systemd/system/convocate-sessions.slice` with
  `CPUQuota = nproc*90%` and `MemoryMax = MemTotal*90%` based on the
  current host's resources.
- Drops `/etc/cron.daily/convocate-image-prune`.
- Drops `/etc/logrotate.d/convocate-agent-logs`.
- Counts adopted sessions (existing `/home/convocate/<uuid>/` dirs)
  for diagnostic output.

**Requires:** `sudo`. Touches `/etc/convocate-agent/`,
`/etc/systemd/system/`, `/etc/cron.daily/`, `/etc/logrotate.d/`.

**Does NOT do:**

- Generate the peering keypairs (those are minted by `init-agent` on
  the shell side and copied over).
- Install the rsyslog TLS client cert (also `init-agent`'s job).
- Start the service (`init-agent` runs `systemctl restart` after
  the keys are in place).

## `serve`

```
convocate-agent serve [--listen :222] \
    [--host-key PATH] [--auth-keys PATH] \
    [--shell-host HOST] [--shell-port :223] \
    [--shell-key PATH] [--shell-known-host PATH]
```

Runs the agent's main loop. **The systemd unit invokes this with the
right flags** — you don't normally run it from the command line.

What it does:

1. **Boots the SSH listener** on `--listen` (default `:222`).
2. **Loads the host key** from `--host-key`
   (default `/etc/convocate-agent/ssh_host_ed25519_key`).
3. **Loads authorized keys** from `--auth-keys`
   (default `/etc/convocate-agent/authorized_keys`).
4. **Reads the agent's identity** from
   `/etc/convocate-agent/agent-id`.
5. **Reads the active image tag** from
   `/etc/convocate-agent/current-image` and uses it for every
   `docker run` call.
6. **Starts the status emitter**: opens a persistent SSH connection
   to `--shell-host:--shell-port` (default `tcp/223`), reconnects
   on failure with exponential backoff. Every CRUD op publishes a
   status event over this connection.
7. **Accepts sessions** on the SSH listener. Only the
   `convocate-agent-rpc` and `convocate-agent-attach` subsystems
   are accepted; everything else is refused with a protocol-level
   reject.

**Flag defaults** (most important):

| Flag | Default | Purpose |
|---|---|---|
| `--listen` | `:222` | SSH listener |
| `--host-key` | `/etc/convocate-agent/ssh_host_ed25519_key` | Server host key |
| `--auth-keys` | `/etc/convocate-agent/authorized_keys` | Pubkeys allowed to connect (one per shell) |
| `--shell-host` | reads `/etc/convocate-agent/shell-host` | Where to push status events |
| `--shell-port` | `223` | Shell-side status listener port |
| `--shell-key` | `/etc/convocate-agent/agent_to_shell_ed25519_key` | Agent's private key for status push |
| `--shell-known-host` | reads pinned host key from `/etc/convocate-agent/` | Pinned shell host key for status push |

Logs go to systemd journal. The agent forwards container output
separately via rsyslog/TLS (see
[DNS and networking](../../guides/dns-and-networking.md)).

## `version`

```
convocate-agent version
```

Prints `convocate-agent version v0.0.x`.

## `help`

```
convocate-agent help
```

Prints a short usage summary.

## Lifecycle

Normal lifecycle is:

1. `convocate-host init-agent` runs `convocate-agent install` on the
   target (one-time setup).
2. `init-agent` writes pubkeys + private keys + cert chain to
   `/etc/convocate-agent/`.
3. `init-agent` runs `systemctl daemon-reload && systemctl enable --
   now convocate-agent.service`.
4. systemd thereafter restarts `convocate-agent` on failure (the unit
   has `Restart=on-failure` + a backoff). The agent comes up,
   re-discovers existing session containers via `docker ps`, and
   reconnects to the shell.

To stop the agent:

```bash
sudo systemctl stop convocate-agent.service
```

Existing session containers keep running across an agent stop —
they're owned by Docker, not by the agent process. They'll keep
serving attached operators (via the shell's already-attached SSH
connections); new attaches will fail until the agent comes back.
