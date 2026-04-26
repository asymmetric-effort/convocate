# claude-station provisioning TO-DO

Living checklist covering everything required to rebuild `claude-station`
from a base Ubuntu 22.04 install to the current state. Each item records
*what* was done, the key commands/files, and status.

- `[x]` = done and verified on current host
- `[ ]` = still outstanding / aspirational

---

## 1. Base OS

- [x] Ubuntu 22.04.5 LTS (Jammy), kernel 5.15 series
- [x] Hostname: `claude-station` (`/etc/hostname`, `/etc/hosts`)
- [x] Timezone: `Etc/UTC`
- [x] `unattended-upgrades` installed
- [x] `cloud-init` present (netplan yaml is cloud-init-generated)

## 2. User accounts

### samcaldwell (uid 1000)
- [x] Primary groups: `samcaldwell`, plus `adm`, `sudo`, `lxd`, `cdrom`, `dip`, `plugdev`
- [x] Shell: `/bin/bash`, home `/home/samcaldwell`
- [x] Sudoers drop-in: `/etc/sudoers.d/samcaldwell` (mode 0440)
- [x] SSH: `~/.ssh/authorized_keys`, `id_ed25519{,.pub}`, `known_hosts`

### claude (uid 1337)
- [x] Groups: `claude`, `docker`
- [x] Shell: `/usr/local/bin/convocate` (session launcher)
- [x] Sudoers drop-in: `/etc/sudoers.d/claude` (mode 0440)
- [x] SSH: `~/.ssh/authorized_keys`, `id_ed25519{,.pub}`, `known_hosts`

Reproducibility commands:
```bash
sudo useradd -u 1000 -m -s /bin/bash -G sudo,adm,lxd,cdrom,dip,plugdev samcaldwell
sudo useradd -u 1337 -m -s /usr/local/bin/convocate -G docker claude
# Populate ~/.ssh/authorized_keys out of band (password-store / secrets vault)
```

## 3. SSH

Current `sshd -T` highlights:
- [x] `Port 22`
- [x] `PubkeyAuthentication yes`
- [x] `PasswordAuthentication no`
- [x] `PermitRootLogin no`
- [x] `AllowUsers samcaldwell claude`
- [x] `X11Forwarding yes`

All hardening overrides live in `/etc/ssh/sshd_config.d/10-harden.conf`.
The drop-in sorts before `50-cloud-init.conf` so its values win
(first-match-wins in sshd).

## 4. Developer tooling

- [x] Go 1.26.2 at `/usr/local/go` (NOT apt; installed from tarball)
- [x] `PATH` includes `/usr/local/go/bin` for interactive shells
- [x] apt packages: `git`, `make`, `tmux`, `build-essential`, `ca-certificates`,
  `gnupg`, `curl`, `nftables`, `ufw`
- [x] `samcaldwell` dotfiles / shell config in place (gitconfig, ssh known_hosts)

## 5. Docker

