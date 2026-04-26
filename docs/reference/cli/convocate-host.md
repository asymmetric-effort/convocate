# `convocate-host`

The deploy-and-provision tool. One-shot CLI; not a service. Runs from
the operator's workstation or shell host.

Every subcommand uses the same SSH auth model:

- **Local invocation** (no `--host`): runs with `sudo` against the
  current machine.
- **Remote invocation** (with `--host`): SSH's to the target as
  `--user` (default `root` if unset) using the operator's `~/.ssh/`
  keys; the connecting user must have NOPASSWD sudo on the remote.

## Synopsis

```
convocate-host install         [--user U --host H]
convocate-host init-shell      --host H [--user U]
convocate-host init-agent      --host H [--user U] [--shell-host H]
                                [--ca-cert PATH --ca-key PATH]
convocate-host update          --host H [--user U]
convocate-host migrate-session --agent A --session UUID
convocate-host create-vm       --hypervisor H --username U --domain D
                                --cpu N --ram MB --osdisk GB --datadisk GB
convocate-host version
convocate-host help
```

## `install`

```
convocate-host install [--user U] [--host H]
```

Prepares a vanilla Ubuntu host for convocate. Idempotent.

**What it does on the target:**

- `apt-get update && apt-get dist-upgrade -y`
- Installs base packages: docker, dnsmasq, jq, curl, git, ufw, ca-
  certificates, openssh-server
- Creates the `claude` user (UID 1337) if missing
- Sets timezone to `Etc/UTC`
- Configures ufw to allow `tcp/22` plus role-specific ports
- Reboots the host if a kernel was upgraded (remote invocations
  only — local invocation refuses to reboot the host you're typing
  on)
- Polls every 30s up to 10 minutes for the host to come back

**Flags:**

| Flag | Default | Purpose |
|---|---|---|
| `--host H` | local | Target host. Omit for local install. |
| `--user U` | `$USER` (remote), n/a (local) | SSH user. Must have NOPASSWD sudo. |

**Run when:** any time you bring up a new physical or virtual host
that's going to play any role in the cluster (shell or agent).

## `init-shell`

```
convocate-host init-shell --host H [--user U]
```

Configures the shell-side: `convocate-status` listener, rsyslog CA +
server cert, ufw rules.

**What it does:**

- Deploys the `convocate` binary (already there from local
  `make install`) and runs `convocate install` on the target.
- Drops `/etc/systemd/system/convocate-status.service`.
- Generates an SSH host key for the status listener at
  `/etc/convocate/status_host_ed25519_key`.
- Mints an ECDSA P-256 CA at `/etc/convocate/rsyslog-ca/`:
  `ca.crt`, `ca.key`, `server.crt`, `server.key`.
- Drops `/etc/rsyslog.d/10-convocate-server.conf` with a TLS
  listener on `tcp/514` that routes incoming events into
  `/var/log/convocate-agent/<agent-id>.log`.
- Drops `/etc/logrotate.d/convocate-agent-logs`.
- Opens `ufw allow 223/tcp` + `ufw allow 514/tcp`.
- Enables and starts `convocate-status.service`.

**Run when:** once, the first time you stand up a shell host. Re-run
to rotate CA artifacts (warning: invalidates every agent's TLS
client cert; you'd then need to re-run `init-agent` for each).

## `init-agent`

```
convocate-host init-agent --host H [--user U] \
    [--shell-host H] \
    [--ca-cert PATH --ca-key PATH]
```

Configures one agent host.

**What it does:** see [Adding a new agent](../../guides/adding-an-agent.md)
for the step-by-step. Summary:

1. Deploys `convocate-agent`; runs `convocate-agent install`.
2. Mints two ed25519 peering keypairs.
3. Issues an rsyslog TLS client cert.
4. Transfers the current `convocate:<v>` image (with SHA-256
   verification).
5. Stamps `/etc/convocate-agent/current-image`.
6. Restarts `convocate-agent.service` and the shell's
   `convocate-status.service`.

**Flags:**

| Flag | Default | Purpose |
|---|---|---|
| `--host H` | required | Agent host to provision |
| `--user U` | `$USER` | SSH user on the agent (NOPASSWD sudo required) |
| `--shell-host H` | local hostname | Where the agent should push status events |
| `--ca-cert PATH` | `/etc/convocate/rsyslog-ca/ca.crt` | Used when running from a workstation that isn't the shell host — point at a local copy of the CA cert |
| `--ca-key PATH` | `/etc/convocate/rsyslog-ca/ca.key` | Likewise for the CA key |

**Run when:** once per new agent. Idempotent; re-run to repair drift.

## `update`

```
convocate-host update --host H [--user U]
```

Rolls a new binary + image to one agent. See
[Updating the cluster](../../guides/updating-the-cluster.md).

Re-deploys `convocate-agent`, transfers the latest local
`convocate:<v>` image, stamps `current-image`, restarts the agent
service.

**Run when:** after `make build && sudo make install` on the shell
host produces a new version you want pushed.

## `migrate-session`

```
convocate-host migrate-session --agent <agent-id> --session <uuid>
```

Moves a pre-v2 orphan session directory from the local shell host to
a target agent. See [Migrating orphans](../../guides/migrating-orphans.md).

**Flags:**

| Flag | Default | Purpose |
|---|---|---|
| `--agent A` | required | Destination agent ID |
| `--session UUID` | required | Which orphan to move |

**Run when:** during a v1.x → v2.x upgrade, once per orphan. Will
likely never be needed again on a clean v2+ install.

## `create-vm`

```
convocate-host create-vm \
    --hypervisor <fqdn|ip> \
    --username <user> \
    --domain <domain> \
    --cpu <count> --ram <MB> --osdisk <GB> --datadisk <GB>
```

Provisions a vanilla Ubuntu host as a KVM hypervisor and bootstraps
a new Ubuntu VM under it. Full walkthrough at
[Provisioning a hypervisor](../../guides/create-vm.md).

**Flags:**

| Flag | Required | Purpose |
|---|---|---|
| `--hypervisor` | yes | Target host that will become a KVM hypervisor |
| `--username` | yes | SSH user on the hypervisor (NOPASSWD sudo required) |
| `--domain` | yes | DNS suffix for the new VM (`<random-host>.<domain>`) |
| `--cpu` | yes | vCPU count for the new VM |
| `--ram` | yes | RAM size in MB |
| `--osdisk` | yes | OS disk size in GB (mounted at `/`) |
| `--datadisk` | yes | Data disk size in GB (mounted at `/var`) |

The total `--cpu` / `--ram` / `--osdisk + --datadisk` must fit in 90%
of the hypervisor's resources or the call refuses pre-flight.

## `version`

```
convocate-host version
```

Prints `convocate-host version v0.0.x`.

## `help`

```
convocate-host help
convocate-host --help
convocate-host -h
```

Prints the usage summary above.
