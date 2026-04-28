# Updating the cluster

How to roll a new convocate version across all your agents.
"Update" here means: new binary, new container image, new systemd
unit if anything changed.

## Build first, distribute second

Updates always start at the shell host. Pull the new code, rebuild,
and reinstall locally:

```bash
cd ~/convocate
git pull
make build
sudo make install
```

`make install` does:

1. `cp build/convocate*` → `/usr/local/bin/`
2. Runs `convocate install` to rebuild the container image with the
   new tag (the version string from `git describe --tags`)

After this, the shell host is on the new version. Existing agents
are still on the previous one until you push.

## Push to each agent

```bash
sudo convocate-host update --host <agent-host>
```

What this does:

1. **Verify connectivity.** SSH into the agent. Refuse to proceed if
   the connection fails or the agent's SSH host key has changed
   (the shell pins host keys at `init-agent` time).
2. **Refuse if a container is running on the old image.** Or rather:
   it warns and proceeds; the new image lives alongside the old
   tag, and existing containers keep using their original tag until
   `(R)estart`. This is the cluster-wide rolling upgrade story.
3. **Deploy the new binary.** Copies `convocate-agent` to
   `/usr/local/bin/convocate-agent`.
4. **Reload the systemd unit if it's been regenerated.** `convocate-
   agent install` rewrites the unit file each time; if the contents
   changed (new flags, new dependencies), `systemctl daemon-reload`
   + `systemctl restart convocate-agent.service`.
5. **Transfer the new container image.** Same `docker save | gzip |
   scp | docker load` flow as in `init-agent`, with SHA-256
   verification.
6. **Stamp `/etc/convocate-agent/current-image`** with the new tag.
   Subsequent `(R)estart` actions on existing sessions will pick
   this up; new `(N)ew` sessions get it immediately.
7. **Restart the agent service** so the new binary is in process.
   Existing session containers keep running across the restart;
   the agent re-discovers them on startup via `docker ps`.

## All agents in one shot

There's no built-in `--all` flag (yet). Wrap it in shell:

```bash
for agent in agent-a agent-b agent-c; do
    echo "=== $agent ==="
    sudo convocate-host update --host "$agent" || break
done
```

The `|| break` makes the loop stop on the first failure, so you can
investigate and resume rather than push a half-broken cluster.

## What survives an update

These persist across `update`:

- Every running session container (with its original image tag).
- Every session directory under `/home/convocate/<uuid>/`.
- All ed25519 keys (host key, peering keys).
- All TLS certs (rsyslog CA + client).
- The `convocate-sessions.slice` cgroup config (rewritten only if
  the underlying `init-agent` template changed).
- The agent's `agent-id`.

## Cutting sessions over to the new image

After `update`, existing sessions are still on their original image.
To move a session to the new image:

1. From the TUI, highlight the session.
2. `(K)` to kill the container.
3. Wait until the row shows `-`.
4. `(R)` to restart.
5. The new container is built from the freshly-stamped image tag.

Do this session-by-session as opportunity allows — there's no
"restart all sessions on this agent" button by design. Every cutover
is a deliberate operator decision.

## When `update` is not enough

If you've changed something in the systemd-unit *template* such that
the previously-installed unit can't be reloaded cleanly, run
`init-agent` again on that host. It's idempotent and will overwrite
anything that's drifted.

If you've changed the rsyslog/TLS pipeline, run `init-shell` first
(to regenerate CA artifacts), then `init-agent` for each agent
(to reissue client certs).

## Rolling back

There is no rollback command. The image-prune cron deletes old
image tags daily, so even if you saved the old tarball, after one
prune cycle you'd have to rebuild the previous version from source.

If you need to roll back:

```bash
# On the shell host:
git checkout v0.0.x          # the version you want to return to
make build
sudo make install            # rebuilds image with that version's tag

# On every agent:
sudo convocate-host update --host <agent>
```

Sessions running on a tag that's already been pruned will keep
running (Docker holds the image alive while a container references
it), but they can't be `(R)estart`ed without the operator first
re-pulling that tag.
