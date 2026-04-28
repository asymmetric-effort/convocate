# Troubleshooting

Common operational problems and how to fix them. Each entry has the
**symptom**, the **likely cause**, and the **diagnostic / fix
sequence**.

## TUI shows "No agents registered"

**Symptom:** You boot `convocate`, the menu loads, but pressing
`(N)ew` shows the "No claude-agent hosts are registered. Run
'convocate-host init-agent' to add one." dialog.

**Likely cause:** No agents have been initialized yet, or the
`/etc/convocate/agent-keys/` directory is empty / missing.

**Fix:**

```bash
ls /etc/convocate/agent-keys/
# expect at least one subdirectory per registered agent
```

If empty, run [Adding a new agent](guides/adding-an-agent.md) for
each agent host. If the directory has subdirectories but they're
not appearing in the TUI, restart `convocate-status.service` so the
shell re-reads its config:

```bash
sudo systemctl restart convocate-status
```

## Session refuses to start with "port X/proto already in use"

**Symptom:** `(N)ew` fails with `port 53/udp already in use` or
similar.

**Likely cause:** Another session on the same agent has the same
port + protocol. Note that port collisions are checked **per
protocol** — `tcp:53` and `udp:53` can coexist.

**Fix:** either:

- Pick a different port (or `0` for none, or `-1` for auto-pick).
- `(D)elete` the conflicting session if it's no longer needed.
- Move one of the sessions to a different agent (delete + recreate;
  there's no auto-rebalance).

## Session refuses to start: "would push slice to N% (cap 90%)"

**Symptom:** Layer 1 admission control rejecting the create.

**Likely cause:** The aggregate of existing sessions plus the new
session exceeds 90% of agent host CPU or memory.

**Fix:** kill or delete an idle session; or pick a less-loaded
agent. If the message says you're already over 90%, the cap was
likely changed (e.g. by editing `convocate-sessions.slice`) without
re-running admission baselines — `(K)ill` something.

## Agent goes silent (no heartbeat for >90s)

**Symptom:** Sessions on a particular agent stop showing live state
in the TUI. List refresh works (so the RPC is fine), but
container.* events stop arriving.

**Diagnostic on the agent:**

```bash
systemctl status convocate-agent          # Active?
journalctl -u convocate-agent -n 100       # Errors?
```

If you see `dial tcp <shell-host>:223: ...` errors:

1. **Firewall.** Check the shell host's ufw rules: `sudo ufw status
   numbered`. Should show `tcp/223 ALLOW`. If not:
   `sudo ufw allow 223/tcp` on the shell host.
2. **Shell-side listener.** On the shell host:
   `systemctl status convocate-status`. If it's stopped:
   `sudo systemctl start convocate-status`.
3. **Stale auth keys.** Inspect `/etc/convocate/status_authorized_keys`
   on the shell. Confirm there's an entry for the agent in question.
   If you removed and re-added the agent without restart,
   `sudo systemctl restart convocate-status` to pick up the file
   change.

## Attach hangs / detaches immediately

**Symptom:** Press `Enter` on a session, the TUI flashes, returns
to the menu without showing the Claude prompt.

**Likely cause:** The container starts but tmux can't open. Usually
a Docker permission or image issue.

**Diagnostic on the agent:**

```bash
docker ps | grep convocate-session       # Container running?
docker logs convocate-session-<uuid>      # Any startup errors?
docker exec -it convocate-session-<uuid> id
                                          # Should show uid=1337(claude)
```

If the container exited, check:

- `cat /etc/convocate-agent/current-image` — does the image tag
  exist locally? (`docker images convocate`)
- `journalctl -u convocate-agent -n 30` for the actual `docker run`
  command and its output.

If the image is missing, re-run `convocate-host update --host
<this-agent>`.

## Stale lock — session shows `L` indicator forever

**Symptom:** Session is in the `L` state, won't transition out.
`(R)estart` and `Enter` both refuse with "session is locked."

**Fix:** Highlight the session, press `(O)verride`, confirm with
`Y`. The lock file gets removed; state goes to `-`.

If override doesn't work either, manually remove the lock file on
the agent:

```bash
ls -la /home/convocate/<uuid>.lock
sudo rm /home/convocate/<uuid>.lock
```

## Image transfer fails during `init-agent` or `update`

**Symptom:** `convocate-host` reports `image hash mismatch` or
`docker load: ...`.

**Likely cause:** SSH connection dropped mid-transfer; gunzip on
the agent received corrupt input.

**Fix:** Just rerun. The flow is idempotent and will re-tar, re-
transfer, and re-load. The temp file is cleaned up on success.

If it fails repeatedly, check:

- Is there enough disk space on the agent? `df -h /tmp` and
  `df -h /var/lib/docker`.
- Is the network stable? Sustained transfer of a ~1GB tarball over
  a flaky VPN will likely fail intermittently.

## DNS not resolving

**Symptom:** A session has a DNS name set but `dig
<dns-name>.<domain>` doesn't resolve, or resolves to the wrong IP.

**Diagnostic on the shell host:**

```bash
cat /var/lib/convocate/dnsmasq-hosts
# expect a line for every session with a DNS name
systemctl status dnsmasq
sudo dnsmasq --test
ss -lntp | grep ':53'
```

Common issues:

| Issue | Fix |
|---|---|
| dnsmasq not running | `sudo systemctl start dnsmasq` |
| `udp/53` already taken (systemd-resolved) | `sudo sed -i 's/^#DNSStubListener=yes/DNSStubListener=no/' /etc/systemd/resolved.conf` then restart resolved |
| Hosts file empty | The TUI didn't pick up the registration; recreate the session |
| Resolves to wrong IP | The agent's host changed; re-register the agent |
| Client not using shell as DNS | Configure the client's resolver to point at the shell host |

See [DNS and networking](guides/dns-and-networking.md) for the full
setup.

## Old image tags accumulating

**Symptom:** `docker images` on an agent shows a long list of
`convocate:v0.0.x` tags going back months.

**Likely cause:** The daily image-prune cron is failing or
disabled.

**Fix:**

```bash
ls -la /etc/cron.daily/convocate-image-prune
# expect mode 0755 (executable)

sudo /etc/cron.daily/convocate-image-prune
# manual run; should clean up

cat /var/log/syslog | grep image-prune       # any errors?
```

The prune script keeps any image tag referenced by a running
container, so as long as no session is actively running an old
tag, prune will remove it.

## TUI shows session as `O` (orphan)

**Symptom:** A session appears with `O` status. Most operations
fail.

**Likely cause:** This is a pre-v2 session directory that's still
on the shell host instead of an agent.

**Fix:** Run [Migrating orphans](guides/migrating-orphans.md):

```bash
sudo convocate-host migrate-session --agent <agent-id> --session <uuid>
```

## CRUD calls fail with "ssh: subsystem request failed"

**Symptom:** Most operations from the TUI return an error like
`subsystem "convocate-agent-rpc": request failed`.

**Likely cause:** The agent isn't running, or its host key changed
and the shell's pinned key is now invalid.

**Diagnostic:**

```bash
# On the agent:
systemctl status convocate-agent

# On the shell:
ls /etc/convocate/agent-keys/<agent-id>/host_key.pub
ssh-keyscan -t ed25519 <agent-host>:222
# Compare against the pinned host_key.pub
```

If the keys differ, re-run `init-agent` from the shell to refresh
the pinned host key.

## "make release" claims a tag already exists

**Symptom:** `make release` (or `release/minor`/`release/major`)
fails with `fatal: tag already exists`.

**Likely cause:** A tag pointing at HEAD already exists from a
previous release attempt or a manual `git tag` invocation.

**Fix:** look at `git tag -l` to see what's there. Either:

- Delete the existing tag if you want to re-release: `git tag -d
  vX.Y.Z` and `git push origin :refs/tags/vX.Y.Z`.
- Bump the version manually instead of using the Makefile target:
  `git tag vX.Y.Z+1 && git push origin vX.Y.Z+1`.

## Where to look first

When something's broken, walk this checklist top-down:

1. `systemctl status convocate-status` (shell host) — listener up?
2. `systemctl status convocate-agent` (each agent) — agents up?
3. `journalctl -u convocate-agent -n 100` — any error spikes?
4. `docker ps` (each agent) — containers in expected state?
5. `cat /etc/convocate-agent/current-image` — image tag matches
   what `docker images convocate` shows?
6. `ls /etc/convocate/agent-keys/` (shell) — every agent registered?
7. `ufw status` — required ports allowed?

90% of issues are caught by steps 1–4.
