# Adding a new agent host

This is the end-to-end flow for getting a fresh Linux box into your
convocate cluster as a new agent. Plan on ~15 minutes start to finish
on a normal connection, plus however long the apt dist-upgrade takes.

## Prerequisites

The new host must be:

- **Ubuntu 22.04+ (or 24.04)** — other distros aren't tested.
- **Reachable from the operator workstation over SSH** — TCP port
  22 (or whatever your sshd is configured to use) must be open.
- **Configured with NOPASSWD sudo for the connecting user** —
  required because `convocate-host install` runs many `sudo`
  commands and won't pause for prompts.
- **Equipped with a working `claude` user already, OR** the
  connecting user must be able to create one. The first
  `convocate-host install` call sets this up if absent.

For your operator side:

- `convocate-host` binary present and on `PATH` (you got it from
  step 1 of [Getting started](../getting-started.md)).
- An SSH keypair that the new host accepts (typically `~/.ssh/
  id_ed25519`).
- The shell host already initialized via `convocate-host init-shell`
  (the agent will need the shell host's address and rsyslog CA).

## Step 1 — Provision the box (idempotent)

```bash
sudo convocate-host install --host <new-agent>
```

What this does on the target:

- `apt-get update && apt-get dist-upgrade -y`
- Installs base packages: docker, dnsmasq, jq, curl, git, ufw,
  ca-certificates, openssh-server
- Creates the `claude` user (UID 1337, group 1337) with `/home/claude`
- Sets timezone to `Etc/UTC`
- Configures ufw to allow `tcp/22` and the agent's `tcp/222`
- **Reboots the host** if a kernel was upgraded
- Polls every 30s up to 10 minutes for it to come back

This step is **idempotent** — safe to run multiple times. Re-running
is also the way you upgrade the OS later.

## Step 2 — Initialize the agent

Once the box is back up:

```bash
sudo convocate-host init-agent \
    --host <new-agent> \
    --shell-host <shell-host>
```

`--shell-host` tells the new agent which host runs the shell-side
status listener (the `tcp/223` socket). It's stamped into
`/etc/convocate-agent/shell-host` on the agent.

What `init-agent` does:

1. **Deploy the binary.** Copies your local `convocate-agent` to
   `/usr/local/bin/convocate-agent` on the target.
2. **Run `convocate-agent install` on the target.** This:
   - Creates `/etc/convocate-agent/`
   - Generates a stable `agent-id` (8-char random string) at
     `/etc/convocate-agent/agent-id`
   - Generates an ed25519 SSH host key at
     `/etc/convocate-agent/ssh_host_ed25519_key`
   - Writes the systemd unit `/etc/systemd/system/convocate-agent.service`
   - Writes `/etc/systemd/system/convocate-sessions.slice` with
     `CPUQuota = nproc*90%` and `MemoryMax = MemTotal*90%`
   - Drops `/etc/cron.daily/convocate-image-prune`
   - Drops `/etc/logrotate.d/convocate-agent-logs`
3. **Mint two ed25519 peering keypairs:**
   - `shell→agent` private + pub: lives on shell side under
     `/etc/convocate/agent-keys/<agent-id>/`. Pub also installed in
     the agent's authorized_keys so the shell can SSH in.
   - `agent→shell` private + pub: private installed at
     `/etc/convocate-agent/agent_to_shell_ed25519_key` on the
     agent. Pub installed in
     `/etc/convocate/status_authorized_keys` on the shell so the
     agent can authenticate when pushing status events.
4. **Issue rsyslog TLS client cert.** Generates an ECDSA P-256 key
   pair on the agent, has the shell-side CA sign it, installs
   `client.crt` and `client.key` under
   `/etc/convocate-agent/rsyslog-tls/` plus the CA at
   `ca.crt`.
5. **Drop the rsyslog client config** at
   `/etc/rsyslog.d/10-convocate-client.conf` to forward
   container logs to the shell host on `tcp/514` over TLS.
6. **Transfer the container image.** Runs `docker save | gzip` on
   the shell side, scp's the tarball to a temp path on the agent,
   verifies SHA-256 on both ends, runs `gunzip | docker load`,
   then `rm` on the temp file.
7. **Stamp the active version** at
   `/etc/convocate-agent/current-image`.
8. **Restart `convocate-agent.service`** so it picks up the new
   binary, the new host key, and the new shell-host config.
9. **Restart the shell's `convocate-status.service`** so it picks
   up the new authorized key and starts accepting connections from
   this agent.

If any step fails, the install is aborted and the partial state is
left in place for inspection — you can rerun the whole command, it's
idempotent.

## Step 3 — Verify

On the **shell host:**

```bash
ls /etc/convocate/agent-keys/
# should now include a directory named after the new agent's id
```

In the TUI: press `(N)ew`. The agent picker should now offer the new
agent. Pick it, create a smoke-test session, attach with Enter, run
`uname -a` to confirm you're inside the container on the new host,
detach with `Ctrl-B D`, kill with `(K)`, delete with `(D)`.

On the **new agent:**

```bash
systemctl status convocate-agent
# active (running)

cat /etc/convocate-agent/current-image
# convocate:v0.0.x

docker images convocate
# should match the tag in current-image

journalctl -u convocate-agent -n 50 --no-pager
# should see "claude-agent: connection from <shell-host-ip>"
# and "status connected to <shell-host-ip>:223"

tail -n 20 /var/log/convocate-agent/<agent-id>.log
# (on the SHELL host, not the agent — this is where forwarded logs land)
```

If the agent shows up in the TUI but the shell-side log file is empty,
TLS forwarding is misbehaving — see
[Troubleshooting](../troubleshooting.md).

## Removing an agent

There's no explicit "deregister" command. To retire an agent:

1. **Delete or migrate every session on it** (the TUI shows them per
   agent; `(D)elete` each, or save what you need first).
2. **Stop the agent service** on the agent host:
   `sudo systemctl stop convocate-agent.service`
   `sudo systemctl disable convocate-agent.service`
3. **Remove the agent's keys directory** on the shell host:
   `sudo rm -rf /etc/convocate/agent-keys/<agent-id>`
4. **Remove the corresponding entry from
   `/etc/convocate/status_authorized_keys`** so the agent can no
   longer push status events.
5. **Restart the shell's `convocate-status.service`.**

After step 5 the agent is gone from the TUI on the next 15-second
refresh. The agent host is otherwise untouched — Docker images,
session directories under `/home/claude/`, etc., remain. Reuse the
machine or `apt purge` whatever you don't need.
