# DNS and networking

convocate uses three TCP ports and one DNS integration. This page is
the reference for what runs where and what goes through the network.

## Ports

| Port | Listener | Purpose |
|---|---|---|
| `tcp/22` | host sshd | Operator-side SSH. Not touched by convocate; humans use this to log in. |
| `tcp/53/udp` | dnsmasq (shell host, optional) | Cluster DNS authority for per-session DNS names. |
| `tcp/222` | `convocate-agent.service` (each agent) | Shell → Agent control plane. Subsystems: `convocate-agent-rpc`, `convocate-agent-attach`. |
| `tcp/223` | `convocate-status.service` (shell host) | Agent → Shell status push. Subsystem: `convocate-status`. |
| `tcp/514` | rsyslog (shell host) | Agent → Shell container log forwarding (TLS). |
| Container ports | Per-session | Each session can publish a port via the TUI's port field at session create / edit. |

## ufw rules

`convocate-host install` configures ufw to:

- **Allow `tcp/22`** (host SSH).
- **Allow `tcp/222`** on agent hosts (so the shell can reach them).
- **Allow `tcp/223`** on the shell host (so agents can reach it).
- **Allow `tcp/514`** on the shell host (rsyslog TLS listener).
- **Allow `udp/53` and `tcp/53`** on the shell host *only if* dnsmasq
  is running there.

Outbound is unrestricted. Inbound default-deny on everything else.

## dnsmasq integration (optional)

When the shell host runs dnsmasq, convocate registers session DNS
names in it automatically. Each session can have a DNS name
(operator-provided in the create/edit dialog), and the resulting
record is `<session-dns-name>.<domain>` → `<agent-host-IP>`.

### How it works

1. The shell host has dnsmasq listening on `udp/53` + `tcp/53`.
2. `/var/lib/convocate/dnsmasq-hosts` is a hosts-file-style mapping
   that the shell rewrites every time a session's DNS metadata
   changes (create / edit / delete).
3. dnsmasq's config drop-in
   `/etc/dnsmasq.d/10-convocate.conf` adds `addn-hosts=/var/lib/
   convocate/dnsmasq-hosts` so dnsmasq reads from that file.
4. Operators (and any host that uses the shell as its DNS server)
   resolve `<session-dns-name>.<domain>` to the IP of the agent
   running that session.

### Setting it up

`convocate-host install` on the shell host installs dnsmasq if it's
not already present. Configure your network's `domain` in
`/etc/dnsmasq.conf` if you haven't:

```
domain=internal.example.com
expand-hosts
```

Then point clients at the shell host as their DNS resolver.

### Privileged-port pitfall

If you run convocate on a host that uses systemd-resolved (the Ubuntu
default), `udp/53` is taken by `systemd-resolved`'s stub listener and
dnsmasq won't bind. Two options:

1. **Disable the stub listener.** Edit `/etc/systemd/resolved.conf`
   and set `DNSStubListener=no`, then `systemctl restart systemd-
   resolved`.
2. **Run dnsmasq on a non-privileged port.** Configure dnsmasq with
   `port=5353` and have your clients query that port explicitly.

`convocate-host install` does (1) automatically when it installs
dnsmasq.

### Using DNS without a publish port

A session's DNS name resolves to the **agent host's** IP, not the
container's. If the session has no published port, the DNS record
still resolves but you can't reach the session from outside the
agent. Useful when the session is for in-container work (Claude
inside the box, not exposing services).

If the session has a published port (`8080/tcp`, say), the resolved
IP + port let you reach it from anywhere on the network that can
talk to the agent host.

## Container networking

Each session container uses Docker's default `bridge` network. The
agent runs:

```
docker run \
    --rm --detach \
    --name convocate-session-<uuid> \
    --hostname convocate-<uuid8> \
    --cgroup-parent convocate-sessions.slice \
    [-p HOST:CONTAINER/PROTO if a port is configured] \
    [--dns <agent-host-IP> if dnsmasq is in play] \
    convocate:<version>
```

What this means:

- Each container has its own network namespace.
- Two containers on the same agent can reach each other's
  exposed ports via the docker bridge.
- The container's own DNS resolver is set to the agent host (so
  it can resolve other sessions' DNS names via dnsmasq).
- The container's `--hostname` is `convocate-<first 8 chars of UUID>`,
  not the user-provided DNS name (the DNS name is for *external*
  resolution, not internal).

## Multi-host networking

Cross-agent reachability:

- Containers on different agents are on different docker bridges,
  not directly reachable to each other by container hostname.
- They *are* reachable to each other via the agent host's IP +
  the container's published port (if any).
- DNS names resolve to the agent host, so a session DNS like
  `myapp.internal.example.com:8080` works from any host that can
  query the shell's dnsmasq.

There is no overlay network, VXLAN, or service mesh. Cross-agent
networking is operator-managed via published ports + DNS, by design.

## Firewall + security

The narrow port list isn't a coincidence — every additional listener
is an attack surface. The intentional choices:

- No HTTP. Operator interaction is SSH only.
- No metrics endpoint. Status events are pushed over the existing
  agent → shell SSH, not exposed externally.
- No Docker socket exposure beyond the agent host. The shell talks
  to the agent's RPC, not to the agent's Docker daemon.

If you need to expose convocate metrics to a monitoring system,
build a sidecar that subscribes to the status event stream on the
shell host and translates to your monitoring system's protocol —
don't open new ports.
