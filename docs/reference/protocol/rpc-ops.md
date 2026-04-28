# RPC ops

The full list of operations the agent exposes over the
`convocate-agent-rpc` SSH subsystem. See
[SSH subsystems](ssh-subsystems.md) for the framing.

Every request has the shape `{"op":"<name>","params":<json>}`, and
every response has `{"ok":true,"result":<json>}` or
`{"ok":false,"error":"<text>"}`.

## Core

### `ping`

Health check. Confirms the agent is alive and returns its identity.

**Params:** `null`

**Result:**
```json
{"agent_id":"abc123","version":"v0.0.1","server_time":"2026-04-26T18:00:00Z"}
```

Used by `CRUDClient.Ping()` and the heartbeat loop on the shell side.

## Session CRUD

### `list`

List all sessions on this agent. Stamps `attached: true|false` based
on the agent's local attach counter.

**Params:** `null`

**Result:** array of session metadata:
```json
[
  {
    "uuid":"5a2c...",
    "name":"refactor",
    "port":8080,
    "protocol":"tcp",
    "dns_name":"refactor",
    "created_at":"...",
    "last_accessed":"...",
    "attached":true,
    "running":true
  }
]
```

`running` reflects the result of `IsRunningFn` (default:
`docker inspect`); errors there are treated as `false` so a flaky
docker daemon doesn't break listing.

### `get`

Fetch one session by UUID.

**Params:** `{"id":"<uuid>"}`

**Result:** single session metadata (same shape as one entry in
`list`).

**Errors:** `session "<uuid>" not found` if no such session.

### `create`

Create a new session. Validates the request, generates a UUID,
writes the session directory + metadata. Does **not** start the
container — that happens lazily on attach.

**Params:**
```json
{"name":"refactor","port":8080,"protocol":"tcp","dns_name":"refactor"}
```

| Field | Required | Validation |
|---|---|---|
| `name` | yes | 1+ char, regex `[a-zA-Z0-9_-]+` |
| `port` | optional | `0` = none, `-1` = auto-pick ≥ 1001, otherwise must not collide on (port, protocol) |
| `protocol` | optional | `"tcp"` (default) or `"udp"` |
| `dns_name` | optional | lowercase hostname chars; must be globally unique |

**Result:** the newly-created session metadata.

**Side effects:**
- Writes `/home/convocate/<uuid>/session.json`
- Emits `container.created` status event

### `edit`

Update mutable fields on an existing session. Does not touch the
running container — changes apply on next `(R)estart`.

**Params:**
```json
{"id":"<uuid>","name":"new-name","port":9090,"protocol":"tcp","dns_name":"new-dns"}
```

Same field validation as `create`. Empty `name` keeps the existing
name (useful when editing only the port).

**Result:** the updated session metadata.

**Side effects:** Emits `container.edited` event.

### `clone`

Clone a session — copies the session directory to a new UUID.

**Params:**
```json
{"source_id":"<source-uuid>","name":"clone-name"}
```

**Result:** the new session's metadata.

**Side effects:**
- Walks the source directory and writes a copy under the new UUID
  (iterative; see the no-recursion rule)
- Sets `port: 0` on the clone to avoid collision
- Emits `container.created` for the clone

### `delete`

Permanently remove a session. Stops the container if running, then
removes the session directory.

**Params:** `{"id":"<uuid>"}`

**Result:** `null`

**Side effects:**
- `docker stop -t 10` if the container is running (best-effort)
- `rm -rf /home/convocate/<uuid>/`
- Emits `container.deleted`

## Lifecycle ops

### `kill`

Stop the running container without removing the session.

**Params:** `{"id":"<uuid>"}`

**Result:** `null`

**Side effects:**
- `docker stop -t 10`
- Emits `container.stopped` if stop succeeded

### `background`

Detach all currently-attached operators without stopping the
container. Implementation: `tmux detach-client -s convocate` inside the
container, which bumps every connected client off.

**Params:** `{"id":"<uuid>"}`

**Result:** `null`

**Side effects:** No status event (background isn't a state
transition; the container stays in the same state, just without
operators attached).

### `restart`

Start the container in detached mode. Refuses if it's already
running.

**Params:** `{"id":"<uuid>"}`

**Result:** `null`

**Errors:**
- `session "<name>" already running`
- `check running: <docker error>` (rare; flaky daemon)
- `start: <docker error>`

**Side effects:**
- `docker run --detach ...` with the session's persisted
  port/protocol/DNS
- Emits `container.started`

### `override`

Clear a stale lock file. Does not stop the container or kill any
process.

**Params:** `{"id":"<uuid>"}`

**Result:** `null`

**Side effects:**
- `rm /home/convocate/<uuid>.lock` if present
- No status event

## Settings (placeholders)

### `settings-get` / `settings-set`

Reserved for future per-agent settings. Currently return empty
results / accept anything as a no-op. Don't rely on the schema.

## Errors

All ops return an error response with `ok:false` and a free-form
`error` string. Common causes:

| Error substring | Likely cause |
|---|---|
| `session "<id>" not found` | Wrong UUID, or session was deleted |
| `port <n>/<proto> already in use` | Another session has the same port+protocol on this agent |
| `dns name "<name>" already in use` | Another session (possibly on another agent) has the same DNS name |
| `unknown op "<name>"` | Older agent doesn't support the op; you've upgraded the shell but not all agents |
| `decode params: <json error>` | Client sent malformed params |
| `check running: <docker error>` | Docker daemon issue on the agent |
