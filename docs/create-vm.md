# `convocate-host create-vm`

Provisions a vanilla Ubuntu 22.04 host into a KVM hypervisor and
bootstraps a fully-unattended Ubuntu VM under it. Introduced in
v2.1.0.

## Synopsis

```
convocate-host create-vm \
    --hypervisor <fqdn|ip> \
    --username <ssh-user> \
    --domain <dns-suffix> \
    --cpu <vcpus> \
    --ram <MB> \
    --osdisk <GB> \
    --datadisk <GB>
```

| Flag | Meaning | Required |
|------|---------|----------|
| `--hypervisor` | FQDN or IP of the target machine | yes |
| `--username` | SSH user on the hypervisor (typical Ubuntu cloud images use `ubuntu`) | yes |
| `--domain` | DNS suffix appended to the random hostname (e.g. `samcaldwell.net`) | yes |
| `--cpu` | vCPU count for the new VM | yes (≥ 1) |
| `--ram` | RAM for the new VM, in MB | yes (≥ 512) |
| `--osdisk` | OS disk (`/`) size in GB | yes (≥ 5) |
| `--datadisk` | Secondary disk mounted at `/var` in GB | yes (≥ 1) |

## Prerequisites

`create-vm` runs from a configured **convocate host**. The local
machine must have:

- `/usr/local/bin/convocate` installed (`make install` from the
  project root provides this)
- `/var/lib/convocate/dnsmasq-hosts` writable by the operator
  (created by `convocate install` during the dnsmasq integration
  step)
- An SSH keypair under `~/.ssh/` (id_ed25519 / id_ecdsa / id_rsa) —
  `create-vm` installs the public half on the hypervisor for
  passwordless re-entry

The hypervisor itself can be a vanilla Ubuntu 22.04+ install with:

- A reachable sshd on tcp/22
- The `--username` account present, with a password (initial
  connection) and either NOPASSWD sudo or sudoers permission to run
  `apt`, `hostnamectl`, `systemctl`, and `virsh`
- Hardware virtualization extensions (or nested-virt enabled if the
  hypervisor itself is a VM)

## What the command does

The flow is intentionally idempotent — re-running against the same
hypervisor refreshes hardening, dnsmasq, and KVM config without
duplicating state.

### Phase 1 — SSH bootstrap

1. Dial `ssh <user>@<hypervisor>:22`. Tries the operator's SSH agent
   + `~/.ssh/id_*` keys first; falls back to an interactive
   password prompt only if no key works.
2. Append the operator's public key to the hypervisor's
   `~/.ssh/authorized_keys` (deduped via `grep -F -x` so re-running
   doesn't pile up entries).
3. Drop a CIS-aligned hardening file at
   `/etc/ssh/sshd_config.d/10-convocate-hardening.conf`. Directives
   drawn from CIS Ubuntu Linux 22.04 LTS Benchmark §5.2:

   ```
   Protocol 2
   PermitRootLogin no
   PasswordAuthentication no
   PermitEmptyPasswords no
   PubkeyAuthentication yes
   ChallengeResponseAuthentication no
   KbdInteractiveAuthentication no
   X11Forwarding no
   AllowTcpForwarding no
   PermitUserEnvironment no
   ClientAliveInterval 300
   ClientAliveCountMax 3
   LoginGraceTime 60
   MaxAuthTries 4
   MaxSessions 4
   MaxStartups 10:30:60
   LogLevel VERBOSE
   HostbasedAuthentication no
   IgnoreRhosts yes
   GSSAPIAuthentication no
   KerberosAuthentication no
   ```

   `sshd -t` validates the new config before `systemctl reload ssh`
   rolls it. The drop-in is named `10-` so it overrides Ubuntu's
   `50-cloud-init.conf` shipped on cloud images.

### Phase 2 — Hostname + DNS

4. Generate a fresh 8-char `[A-Za-z]` hostname (52⁸ ≈ 5.3×10¹³
   namespace).
5. `hostnamectl set-hostname <name>` and rewrite `/etc/hosts` so
   `127.0.1.1` resolves to `<fqdn> <hostname>`.
6. Resolve the hypervisor's IPv4 (literal IP passes through;
   hostnames go through `net.LookupHost`).