- [x] `docker.io 29.1.3-0ubuntu3~22.04.1` (Ubuntu's package, not Docker CE)
- [x] `containerd 2.2.1-0ubuntu1~22.04.1`
- [x] `claude` user in the `docker` group
- [x] Storage driver: `overlayfs`, cgroup driver `systemd`, cgroup v2, runtime `runc`
- [x] Daily image prune: `/etc/cron.daily/docker-image-prune` runs
  `docker image prune -af` to reclaim disk from unused images
- [x] LAN-to-container-port lockdown: `docker-user-lockdown.service` +
  `/usr/local/sbin/docker-user-lockdown.sh`. Installs two rules into
  `DOCKER-USER` (order matters):
  1. `RELATED,ESTABLISHED -j RETURN` — allow response traffic for
     container outbound (prevents DROP from killing DNS/HTTP responses).
  2. `-i enp0s31f6 -j DROP` — block LAN-originated new connections to
     docker-published ports.
- [x] Default container DNS set to host dnsmasq via `docker0` gateway:
  `/etc/docker/daemon.json` → `{ "dns": ["172.17.0.1"] }`. New containers
  pick this up automatically; existing containers' `/etc/resolv.conf`
  was patched in-place (regenerated on next container restart).
- [x] Live Restore: **disabled**. A `systemctl restart docker` stops all
  running containers — schedule daemon restarts intentionally.

## 6. convocate application

Repo: `/home/samcaldwell/git/convocate`

- [x] Repository cloned, Go module builds (`go build ./...`)
- [x] `Dockerfile` builds the `convocate:latest` session image
- [x] `/usr/local/bin/convocate` installed as `claude`'s login shell
- [x] Sessions launched via TUI menu; containers run under `docker`
- [x] Shared config mount from `~/.claude` into session containers

## 7. DNS services (dnsmasq)

Host runs an authoritative + caching forwarder on three IPs (UDP + TCP):
- `127.0.0.1:53` — host-local
- `192.168.3.90:53` — LAN / WARP private-network clients
- `172.17.0.1:53` — docker containers via docker0 gateway

- [x] `dnsmasq` package installed (previously only `dnsmasq-base`)
- [x] Non-authoritative queries forwarded to LAN default gateway
- [x] `cache-size=1000`, `domain-needed`, `bogus-priv`
- [x] `systemd-resolved` stub on `127.0.0.53` disabled
  (`/etc/systemd/resolved.conf.d/00-convocate-dnsmasq.conf`)
- [x] `/etc/resolv.conf` is a static file (nameserver 127.0.0.1)
  (backup at `/etc/resolv.conf.bak-claude-dns`)
- [x] dnsmasq starts after docker so `docker0` exists before bind
  (`/etc/systemd/system/dnsmasq.service.d/10-after-docker.conf`)

Files:
- `/etc/dnsmasq.d/claude-station.conf` — authoritative + upstream + listen-address
- `/etc/dnsmasq.d/ubuntu-fan` — stock drop-in (`bind-interfaces`)
- `/etc/systemd/resolved.conf.d/00-convocate-dnsmasq.conf` — stub disabled, DNS=127.0.0.1
- `/etc/systemd/system/dnsmasq.service.d/10-after-docker.conf` — After=docker.service

## 8. ZTNA (Cloudflare Tunnel)

ZTNA on this host is Cloudflare Tunnel (`cloudflared`) and nothing else.

- [x] `cloudflared` at `/usr/local/bin/cloudflared`
- [x] `cloudflared.service` active (started 2026-04-22 13:32 UTC)
- [x] Runs as `/usr/bin/cloudflared --protocol http2 --config /etc/cloudflared/config.yml tunnel run`
- [x] Tunnel UUID: `9fd8a134-5a92-4e5e-8a9d-f23f60ba4d43`
- [x] Cloudflare Account tag: `6d319c04a0e892633af40185415d1f91`
  (human-readable tunnel *name* lives only in the Cloudflare Zero Trust
  dashboard; not stored on disk here)
- [x] Config: `/etc/cloudflared/config.yml` (also `config.yaml` — identical copy)
- [x] Credentials (contains `TunnelSecret`): `/root/.cloudflared/9fd8a134-5a92-4e5e-8a9d-f23f60ba4d43.json` (root-only)
- [x] Origin cert: `/root/.cloudflared/cert.pem` (root-only)
- [x] Log directory: `/var/log/cloudflared`
- [x] Ingress rules (from `config.yml`):
  - `ssh-claude.asymmetric-effort.com` → `ssh://127.0.0.1:22`
  - catch-all → `http_status:404`

## 9. Network exposure / firewall

Sockets bound on the host (`ss -lnptu`):
- `22/tcp` sshd (`0.0.0.0`)
- `53/udp+tcp` dnsmasq (`127.0.0.1` and `192.168.3.90`)
- `80/tcp`, `8001/tcp`, `8002/tcp`, `8003/tcp` docker-proxy
  (convocate session containers)

- [x] `ufw` enabled: `default deny incoming`, `default allow outgoing`
- [x] `ufw` allow rules (`ufw status numbered`):
  1. `192.168.3.90 22` from `0.0.0.0`
  2. `192.168.3.90 80` from `0.0.0.0`
  3. `192.168.3.90 443` from `0.0.0.0`
  4. `22/tcp` from Anywhere, v4 + v6 (`# LAN ssh`)
  5. `53/udp` and `53/tcp` on `docker0` from `172.17.0.0/16` to
     `172.17.0.1` (`# container DNS -> dnsmasq`)
- [x] `DOCKER-USER` chain drops inbound on `enp0s31f6` for FORWARD
  traffic (see §5). Docker-proxy ports go through FORWARD, not INPUT,
  so the ufw `:80`/`:443` allow rules do **not** expose docker-published
  ports to the LAN — those rules only apply to a host-level daemon
  bound directly to `:80`/`:443`.
- [x] Cloudflared reaches local services via loopback, which both ufw
  and `DOCKER-USER` permit.

Effective LAN reachability:
- `:22` sshd — reachable (ufw allow + goes through INPUT)
- `:53` dnsmasq — **not** reachable from LAN (no ufw allow on
  `enp0s31f6`); reachable from docker containers on `docker0`.
- Docker-published ports (`80`, `8001`, `8002`, `8003`) — **not**
  reachable (DOCKER-USER drops LAN-originated new connections)
- Container outbound (DNS, HTTP, git, etc.) — works; DOCKER-USER
  allows RELATED,ESTABLISHED so responses reach containers.

Cloudflared (tunnel + WARP private-network routing) bypasses all of
the above because it originates connections from loopback.

---

## Notes / decisions log

- **Authoritative zones wildcard everything below them.** e.g.
  `anything.dev.samcaldwell.net` -> `192.168.3.90`. This is deliberate
  for dev-env split-horizon DNS.
- **ZTNA software**: Cloudflare Tunnel (`cloudflared`). No Tailscale,
  WireGuard, or ZeroTier installed.
