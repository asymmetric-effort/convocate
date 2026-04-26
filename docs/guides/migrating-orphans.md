# Migrating pre-v2 orphan sessions

If you upgraded an older `claude-shell` install (where the shell
host ran the session containers itself) to convocate v2.0.0+ (where
agents run them), any leftover `/home/claude/<uuid>/` directories on
the shell host are *orphans*. The TUI shows them with an `O` status
indicator and refuses most operations.

This guide walks through moving them onto an agent so they show up
as normal sessions again.

## When you need this

You only need this guide if **all** of these are true:

- You ran `claude-shell` (the original name) v1.x or earlier.
- You upgraded that same machine to convocate v2.0.0+.
- You've added at least one agent that can host sessions.
- The orphan sessions still have data you care about.

If you're on a clean v2+ install, there are no orphans and this page
doesn't apply.

## Inspect what's there

Look at what's under `/home/claude/` on the shell host:

```bash
ls -la /home/claude/
# expect to see UUID-named directories

cat /home/claude/<uuid>/session.json
# the metadata, if you want to see what each session was
```

Cross-reference with the TUI: any session shown as `O` is an orphan
that needs migration.

## Pre-flight

Before migrating an orphan:

1. **Stop any running container** for it on the shell host:
   ```bash
   docker stop convocate-session-<uuid>      # if it's still up
   ```
   (Pre-v2 used the same naming convention, so the container name
   matches.)
2. **Confirm the destination agent is healthy** in the TUI.
3. **Confirm you have NOPASSWD sudo on the shell host** — `migrate-
   session` runs locally on the shell with sudo and SSH's into the
   target agent.

## Run the migration

```bash
sudo convocate-host migrate-session \
    --agent <agent-id> \
    --session <uuid>
```

What this does:

1. **Refuse if the container is still running** locally. (Layer 1
   safety: don't migrate live data underneath a process that's
   writing to it.)
2. **Refuse if the destination agent already has a session with
   that UUID.** Should be impossible in a clean upgrade, but the
   check costs nothing.
3. **`tar` the session directory** at `/home/claude/<uuid>/` on the
   shell host into a temp tarball.
4. **SHA-256 the tarball** for integrity checking.
5. **`scp` the tarball** to a temp path on the agent.
6. **Verify SHA-256** on the agent side.
7. **Untar into `/home/claude/<uuid>/`** on the agent, with
   ownership rewritten to the agent's `claude` user.
8. **Update the session metadata** to record the new agent
   stamp (so the router will route subsequent ops to the
   correct agent).
9. **Remove the source directory** on the shell host.
10. **Clean up the temp tarball** on both ends.

## Verify

In the TUI on next 15-second refresh:

- The session is no longer in the orphan list.
- It shows up under the destination agent with `-` status (stopped).
- Press `(R)` or Enter to start it; should come up exactly as it
  was, with all conversation history and project files intact.

## Bulk migration

There's no `--all` flag. Wrap it:

```bash
for uuid in $(ls /home/claude/ | grep -E '^[0-9a-f]{8}-'); do
    echo "=== $uuid ==="
    sudo convocate-host migrate-session \
        --agent <agent-id> --session "$uuid"
done
```

If a particular session is still running locally, the loop will
fail on it and continue. Stop it (`docker stop ...`) and re-run.

## What if the migration fails partway?

The flow is *not* fully transactional. The recovery story:

| Failure point | State to clean up |
|---|---|
| Tarball creation | Nothing — source dir untouched |
| SCP transfer | Source dir untouched; remove temp file on agent |
| Untar on agent | Source dir untouched; remove half-extracted dir on agent |
| Agent metadata update | Both copies exist; pick the agent copy as authoritative, then `rm -rf` the source dir |
| Source removal | Both copies exist; `rm -rf` the source dir manually |

In all failure cases, the source directory survives until the very
last step, so you don't lose data — you might just have to clean up.

## Why is this a one-time tool?

Once your cluster is fully on v2+, every session is created on an
agent from the start. There's no path that produces a new orphan.
After you've migrated everything you care about, you can forget
this guide exists.
