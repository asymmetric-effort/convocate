# Repo guidance for Claude Code

This file is read by Claude Code at the start of every session in this repo.
Keep it terse; the goal is to orient you, not to duplicate the source.

## Organization-wide coding standards

**Authoritative reference:** <http://coding-standards.asymmetric-effort.com/>

That site is the source of truth for how code is written across every
asymmetric-effort project. Read it before touching anything; the
project-specific Conventions section below augments it but does not
override it. When this file disagrees with the standards site, the
standards site wins — fix this file.

## What this project is

`convocate` is a Go CLI that runs the Claude CLI inside per-session Docker
containers. Users pick a session from a `tcell`-based TUI menu; each session
is a UUID-named directory on the host, a tmux session inside a container, and
optional metadata (published port, protocol, DNS name).

Binary entry point: `cmd/convocate/main.go`. Everything else is under
`internal/`.

## Build, test, lint

```
make build          # compiles to ./build/convocate
make test           # go test ./...
make lint           # go vet + yaml lint
make clean lint test build install release   # full ship cycle
```

- `make install` copies the binary to `/usr/local/bin/convocate` and
  configures the login shell. Requires `sudo`.
- `make release` tags the next patch version and pushes the tag. Be aware
  that `git describe --tags --abbrev=0` picks the first reachable tag, so
  if you've manually created tags on the same commit the bump logic can
  collide — check `git tag` if `make release` errors with "tag already
  exists".

## Package map

| Path | Role |
|---|---|
| `cmd/convocate` | CLI entry (`run`, `runSessionManager`, handlers) |
| `internal/menu` | `tcell` TUI: session list, create/edit form, action dialogs |
| `internal/session` | `Manager` + `Metadata` persisted as `session.json` |
| `internal/container` | `docker run/exec/inspect/stop` wrappers around sessions |
| `internal/capacity` | Refuses new container starts when CPU or memory >= 80% |
| `internal/dns` | Rewrites `/var/lib/convocate/dnsmasq-hosts` from session DNS names |
| `internal/install` | The `convocate install` subcommand |
| `internal/config` | Paths, constants (container name prefix, user, sockets) |
| `internal/logging` | syslog wrapper |
| `internal/assets` | Embedded Dockerfile + entrypoint.sh |
| `internal/skel` | Session skeleton directory contents |
| `internal/user` | `user.Lookup` wrapper for the `claude` user |
| `internal/diskspace` | Free-space check for the build context |

## TUI shape (internal/menu/tui.go)

The TUI is a single-screen event loop built on `tcell`.

- Menu bar keys: `(N)ew  (C)lone  (E)dit  (B)ackground  (D)elete  (K)ill  (O)verride  (S)ettings  (R)estart  (Q)uit`.
- Session list shows `Port/Proto` (e.g. `53/udp`) and a status indicator:
  `C` = terminal connected, `R` = running detached, `L` = lock held only,
  `-` = stopped.
- Auto-refresh: 1 s tick for the clock/status; 15 s reload of the session list.
- Create and Edit share one form (`drawSessionFormDialog`): Name → Protocol
  → Port → DNS Name. Tab/Shift+Tab cycles. Protocol is a toggle (`t`/`u`/
  Space). DNS Name accepts hostname chars only; lowercased on input.
- Restart flow: `(R)estart` prompts; on Y, the container is started detached
  via `Runner.StartDetached` and a "Restarting…" notification appears that
  only dismisses on Enter. Pressing Enter later on a running session
  attaches.
- Error dialogs wrap long docker stderr via `wrapErrorText` (500-char cap)
  and widen to `errorDialogMinWidth = 72` when an error is displayed.

Common test helpers: `newTestScreen` (110x30), `newWideTestScreen` (120x30).
Use the wide one when an assertion touches trailing columns of the session
table or the full menu bar.

## Session persistence (internal/session/session.go)

Session directories live at `<paths.SessionsBase>/<uuid>/` with a
`session.json` holding `Metadata`:

