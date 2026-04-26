# Status events

Events the agent pushes to the shell over the `convocate-status`
SSH subsystem. See [SSH subsystems](ssh-subsystems.md) for the
transport.

All events share the envelope:

```json
{
  "type": "<type-name>",
  "agent_id": "<auto-stamped agent id>",
  "session_id": "<uuid, when applicable>",
  "timestamp": "<RFC3339 UTC>",
  "data": <type-specific payload>
}
```

`agent_id` and `timestamp` are stamped by the emitter at publish
time; CRUD callers don't need to fill them. `session_id` is set for
all `container.*` events; absent for `agent.*` events.

## Agent lifecycle events

### `agent.started`

Emitted once by `convocate-agent serve` when its main loop is up
and listening.

| Field | Type | Notes |
|---|---|---|
| `data.version` | string | The agent binary's version (`v0.0.x`) |
| `data.image_tag` | string | The active image tag from `/etc/convocate-agent/current-image` |

```json
{
  "type": "agent.started",
  "agent_id": "abc123",
  "timestamp": "2026-04-26T18:00:00Z",
  "data": {"version":"v0.0.1","image_tag":"convocate:v0.0.1"}
}
```

### `agent.heartbeat`

Emitted at the configured heartbeat interval (default 30s).

| Field | Type | Notes |
|---|---|---|
| `data` | null | No payload — presence is the signal |

The shell uses heartbeat absence (e.g., > 90s gap) to mark an agent
as silent in the TUI.

### `agent.shutdown`

Emitted by the agent's signal handler when receiving SIGTERM /
SIGINT. Best-effort: a hard kill won't generate it.

```json
{
  "type":"agent.shutdown",
  "agent_id":"abc123",
  "timestamp":"...",
  "data":{"reason":"SIGTERM"}
}
```

## Container lifecycle events

All `container.*` events have `session_id` set.

### `container.created`

Emitted from the agent's `Create` RPC handler immediately after
`Manager.Create` returns successfully, **before** any `docker run`.

| Field | Type | Notes |
|---|---|---|
| `data` | object | Full session metadata (matches `session.json`) |

```json
{
  "type":"container.created",
  "agent_id":"abc123",
  "session_id":"5a2c0f81-...",
  "timestamp":"...",
  "data":{
    "uuid":"5a2c0f81-...",
    "name":"refactor-x",
    "port":8080,
    "protocol":"tcp",
    "dns_name":"refactor-x",
    "created_at":"...",
    "last_accessed":"..."
  }
}
```

### `container.edited`

Emitted from `Edit` RPC handler after `Manager.UpdateWithOptions`.

`data` is the full updated session metadata, same shape as
`container.created`.

### `container.started`

Emitted by the agent after a successful `docker run` (during
attach if the container was stopped, or after `(R)estart`).

`data` is the session metadata. Use this to detect when a session
transitions from `-` to `R`.

### `container.stopped`

Emitted after `docker stop` returns successfully (from `(K)ill` or
`(B)ackground`'s container-stop variant).

`data` is `null` — only the `session_id` is informational.

### `container.deleted`

Emitted after `Manager.Delete` removes the session directory.

`data` is `null`.

## Field reference

### Session metadata shape

Used as `data` in `container.created` and `container.edited`:

```json
{
  "uuid": "string (UUID v4)",
  "name": "string",
  "port": 0,                // int. 0 = no published port
  "protocol": "tcp",        // "tcp" | "udp"
  "dns_name": "string",     // empty string = no DNS registration
  "created_at": "RFC3339",
  "last_accessed": "RFC3339",
  "agent_id": "string",     // stamped by router on the shell side
  "agent_host": "string"    // ditto
}
```

`agent_id` and `agent_host` may be empty in events emitted by the
agent itself (the agent doesn't know its own ID at the metadata
layer); the shell side stamps them as it builds its session list.

## Event ordering guarantees

- **Per session:** events are emitted in the order operations
  happen on the agent. `container.created` always precedes the
  first `container.started` for a session; `container.deleted` is
  always last.
- **Cross session, same agent:** ordered. `container.created` for
  session A and `container.started` for session B can interleave,
  but their relative order is the order operations occurred on
  the agent.
- **Cross agent:** no ordering. The shell sees events from each
  agent in arrival order, with no synchronization between agents.

## Loss model

- **In-flight loss:** if the agent's status connection dies
  mid-batch, the emitter's queue keeps draining and reconnects.
  Events queued before the disconnect are sent over the new
  connection.
- **Backpressure loss:** when the queue (default 256 events) is
  full, new events are dropped with a log line on the agent.
- **Restart loss:** events emitted while the shell-side listener
  is down are lost — the agent's queue empties on emitter close,
  it doesn't persist to disk.

The TUI tolerates loss because it does a full session-list refresh
every 15 seconds via the CRUD `list` op. Status events are an
optimization for liveness, not the source of truth.
