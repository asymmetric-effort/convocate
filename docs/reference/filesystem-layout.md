# Filesystem layout

A complete map of every path convocate touches.

## Shell host

### Configuration (root-owned, mode 0750 unless noted)

```
/etc/convocate/
├── status_host_ed25519_key       # SSH host key for tcp/223 listener (mode 0600)
├── status_host_ed25519_key.pub   # Public half
├── status_authorized_keys        # Pubkeys allowed to push status events (one per agent)
├── rsyslog-ca/
│   ├── ca.crt                    # Per-cluster ECDSA P-256 CA
│   ├── ca.key                    # CA private key (mode 0600)
│   ├── server.crt                # Shell-side rsyslog TLS cert
│   └── server.key                # Server key (mode 0600)
└── agent-keys/
    └── <agent-id>/               # One subdirectory per registered agent
        ├── agent-host             # Plain text: "host[:port]"
        ├── shell_to_agent_ed25519_key       # Shell's private key for tcp/222 (mode 0600)
        ├── shell_to_agent_ed25519_key.pub   # Public half
        ├── agent_to_shell_ed25519_key.pub   # Agent's public key for tcp/223 verification
        └── host_key.pub                     # Pinned agent host key
```

### Runtime / state

```
/var/lib/convocate/
└── dnsmasq-hosts                 # Auto-generated; rewritten on session DNS change

/var/log/convocate-agent/
└── <agent-id>.log                # rsyslog-routed; per-agent container log file
```

### dnsmasq integration

```
/etc/dnsmasq.d/
└── 10-convocate.conf             # addn-hosts=/var/lib/convocate/dnsmasq-hosts
```

### rsyslog integration

```
/etc/rsyslog.d/
└── 10-convocate-server.conf      # TLS listener on :514, route to /var/log/convocate-agent/

/etc/logrotate.d/
└── convocate-agent-logs          # Daily rotate of the per-agent log files
```

### Systemd

```
/etc/systemd/system/
└── convocate-status.service      # The shell's tcp/223 listener service
```

### Binary

```
/usr/local/bin/
├── convocate                     # TUI binary
├── convocate-host                # Provisioner / updater
└── convocate-agent               # Required when shell + agent on same box
```

## Agent host

### Configuration (root-owned, mode 0750 unless noted)

```
/etc/convocate-agent/
├── agent-id                      # Stable 8-char random ID, generated once
├── ssh_host_ed25519_key          # SSH host key for tcp/222 listener (mode 0600)
├── ssh_host_ed25519_key.pub      # Public half (pinned on the shell side)
├── authorized_keys               # Pubkeys allowed to dial in (typically one per shell)
├── shell-host                    # Plain text: where to push status events
├── current-image                 # Plain text: convocate:vX.Y.Z (the active tag)
├── agent_to_shell_ed25519_key    # Agent's private key for tcp/223 push (mode 0600)
├── agent_to_shell_ed25519_key.pub
└── rsyslog-tls/
    ├── ca.crt                    # Copy of the cluster CA (for trust chain)
    ├── client.crt                # Per-agent TLS client cert
    └── client.key                # Client key (mode 0600)
```

### Sessions (writable by `claude` user)

```
/home/claude/
├── <uuid>/                       # One per session
│   ├── session.json              # Session metadata
│   ├── .claude/                  # Per-session Claude CLI state
│   ├── .skel/                    # Skel files (CLAUDE.md etc.) on first start
│   └── ... (project files, conversation history, anything created during use)
└── <uuid>.lock                   # PID-owned lock file when an op is in flight
```

### rsyslog client

```
/etc/rsyslog.d/
└── 10-convocate-client.conf      # Forward container logs to shell-host:514 over TLS
```

### Systemd

```
/etc/systemd/system/
├── convocate-agent.service       # The agent main loop
└── convocate-sessions.slice      # Cgroup parent for all session containers
                                  # (CPUQuota = nproc*90%, MemoryMax = MemTotal*90%)
```

### Cron / scheduled

```
/etc/cron.daily/
└── convocate-image-prune         # Drop convocate:* tags no container references

/etc/logrotate.d/
└── convocate-agent-logs          # Local rotation if rsyslog forwarding fails
```

### Binary

```
/usr/local/bin/
└── convocate-agent
```

### Optional Anthropic integration (read-only bind-mounted into containers)

```
/usr/local/bin/claude              # Anthropic Claude CLI; mounted at the same path
                                   # in every session container (read-only)

/home/claude/.claude/              # The agent's claude-user Claude config; bind-
                                   # mounted into every session as ~/.claude-shared/
                                   # (read-only). Sessions write their own ~/.claude/
                                   # which starts empty and persists per session.

/home/claude/.ssh/                 # Mounted read-only into every session
/home/claude/.gitconfig            # Same
```

## Inside a session container

```
/home/claude/                     # Bind-mounted from <agent>:/home/claude/<uuid>/
├── .claude/                      # Per-session Claude state (writable)
├── .claude-shared/               # Bind-mounted from <agent>:/home/claude/.claude/ (RO)
├── .ssh/                         # Bind-mounted from <agent>:/home/claude/.ssh/ (RO)
├── .gitconfig                    # Bind-mounted (RO)
├── CLAUDE.md                     # From the skel; tells Claude Code project conventions
└── (working files, project checkouts, etc.)

/usr/local/bin/claude             # Bind-mounted from agent (RO)

/var/run/docker.sock              # Bind-mounted from agent for Docker-in-Docker
                                  # (this is a known trust-boundary; see security)
```

## Permission summary

| Path | Owner | Mode | Why |
|---|---|---|---|
| `/etc/convocate*/` | root:root | 0750 | Config; not readable by `claude` user |
| `*.key` files | root:root | 0600 | Private keys |
| `*.pub` / `*.crt` | root:root | 0644 | Public material |
| `agent-id`, `current-image`, `shell-host` | root:root | 0644 | Plain-text config |
| `/var/lib/convocate/dnsmasq-hosts` | root:root | 0644 | Read by dnsmasq |
| `/var/log/convocate-agent/*.log` | root or syslog | 0640 | rsyslog-routed |
| `/home/claude/<uuid>/` | claude:claude | 0700 | Per-session, claude-only |
| `/home/claude/<uuid>.lock` | claude:claude | 0644 | Lock file |