7. Append `<ip> <fqdn>` to the **shell host's**
   `/var/lib/convocate/dnsmasq-hosts` so the cluster resolves
   the new host via the shell host's dnsmasq. Replaces an existing
   line for the same FQDN — re-runs aren't additive.

### Phase 3 — Patch + reboot

8. `apt-get update -y && apt-get -o Dpkg::Options::=--force-confdef
   -o Dpkg::Options::=--force-confold -y dist-upgrade` and
   `shutdown -r +0`.
9. Reconnect with retries every 30 seconds for up to 10 minutes
   (cap = 20 attempts). The first attempt waits 30s before dialing
   so sshd has time to come back.

### Phase 4 — Local ISO cache

10. Detect operator's CPU arch (`amd64` or `arm64`).
11. Compute the Ubuntu 22.04.5 live-server ISO URL:
    - amd64: `https://releases.ubuntu.com/22.04/`
    - arm64: `https://cdimage.ubuntu.com/releases/22.04/release/`
12. Pull the matching `SHA256SUMS` and find the line for our ISO.
13. If `~/.convocate/iso/<file>` already exists with a matching
    sha256, skip. Otherwise stream the ISO down (write to
    `<file>.partial`, rename atomically), verify hash, return path.

### Phase 5 — Hypervisor dnsmasq forwarder

