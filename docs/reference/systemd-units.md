# Systemd units

convocate creates four systemd units across the cluster. Two on the
shell host, two on every agent.

## Shell host

### `convocate-status.service`

Drives the `tcp/223` SSH listener that accepts the
`convocate-status` subsystem from agents. Written by
`convocate-host init-shell`.

**Path:** `/etc/systemd/system/convocate-status.service`

**Inspection:**

```bash
systemctl status convocate-status
journalctl -u convocate-status -n 100 --no-pager
```

**Restart conditions:**

- After editing `/etc/convocate/status_authorized_keys` (when adding
  or removing an agent's pubkey)
- After regenerating the host key
- `Restart=on-failure` configured in the unit, with a backoff

**Lifecycle:**

```bash
sudo systemctl enable convocate-status     # Done by init-shell
sudo systemctl start  convocate-status     # Done by init-shell
sudo systemctl restart convocate-status    # After auth-key changes
```

### rsyslog (system service)

`/etc/rsyslog.d/10-convocate-server.conf` is a drop-in for the
**system rsyslog**. It's not a separate unit; convocate piggybacks
on the standard `rsyslog.service`.

**To pick up config changes:** `sudo systemctl restart rsyslog`.

## Agent host

### `convocate-agent.service`

The main agent service. Listens on `tcp/222`, processes RPC and
attach subsystems, pushes status events back to the shell. Written
by `convocate-agent install` (which is invoked by `convocate-host
init-agent`).

**Path:** `/etc/systemd/system/convocate-agent.service`

**Inspection:**

```bash
systemctl status convocate-agent
journalctl -u convocate-agent -n 200 --no-pager
journalctl -u convocate-agent -f         # tail -f equivalent
```

**Restart conditions:**

- `Restart=on-failure` with backoff
- After a `convocate-host update` (the update flow restarts the
  service)
- After editing `/etc/convocate-agent/authorized_keys` to register a
  new shell host
- After rotating any of the keys under `/etc/convocate-agent/`

**Notes:**

- The unit runs as **root**. The agent itself doesn't need root
  for most operations, but it does need to drive Docker (and on
  some setups Docker socket access requires root or membership in
  the `docker` group).
- Existing session containers keep running across an agent
  restart. The agent re-discovers them via `docker ps` when it
  comes back up.

### `convocate-sessions.slice`

A systemd slice (cgroup parent) for all session containers. Not a
service — it's a structural unit. Written by `convocate-agent
install`.

**Path:** `/etc/systemd/system/convocate-sessions.slice`

**Configuration:**

```ini
[Unit]
Description=convocate session containers (90% host cap)

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
CPUQuota=<nproc * 90>%        # e.g. 720% on an 8-core box
MemoryMax=<MemTotal * 0.9>    # absolute bytes
```

The agent passes `--cgroup-parent convocate-sessions.slice` to
every `docker run`, so every session container is a child of this
slice and the kernel enforces the aggregate cap.

**Inspection:**

```bash
systemctl status convocate-sessions.slice
systemctl show convocate-sessions.slice
```

**Editing:** generally don't. The `init-agent` flow regenerates
this file based on the host's actual CPU/memory; if you want
different limits, edit the unit and `systemctl daemon-reload`,
then restart `convocate-agent`. Note: at the next `init-agent` re-
run your edits will be overwritten.

### Cron unit (cron.daily)

`/etc/cron.daily/convocate-image-prune` runs once a day under
`anacron` / standard cron. Not a systemd unit; just a script the
agent's install step drops in.

**What it does:** removes `convocate:*` image tags that no
running container references. Existing containers hold their own
images alive across pruning.

**Disable:** `sudo chmod -x /etc/cron.daily/convocate-image-prune`.

### `rsyslog.service`

Same as on the shell host. Convocate's per-agent client
configuration is at `/etc/rsyslog.d/10-convocate-client.conf`,
forwarding container logs to the shell on `tcp/514` over TLS.

## Diagnostic flow when something's stuck

1. **`systemctl status convocate-agent` on each agent** — is the
   service running? If not, check the unit's `LoadState` and look
   at `journalctl -u convocate-agent`.

2. **`systemctl status convocate-status` on the shell host** — is
   the listener up? If not, the agent's status push will be
   failing in a loop.

3. **Cgroup pressure?** `systemctl show convocate-sessions.slice
   --property=MemoryCurrent,CPUUsageNSec`. If you're at the cap,
   sessions will start failing or get killed.

4. **`docker ps`** on the agent — are the containers actually
   running? If they are but the TUI shows them as `-`, the agent's
   `IsRunningFn` probe is misbehaving (rare; usually flaky docker
   daemon).

5. **`tail -f /var/log/convocate-agent/<id>.log`** on the shell
   host — is rsyslog forwarding working? Empty file when sessions
   are active means the TLS forwarder is broken; re-run
   `init-agent` to reissue the client cert.

## Removing all convocate units

If you want to fully remove convocate from a host:

```bash
# Agent:
sudo systemctl disable --now convocate-agent.service
sudo rm /etc/systemd/system/convocate-agent.service
sudo rm /etc/systemd/system/convocate-sessions.slice
sudo rm /etc/cron.daily/convocate-image-prune
sudo rm /etc/rsyslog.d/10-convocate-client.conf

# Shell:
sudo systemctl disable --now convocate-status.service
sudo rm /etc/systemd/system/convocate-status.service
sudo rm /etc/rsyslog.d/10-convocate-server.conf

# Both:
sudo systemctl daemon-reload
sudo systemctl restart rsyslog
```

This leaves session directories under `/home/claude/`, the
container image tags, and the binaries themselves untouched. Clean
those separately if you want a full removal.
