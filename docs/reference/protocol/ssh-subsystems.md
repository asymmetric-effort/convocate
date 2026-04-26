# SSH subsystems

convocate uses three named SSH subsystems for its control plane.
None of them carry shell access; each is a narrow protocol with a
specific framing.

| Subsystem | Direction | Listener port | Framing |
|---|---|---|---|
| `convocate-agent-rpc` | Shell → Agent | `tcp/222` | Newline-delimited JSON request/response |
| `convocate-agent-attach` | Shell → Agent | `tcp/222` | JSON header line, then raw bytes |
| `convocate-status` | Agent → Shell | `tcp/223` | Newline-delimited JSON event stream |

## `convocate-agent-rpc`

CRUD JSON-RPC over an SSH session channel. The shell opens a
session, requests this subsystem, writes one request, reads one
response, closes.

**Server:** `convocate-agent.service` on `tcp/222`.
**Client:** the `convocate` TUI, holding a persistent SSH connection
per agent and opening a fresh subsystem channel per RPC call.

### Request

A single JSON object on a single line, then EOF (the shell closes
its write half after sending):

```json
{"op":"<op-name>","params":<op-specific JSON>}
```

`params` may be `null` for ops that take no arguments.

### Response

A single JSON object on a single line, then EOF (the agent closes
its write half after sending):

**On success:**
```json
{"ok":true,"result":<op-specific JSON>}
```

**On error:**
```json
{"ok":false,"error":"<human-readable error>"}
```

`error` is a free-form string; clients show it directly to the
operator.

### Op names

See [RPC ops](rpc-ops.md) for the full list with parameter and
result schemas.

### Failure modes

| Symptom | Cause |
|---|---|
| `ssh: subsystem request failed` | Agent's SSH server rejected the subsystem name. Should never happen with a matching client/server. |
| `decode response: EOF` | Agent crashed mid-response. The client surfaces this; the operator retries. |
| `agent op "X": <error>` | Op-specific error returned by the agent. Surface text varies. |

## `convocate-agent-attach`

Raw PTY relay between the SSH channel and a session container's
tmux pseudo-terminal.

**Server:** `convocate-agent.service` on `tcp/222`.
**Client:** the `convocate` TUI when the operator presses Enter on
a session.

### Header

After the subsystem is opened, the **client writes** one JSON
object on one line, terminated by `\n`:

```json
{"id":"<session-uuid>","cols":120,"rows":40}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | string (UUID) | yes | Session UUID; must exist on this agent |
| `cols` | uint16 | no, default 80 | Initial terminal width |
| `rows` | uint16 | no, default 24 | Initial terminal height |

Unknown fields are silently ignored.

### Acknowledgment

The agent doesn't send an explicit ack. After processing the header
it runs `docker exec -it <container> sudo -u claude -- tmux attach-
session -t claude` and pipes the resulting PTY's bytes to the SSH
channel. If the container doesn't exist or `tmux attach` fails, the
agent writes a single line of JSON:

```json
{"ok":false,"error":"<reason>"}
```

…then closes the channel. The client treats either ok=false JSON or
an immediate channel close as an error and surfaces it to the TUI.

### Steady state

Once the PTY is up, every byte the operator types over the SSH
channel goes to the container's stdin; every byte the container
writes (Claude's output, tmux status line, etc.) flows back over
the SSH channel.

### Window resize

SSH's `window-change` channel request is honored. The agent applies
the new size to the PTY immediately so tmux re-renders at the new
dimensions. No higher-level message is needed.

### Termination

The channel closes when:

- The operator detaches (`Ctrl-B D`), which closes tmux's view but
  leaves the container running. The agent's `docker exec` exits;
  the channel closes cleanly.
- The container exits or is killed.
- The SSH connection drops.

The agent **does not stop the container** when an attach channel
closes. That's a separate operation (`(K)ill` from the TUI, which
makes a different RPC call).

## `convocate-status`

Newline-delimited JSON event stream. Agent → Shell.

**Server:** `convocate-status.service` on `tcp/223`.
**Client:** `convocate-agent.service`, holding a persistent SSH
connection back to the shell host.

### Connection

The agent opens an SSH session, requests this subsystem, and starts
writing events. The shell never writes anything back. The shell
closes the channel at process shutdown; the agent reconnects with
backoff (default 1s, doubling, capped at 30s) and resumes.

### Frame

Each event is a single JSON object on a single line:

```json
{"type":"container.started","agent_id":"<id>","session_id":"<uuid>","timestamp":"2026-04-26T18:42:11Z","data":<type-specific>}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `type` | string | yes | Event type; see [Status events](status-events.md) |
| `agent_id` | string | yes | Auto-stamped by the emitter from the agent's identity |
| `session_id` | string (UUID) | conditional | Required for `container.*` events; absent for `agent.*` events |
| `timestamp` | RFC3339 string | yes | UTC; auto-stamped by the emitter at publish time |
| `data` | JSON | optional | Event-type-specific payload (see [Status events](status-events.md)) |

### No backpressure

The emitter has a fixed-size queue (default 256 events). When the
queue is full, **events are dropped on the floor** with a log line on
the agent. This is deliberate: a stalled status channel must never
block a CRUD op. Dropped events are recovered at the next real event
or heartbeat (every 30s by default).

### Heartbeat

When configured with a non-zero heartbeat interval (default 30s),
the emitter publishes:

```json
{"type":"agent.heartbeat","agent_id":"<id>","timestamp":"...","data":null}
```

The shell-side listener uses heartbeat absence to detect a silent
agent (one whose status connection has died but whose `tcp/222`
listener may still be reachable).

### Malformed events

If the shell-side listener fails to decode a line, it logs and
**continues** — the bad event is dropped, the next valid event is
processed normally. This makes the protocol forward-compatible:
adding new event types or fields doesn't break older shells.
