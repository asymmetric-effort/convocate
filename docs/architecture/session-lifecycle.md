# Session lifecycle

A session is the unit of work convocate manages. Each session is one
Claude CLI process running inside its own Docker container on an agent
host, with persistent state under `/home/convocate/<uuid>/`.

## Identity

| Field | Source | Stable? | Used for |
|---|---|---|---|
| **UUID** | `uuid.New()` at create time | Always — the primary key | Container name, session dir, lock file, RPC `id` parameter |
| **Name** | Operator-provided in the create dialog | Renameable via `(E)dit` | Display in TUI |
| **Port** | Operator-provided; `0` means none, `-1` means auto-pick | Renameable via `(E)dit` | `-p HOST:CONTAINER/PROTO` flag at `docker run` |
| **Protocol** | `tcp` or `udp` (default `tcp`) | Renameable via `(E)dit` | Per-protocol port collision check (`tcp:53` and `udp:53` coexist) |
| **DNS name** | Operator-provided | Renameable via `(E)dit` | dnsmasq integration; `<dns-name>.<domain>` resolves to the agent host |
| **AgentID** | Stamped at creation by the router | Stable | Routing all subsequent ops back to the right agent |
| **CreatedAt** / **LastAccessed** | Set automatically | LastAccessed updates on attach | Sort order in the TUI |

## States

A session moves through these states. The TUI surfaces the state as a
single-character indicator in the leftmost session-list column:

| Char | Meaning | Detected by |
|---|---|---|
| `-` | Stopped (the container isn't running, lock file isn't held) | Default |
| `L` | **L**ock held only — `<uuid>.lock` exists with a live PID, but no container | `IsLocked` returns true, `IsRunning` returns false |
| `R` | **R**unning, detached — container is up, no client attached | `IsRunning` true, `isAttached` false |
| `C` | **C**onnected — running and at least one operator currently has a PTY attached | `IsRunning` true, `isAttached` true |
| `O` | **O**rphan — a session directory exists locally on the shell host but isn't claimed by any agent | The router can't find an agent for this UUID |

## Transitions

```
                            (N)ew                        attach (Enter)
   ┌──────────────────┐  ─────────────►  ┌───────────────┐ ──────────►  ┌─────────────┐
   │ no session       │                  │ stopped (-)   │              │ connected (C) │
   └──────────────────┘                  └───────────────┘              └─────────────┘
                                                ▲                             │
                                                │ (R)estart                  │ Ctrl-B D
                                                │                            │ (detach)
                                                │                            ▼
   ┌──────────────────┐                  ┌───────────────┐              ┌─────────────┐
   │ deleted          │ ◄── (D)elete ─── │ stopped (-)   │ ◄── (B)ack ──│ running (R) │
   └──────────────────┘                  └───────────────┘ ── (K)ill ──►└─────────────┘
```

- **Create** (`(N)ew`): allocate UUID, write metadata, create session
  directory, optionally `docker run` (attach flow does this lazily).
- **Restart** (`(R)`): `docker run --detach` for an existing session.
  Stays in the `R` state until someone attaches.
- **Attach** (`Enter`): from `-` or `R`, opens
  `convocate-agent-attach` over SSH, runs `tmux attach-session -t
  claude` in the container. State becomes `C` until the user detaches
  or kills.
- **Detach** (`Ctrl-B D`, the tmux detach chord, or `(B)ackground`):
  closes the SSH attach channel without stopping the container. State
  goes `C` → `R`.
- **Kill** (`(K)`): `docker stop -t 10` on the container. State goes
  `R` or `C` → `-`.
- **Delete** (`(D)`): kills if running, then removes the session
  directory and metadata. Permanent.
- **Override** (`(O)`): only relevant in the `L` state — clears a
  stale lock file when the owning PID is dead but `claude-shell`
  pre-v2 didn't clean up.

## Locking

Each session has a lock file at `<sessions-base>/<uuid>.lock`
containing the owning PID. Locks are:

- **Acquired** by `Manager.Lock(id)` when the TUI starts an action
  that mustn't race with another operator.
- **Detected as stale** when the owning PID is no longer alive, or
  when the lock's mtime is more than 24 hours old. `IsLocked`
  drops stale locks automatically.
- **Manually overridable** via `(O)verride` in the TUI when an
  operator is sure the lock isn't legitimate (e.g. previous agent
  process crashed without cleaning up).

## Per-session storage

On the **agent** host:

```
/home/convocate/<uuid>/         ← the session's home dir, mounted into the container
/home/convocate/<uuid>.lock     ← file lock (PID inside)
/home/convocate/<uuid>/session.json
                             ← persisted metadata (name, port, protocol, DNS, etc.)
```

In the **container**:

- `/home/convocate/` — bind-mounted from `/home/convocate/<uuid>/` on the host (read-write)
- `/home/convocate/.claude/` — the user's Claude CLI config, fresh per session
- `/home/convocate/.claude-shared/` — bind-mounted from `~/.claude/` on the host (read-only). This is how Claude Code account credentials are shared without each session having to log in
- `/home/convocate/.ssh/` — bind-mounted from the agent's `convocate` user (read-only)
- `/home/convocate/.gitconfig` — bind-mounted (read-only)
- `/usr/local/bin/claude` — bind-mounted from the agent's installed Claude CLI (read-only)