14. Detect the hypervisor's default gateway (`ip -4 route show
    default`).
15. Disable systemd-resolved's `:53` stub listener (drops
    `/etc/systemd/resolved.conf.d/00-convocate-dnsmasq.conf`).
16. Install dnsmasq.
17. Drop `/etc/dnsmasq.d/10-convocate.conf`:
    ```
    server=<shell-host-ip>     # forward queries here first
    server=<default-gateway>   # fallback
    cache-size=1000
    domain-needed
    bogus-priv
    ```
18. `systemctl restart dnsmasq`.

### Phase 6 — KVM stack

19. `apt install -y qemu-kvm libvirt-daemon-system libvirt-clients
    virtinst bridge-utils cloud-image-utils genisoimage`.
20. `systemctl enable --now libvirtd`.
21. `usermod -aG libvirt <user>` and `usermod -aG kvm <user>`.
22. `virsh net-autostart default && virsh net-start default`.
23. Verify `/dev/kvm` exists and is readable.

### Phase 7 — Resource probe + caps

24. Read host resources:
    - `nproc` → CPU cores
    - `/proc/meminfo` → MemTotal (KB → MB)
    - `df -B1 /var/lib/libvirt/images` (or `/var` fallback) → DiskGB
25. **Layer 2 cap.** Drop
    `/etc/systemd/system/machine.slice.d/99-convocate-cap.conf`:
    ```
    [Slice]
    CPUAccounting=yes
    CPUQuota=<nproc * 90>%
    MemoryAccounting=yes
    MemoryMax=<MemTotal * 0.9>
    ```
    The kernel enforces this aggregate ceiling on all
    libvirt-managed VMs, regardless of how they were created.
26. **Layer 1 admission check.** Sum existing VM allocations:
    - `virsh list --all --name` → every defined domain
    - `virsh dominfo <name>` per domain → CPU + Max memory
    - `virsh vol-list --pool default --details` → sum of volume
      capacities
    Refuse if `existing + requested > 90% of host` on any axis.

### Phase 8 — VM bootstrap

27. SCP the Ubuntu ISO to `/var/lib/libvirt/images/iso/<file>`.
28. Generate cloud-init NoCloud `user-data` + `meta-data`. The
    user-data subiquity autoinstall config covers:
    - hostname + username + authorized SSH key
    - `openssh-server`, `sudo`, `cloud-init`, `python3`
    - direct disk layout on `/dev/vda` for `/`
    - late-command: if `/dev/vdb` exists, `mkfs.ext4 -F /dev/vdb` +
      fstab line mounting it at `/var`
    - `/etc/sudoers.d/90-convocate-vm` granting NOPASSWD for the new
      user
29. Build a NoCloud seed ISO via `cloud-localds` (preferred) or
    `genisoimage` fallback.
30. `virt-install --noautoconsole`:
    ```
    virt-install \
      --connect qemu:///system \
      --name <hostname> \
      --vcpus <cpu> \
      --memory <ram> \
      --osinfo ubuntu22.04 \
      --disk path=/var/lib/libvirt/images/<host>-os.qcow2,size=<osdisk>,format=qcow2,bus=virtio \
      --disk path=/var/lib/libvirt/images/<host>-data.qcow2,size=<datadisk>,format=qcow2,bus=virtio \
      --cdrom <ubuntu.iso> \
      --disk path=<seed.iso>,device=cdrom,readonly=on \
      --network network=default,model=virtio \
      --graphics none \
      --console pty,target_type=serial \
      --extra-args 'console=ttyS0,115200n8 autoinstall' \
      --noautoconsole
    ```
    `virt-install` returns immediately; subiquity boots inside the
    VM, finds the NoCloud datasource, performs the unattended
    install, reboots into the installed system. The new VM is
    SSH-reachable as `<username>@<hostname>.<domain>` once the
    install finishes (~5–10 minutes depending on network speed).

## Resource ceiling — both layers

Two independent enforcement points keep a greedy VM (or a buggy
orchestrator) from making the hypervisor unusable:

**Layer 1 — admission control**, applied at create-vm time.
`existing_pledge + requested > 0.9 * host_total` refuses the
create. Quantitative error message names the axis, the request,
the existing pledge, and the cap, so the operator sees exactly
what to free.

**Layer 2 — `machine.slice` cgroup cap**, kernel-enforced. Every
libvirt VM runs under `machine.slice`. Our drop-in caps the slice
at `nproc*90%` CPU and `MemTotal*0.9` bytes. A noisy VM hits the
ceiling, gets throttled, and the operator's SSH session keeps
working.

If someone creates a VM out-of-band (virt-manager, raw `virsh
define`), Layer 1 is bypassed but Layer 2 still applies — the
kernel keeps everyone honest.

## Files written

On the operator's machine (the shell host):
- `/var/lib/convocate/dnsmasq-hosts` — A record appended/replaced
- `~/.convocate/iso/ubuntu-22.04.5-live-server-<arch>.iso` —
  cached, sha256-verified

On the hypervisor:
- `/etc/ssh/sshd_config.d/10-convocate-hardening.conf`
- `/etc/systemd/resolved.conf.d/00-convocate-dnsmasq.conf`
- `/etc/dnsmasq.d/10-convocate.conf`
- `/etc/systemd/system/machine.slice.d/99-convocate-cap.conf`
- `/etc/hosts` (127.0.1.1 line replaced)
- `/var/lib/libvirt/images/iso/ubuntu-22.04.5-live-server-<arch>.iso`
- `/var/lib/libvirt/images/iso/convocate-seed-<hostname>.iso`
- `/var/lib/libvirt/images/<hostname>-os.qcow2`
- `/var/lib/libvirt/images/<hostname>-data.qcow2`

Plus the SSH peering keys in the connecting user's
`~/.ssh/authorized_keys`.

## Troubleshooting

**"this host is not a configured convocate host"** — run `make
install` (or `convocate install`) on the local machine first.
The check requires both `/usr/local/bin/convocate` and
`/var/lib/convocate/`.

**Password prompt repeats** — keys aren't reaching the hypervisor.
On Ubuntu cloud images the default `ubuntu` user has key auth set
up via cloud-init's user-data; if you've disabled that, the first
create-vm needs the account password to bootstrap.

**"no default route on hypervisor"** — the network on the new box
is misconfigured. Bring an interface up and re-run.

**"admission failed — requested N + already pledged M would
exceed 90% cap"** — Layer 1 refusing. Free a VM first or pick a
larger hypervisor.

**"/dev/kvm not available"** — the hypervisor lacks virtualization
extensions. If the hypervisor itself is a VM, enable nested virt on
its parent. Otherwise the operator needs different hardware.

**Install hangs at "waiting for hypervisor to come back up"** —
the box rebooted but sshd is slow to start. The retry loop allows
10 minutes; if it consistently times out, check your network +
firewall (do you have a captive portal? VPN that drops connections
mid-reboot?) and re-run.

## CIS reference

The SSH hardening drop-in is aligned with CIS Ubuntu Linux 22.04
LTS Benchmark v1.0.0, sections:

- 5.2.4 Ensure permissions on SSH private host key files
- 5.2.7 Ensure SSH access is limited
- 5.2.8 Ensure SSH LogLevel is appropriate
- 5.2.10 Ensure SSH PermitRootLogin is disabled
- 5.2.11 Ensure SSH PermitEmptyPasswords is disabled
- 5.2.12 Ensure SSH PermitUserEnvironment is disabled
- 5.2.13 Ensure only strong Ciphers are used
- 5.2.16 Ensure SSH MaxAuthTries is set to 4 or less
- 5.2.17 Ensure SSH MaxStartups is configured
- 5.2.18 Ensure SSH MaxSessions is set to 10 or less
- 5.2.19 Ensure SSH LoginGraceTime is set to 60s or less
- 5.2.20 Ensure SSH warning banner is configured
- 5.2.21 Ensure SSH PAM is enabled
- 5.2.22 Ensure SSH AllowTcpForwarding is disabled
- 5.2.23 Ensure SSH X11Forwarding is disabled

Sections related to ciphers, MACs, and KEX are intentionally left
to the system defaults — Ubuntu 22.04's `/etc/ssh/sshd_config`
ships with a CIS-conforming algorithm list out of the box.

## Tests + coverage

`internal/hypervisor/` has comprehensive unit tests covering:

- option validation matrix
- SSH runner key-then-password fallback
- CIS hardening drop-in shape
- random hostname generation + alphabet bound
- dnsmasq A-record idempotent rewrite (insert / replace / append)
- arch detection + ISO URL construction (amd64 + arm64 paths)
- SHA256SUMS parser (with-* and without-* line shapes, malformed
  lines, missing file)
- end-to-end ISO Fetch via httptest fake server (download + cache
  reuse + redownload on hash mismatch)
- cloud-init user-data + meta-data generation
- seed ISO build script shape (cloud-localds + genisoimage fallback)
- KVM apt install + group membership commands
- machine.slice drop-in shape with computed CPUQuota / MemoryMax
- libvirt resource queries (`virsh list`, `dominfo`, `vol-list`)
- admission control matrix per axis, with quantitative error
  messages
- orchestrator happy path + every per-phase failure
  (dial / hostname / hardening / SetHostname / DNS resolve / DNS
   register / apt / reconnect / dnsmasq config / KVM install /
   /dev/kvm verify / resource detect / slice cap / pledge query /
   ISO mkdir / SCP ISO / autoinstall seed user-data + meta-data
   upload / virt-install)

Package coverage at v2.1.0 release: **94.1%**. The remaining
gaps are crypto/rand error paths (effectively unreachable on a
working OS), `http.NewRequest` failure paths (only fires for an
invalid HTTP method or malformed URL), and the
unsupported-architecture branch in `detectArch` (requires running
the test on an arch other than amd64 or arm64). Pushing past
that threshold would require build-tagged stubs for the standard
library which adds maintenance cost without proportional value.