```go
type Metadata struct {
    UUID, Name string
    CreatedAt, LastAccessed time.Time
    Port int              // 0 = none; -1 (PortAuto) = auto-pick >= 1001
    Protocol string       // "tcp" | "udp"; empty reads as "tcp"
    DNSName string        // optional, lowercase hostname
}
```

Use `CreateWithOptions` / `UpdateWithOptions` for new code; the older
`CreateWithPort`, `CreateWithPortProtocol`, `Update(id, name, port, proto)`
wrappers delegate to them. Port collisions are checked per-protocol —
`tcp:53` and `udp:53` coexist. DNS collisions are checked globally.

Locks are file-based (`<uuid>.lock`) and encode the owning PID; they're
stale-cleaned when the PID is dead or the mtime is older than 24h.

## Container invariants

`container.Runner.buildRunArgs` always emits `--rm --detach` and
`-p HOST:CONTAINER/PROTO` when a port is set. `Runner.Start` does docker
run + `docker exec -it tmux attach-session`; `Runner.StartDetached` skips
the attach. `DetachClients` sends `tmux detach-client -s claude` inside the
container to background a connected user without stopping the container.

## Things that have tripped us up

- **Dialog error truncation** — the old single-line error clip hid docker's
  real message. Route new error-surfacing dialogs through `wrapErrorText` +
  `sizeDialogForError`.
- **Menu-bar width** — menu bar + new labels already push ~100 cols. The
  test screen width is 110; keep it under that when adding labels or bump
  both in lockstep.
- **`make release` picks the first tag** — if a tag for the same commit
  already exists (e.g. from a prior manual tag), the bump can clash. Delete
  the stray tag or tag the next version by hand rather than fighting the
  Makefile.
- **Privileged ports** — `-p 53:53` on a host running `systemd-resolved`
  will fail to bind. The 80% capacity cap doesn't help here; either disable
  `DNSStubListener` or use a non-privileged host port.
- **sed regex over-match** — an earlier bulk rename of
  `CreateWithUUID(x, y)` → `CreateWithUUID(x, y, 0)` matched
  `func TestCreateWithUUID(t *testing.T)` too. When doing wide refactors
  across tests, spot-check the diff for test function declarations.

## Conventions

- **No recursion.** Go has no tail-call optimization, so every recursive
  call grows the goroutine stack and can panic on adversarial input
  (deep trees, cyclic data, hostile filesystem layouts). For an
  orchestrator that holds long-lived session state, that's
  unacceptable. Use loops, explicit stacks/queues, `filepath.WalkDir`,
  or work-list patterns instead. Mutual recursion (A → B → A) is also
  forbidden — same problem. If you find yourself reaching for a
  helper that calls itself, stop and rewrite as iteration.
  Reference implementation: `session.copyDir` (uses an explicit
  work-list slice).
- **No speculative abstractions.** Fix what's in front of you; the next
  caller will refactor when it actually shows up.
- **Don't write docstrings explaining what code already says.** Comment
  the *why* when it's non-obvious (hidden invariant, workaround, past
  incident).
- **Tests verify behavior, not implementation.** When you change a
  signature, update callers and the failing tests; don't relax assertions.
- **Coverage targets** — aim for 90%+ in business-logic packages (`menu`,
  `session`, `container`, `capacity`, `dns`). Installer and cmd/main have
  interactive/root-only paths that can't reasonably be unit tested.

## Current version

See `git describe --tags` at read time. The latest tag at the time this
file was written is **v2.0.0** (2026-04-24). `Version` is set via
`-ldflags "-X main.Version=$(VERSION)"` in the Makefile and tagged onto
the built container image (`convocate:<semver>`) by
`convocate install`.

Release history:
- `v1.0.0` — multi-host orchestration arc (convocate-host + convocate-agent
  deployed; SSH peering; rsyslog TLS; agent-aware TUI).
- `v2.0.0` — "shell is pure client" arc. convocate no longer runs
  containers; all sessions live on agents. Image built on shell,
  pushed to each agent. Orphan migration via
  `convocate-host migrate-session`. `convocate-sessions.slice` 90% cgroup
  cap. See `docs/v2.0.0.md` for the full plan + architectural
  snapshot + known limitations.
