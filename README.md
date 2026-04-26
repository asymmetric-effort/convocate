# convocate

[![CI](https://github.com/asymmetric-effort/convocate/actions/workflows/ci.yml/badge.svg)](https://github.com/asymmetric-effort/convocate/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/asymmetric-effort/convocate/badges/.badges/coverage.json)](https://github.com/asymmetric-effort/convocate/actions/workflows/ci.yml)

A three-binary system for orchestrating isolated, containerized Claude CLI
sessions across one or many Linux hosts.

📖 **Full documentation: <https://convocate.asymmetric-effort.com>**

## Overview

As of v2.0.0 convocate ships three cooperating binaries:

- **`convocate`** — the interactive TUI. Runs as user `claude`. Lists
  sessions across all registered agents, creates sessions on a chosen
  agent, attaches to a session's container over SSH. Doesn't run
  containers itself.
- **`convocate-agent`** — the worker. One per host that actually hosts
  session containers. Listens on `tcp/222` over SSH. Runs `docker run`
  for sessions. Enrolls every container under `convocate-sessions.slice`
  for a 90% aggregate cgroup cap.
- **`convocate-host`** — the deploy tool. Provisions vanilla Ubuntu hosts,
  copies binaries over SSH, wires up SSH peering + TLS-encrypted log
  forwarding, distributes the container image.

Per-session isolation (separate home dir + dedicated container) and
shared-config-by-mount (`~/.claude`, SSH keys, `.gitconfig` read-only
from the agent host into each container) are unchanged from earlier
versions — they just run on agents now instead of on the shell host.

## Prerequisites

Per-host requirements by role:

| Role | Needs |
|------|-------|
| Operator workstation | `git`, `make`, Go 1.26+ (source build), an SSH key that reaches each target host |
| Shell host | Ubuntu 22.04+, docker (for the image build), dnsmasq (optional — the shell is the cluster DNS authority when present) |
| Agent host | Ubuntu 22.04+, docker, systemd (cgroup v2) |

For the simplest topology you can run shell + one agent on the same
Ubuntu box. `convocate-agent` binds `tcp/222` and the shell status
listener binds `tcp/223`, so they coexist cleanly.

## Getting Started

End-to-end walkthrough from a vanilla Ubuntu install to a working
deployment. Replace `<shell-host>` and `<agent-host>` with your
hostnames or IPs — they can be the same box.

### 1. Build from source

On whichever machine will run the TUI (usually your shell host):

```bash
git clone https://github.com/asymmetric-effort/convocate.git
cd convocate
make build
sudo make install                # copies all three binaries to /usr/local/bin
                                  # then runs `convocate install`
```

`make install` does the source-install + runs `convocate install`,
which:

- checks Docker is present
- creates the `claude` user (uid 1337) if missing
- builds the session container image tagged with the binary's version
  (`convocate:v2.0.x`)
- sets `/usr/local/bin/convocate` as `claude`'s login shell
- provisions `/var/lib/convocate/dnsmasq-hosts` so the shell can
  register per-session DNS names when `dnsmasq` is installed

### 2. Provision hosts (one-time, per new host)

`convocate-host install` turns a fresh Ubuntu 22.04 box into one ready
to host the rest of the stack. Operates locally with `sudo`, or
remotely via SSH (NOPASSWD sudo required on the remote):

```bash
# Local shell host:
sudo convocate-host install

# Remote agent host (runs from your workstation):
convocate-host install --host <agent-host>
```

This installs the base apt packages, docker, dnsmasq, creates the
`claude` user, enables ufw, and sets the timezone to UTC. It runs
`apt dist-upgrade` and — for remote invocations — reboots the target
before continuing with the remaining steps. The complete list of
things it configures is in the
[provisioning checklist](https://convocate.asymmetric-effort.com/getting-started/).

### 3. Init the shell side

On the shell host:

```bash
sudo convocate-host init-shell --host <shell-host>
```

This:

- deploys the `convocate` binary (already there from step 1) + runs
  its install subcommand remotely (idempotent)
- drops the `convocate-status` systemd unit that listens on
  `tcp/223` for agent → shell status pushes
- mints an ECDSA P-256 CA under `/etc/convocate/rsyslog-ca/` and
  signs a server cert
- drops `/etc/rsyslog.d/10-convocate-server.conf` with a TLS
  listener on `tcp/514` that routes per-agent logs into
  `/var/log/convocate-agent/<agent-id>.log`
- opens `ufw allow 223/tcp`
- enables + starts `convocate-status.service`

### 4. Init each agent

For every host that will run sessions:

```bash
sudo convocate-host init-agent \
    --host <agent-host> \
    --shell-host <shell-host>
```

This:

- deploys the `convocate-agent` binary + runs its install subcommand
  (creates systemd unit, writes `convocate-sessions.slice` with
  `CPUQuota = nproc*90%` and `MemoryMax = MemTotal*90%`, drops the
  daily image-prune cron)
- mints two ed25519 peering keypairs (`shell→agent`, `agent→shell`)
  and installs both halves on each end
- issues a TLS client cert for the agent's rsyslog forwarder, signed
  by the CA from step 3
- `docker save | gzip | ssh | docker load` transfers the current
  `convocate:v2.0.x` image to the agent, verifying a SHA-256 over
  the tarball on both ends
- writes `/etc/convocate-agent/current-image` so the agent knows which
  tag to `docker run`
- starts `convocate-agent.service`

If you're running init-agent from a workstation rather than the
shell host, pass `--ca-cert` / `--ca-key` pointing at local copies
of the rsyslog CA material.

### 5. Launch the TUI

SSH into the shell host as the `claude` user (the install step set
that user's login shell to `convocate`):

```bash
ssh claude@<shell-host>
```

You should see the session menu. Press `N` to create — you'll be
prompted to pick one of the registered agents. Enter to attach to
an existing session. `Ctrl+B D` (tmux's detach chord) disconnects
without killing the container.

### 6. Verify end-to-end

```bash
# On the shell host:
systemctl status convocate-status   # should be active
ls /etc/convocate/agent-keys/       # one subdir per registered agent

# On each agent:
systemctl status convocate-agent          # active
cat /etc/convocate-agent/current-image    # prints the convocate:v2.0.x tag
docker images convocate             # should match that tag
tail -f /var/log/convocate-agent/<id>.log # agent → shell log forwarding
```

## Day-2 Operations

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
older convocate install, any `/home/claude/<uuid>/` directories
from before v2 show up in the TUI with an `O` status and can't be
acted on directly. Move them onto an agent:

```bash
# Stop the old local container first if one is still running:
docker stop convocate-session-<uuid>

sudo convocate-host migrate-session \
    --agent <agent-id> \
    --session <uuid>
```

The session reappears under the target agent on next TUI refresh.

## Architecture

### Session isolation

Each session has:

- a unique UUIDv4
- its own home directory at `/home/claude/<uuid>/` on the **agent** host
- a dedicated Docker container named `convocate-session-<uuid>`,
  enrolled in `convocate-sessions.slice` for kernel-enforced 90%
  aggregate CPU + memory cap

### Shared resources (read-only bind mounts, on the agent)

- Claude CLI binary (`/usr/local/bin/claude`)
- SSH keys (`~/.ssh/`)
- Git configuration (`~/.gitconfig`)
- Claude settings (`~/.claude/` as `~/.claude-shared/`)

### Control plane

- **Shell → Agent (`tcp/222`)**: SSH subsystems
  `convocate-agent-rpc` (CRUD JSON RPC) and `convocate-agent-attach`
  (raw pty relay). Shell holds a persistent connection per agent.
- **Agent → Shell (`tcp/223`)**: SSH subsystem
  `convocate-status` — newline-JSON event stream
  (`agent.started`, `agent.heartbeat`, `container.created/started/
  stopped/deleted`, etc.). Agent holds a persistent connection.
- **Agent → Shell (`tcp/514`)**: rsyslog TLS-encrypted log forwarding,
  authenticated with per-agent client certs under the CA mint-ed by
  `init-shell`.

### Container image distribution

The shell host builds `convocate:<semver>` during
`convocate install`. `init-agent` and `update` ship that tarball
to each agent via `docker save | gzip | ssh | docker load`, verifying
SHA-256 on both ends. A daily cron on each agent deletes image tags
no container is still referencing.

See [`docs/reference/v2.0.0.md`](./docs/reference/v2.0.0.md) for the full
architectural snapshot and known limitations.

## Development

```bash
make build             # build all three binaries under ./build/
make test              # unit tests + integration
make lint              # go vet + yaml lint + json lint
make clean             # remove ./build/

# Release flow (tags + pushes):
make release           # patch bump (v2.0.x → v2.0.x+1)
make release/minor     # minor bump
make release/major     # major bump
```

Coverage targets for business-logic packages (menu, session, container,
capacity, dns, multihost) are 90%+. Installer and hostinstall SSH-runner
paths are explicitly excused because they require root + real network.

## License

MIT License — see [LICENSE.txt](LICENSE.txt)
