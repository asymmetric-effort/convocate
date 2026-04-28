# Getting started

End-to-end walkthrough from a vanilla Ubuntu install to a working
deployment. Replace `<shell-host>` and `<agent-host>` with your hostnames
or IPs — they can be the same box.

## Prerequisites

| Role | Needs |
|------|-------|
| Operator workstation | `git`, `make`, Go 1.26+ (source build), an SSH key that reaches each target host |
| Shell host | Ubuntu 22.04+, Docker (for the image build), dnsmasq (optional — the shell is the cluster DNS authority when present) |
| Agent host | Ubuntu 22.04+, Docker, systemd (cgroup v2) |

For the simplest topology, run shell + one agent on the same Ubuntu box.
`convocate-agent` binds `tcp/222` and the shell status listener binds
`tcp/223`, so they coexist cleanly.

## 1. Build from source

On whichever machine will run the TUI (usually your shell host):

```bash
git clone https://github.com/asymmetric-effort/convocate.git
cd convocate
make build
sudo make install                # copies all three binaries to /usr/local/bin
                                  # then runs `convocate install`
```

`make install` does the source-install + runs `convocate install`, which:

- checks Docker is present
- creates the `convocate` user (uid 1337) if missing
- builds the session container image tagged with the binary's version (`convocate:vX.Y.Z`)
- sets `/usr/local/bin/convocate` as `claude`'s login shell
- provisions `/var/lib/convocate/dnsmasq-hosts` so the shell can register per-session DNS names when `dnsmasq` is installed

## 2. Provision hosts (one-time, per new host)

`convocate-host install` turns a fresh Ubuntu 22.04 box into one ready
to host the rest of the stack. Operates locally with `sudo`, or remotely
via SSH (NOPASSWD sudo required on the remote):

```bash
# Local shell host:
sudo convocate-host install

# Remote agent host (runs from your workstation):
convocate-host install --host <agent-host>
```

This installs the base apt packages, Docker, dnsmasq, creates the
`convocate` user, enables ufw, and sets the timezone to UTC. It runs
`apt dist-upgrade` and — for remote invocations — reboots the target
before continuing with the remaining steps.

## 3. Init the shell side

On the shell host:

```bash
sudo convocate-host init-shell --host <shell-host>
```

This:

- deploys the `convocate` binary (already there from step 1) + runs its install subcommand remotely (idempotent)
- drops the `convocate-status` systemd unit that listens on `tcp/223` for agent → shell status pushes
- mints an ECDSA P-256 CA under `/etc/convocate/rsyslog-ca/` and signs a server cert
- drops `/etc/rsyslog.d/10-convocate-server.conf` with a TLS listener on `tcp/514` that routes per-agent logs into `/var/log/convocate-agent/<agent-id>.log`
- opens `ufw allow 223/tcp`
- enables + starts `convocate-status.service`

## 4. Init each agent

For every host that will run sessions:

```bash
sudo convocate-host init-agent \
    --host <agent-host> \
    --shell-host <shell-host>
```

This:

- deploys the `convocate-agent` binary + runs its install subcommand (creates systemd unit, writes `convocate-sessions.slice` with `CPUQuota = nproc*90%` and `MemoryMax = MemTotal*90%`, drops the daily image-prune cron)
- mints two ed25519 peering keypairs (`shell→agent`, `agent→shell`) and installs both halves on each end
- issues a TLS client cert for the agent's rsyslog forwarder, signed by the CA from step 3
- `docker save | gzip | ssh | docker load` transfers the current `convocate:<v>` image to the agent, verifying a SHA-256 over the tarball on both ends
- writes `/etc/convocate-agent/current-image` so the agent knows which tag to `docker run`
- starts `convocate-agent.service`

If you're running `init-agent` from a workstation rather than the
shell host, pass `--ca-cert` / `--ca-key` pointing at local copies
of the rsyslog CA material.

## 5. Launch the TUI

SSH into the shell host as the `convocate` user (the install step set
that user's login shell to `convocate`):

```bash
ssh convocate@<shell-host>
```

You should see the session menu. Press `N` to create — you'll be
prompted to pick one of the registered agents. Enter to attach to
an existing session. `Ctrl+B D` (tmux's detach chord) disconnects
without killing the container.

## 6. Verify end-to-end

```bash
# On the shell host:
systemctl status convocate-status   # should be active
ls /etc/convocate/agent-keys/       # one subdir per registered agent

# On each agent:
systemctl status convocate-agent          # active
cat /etc/convocate-agent/current-image    # prints the convocate:<v> tag
docker images convocate                   # should match that tag
tail -f /var/log/convocate-agent/<id>.log # agent → shell log forwarding
```

## Day-2 operations

**Roll out a new binary + image across the cluster.** Build + install
on the shell host, then push to every agent:

```bash
cd convocate && git pull
make build && sudo make install        # rebuilds + retags image
for agent in agent-a agent-b agent-c; do
    sudo convocate-host update --host "$agent"
done
```

Update pushes both the fresh binary and the new tagged image to
each agent and rewrites `/etc/convocate-agent/current-image`. Existing
containers keep running on their original image tag until restart —
cutover is session-by-session, gated on `(R)estart` in the TUI.

**Migrate a pre-v2 orphan session to an agent.** If you upgraded an
older install, any `/home/convocate/<uuid>/` directories from before v2
show up in the TUI with an `O` status and can't be acted on directly.
Move them onto an agent:

```bash
# Stop the old local container first if one is still running:
docker stop convocate-session-<uuid>

sudo convocate-host migrate-session \
    --agent <agent-id> \
    --session <uuid>
```

The session reappears under the target agent on next TUI refresh.
