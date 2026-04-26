# Capacity and isolation

convocate enforces resource limits at two layers. Both layers cap at
**90% of the host's CPU and memory** — a deliberate buffer so the host
itself, plus the agent process, plus any non-convocate workloads,
have at least 10% headroom.

## Layer 1 — admission control (in-process, at create time)

When the operator presses `(N)ew`, the agent's
`SessionOrchestrator.Create` runs an admission check before
`docker run`:

1. Sum the CPU + memory + disk of every existing container in
   `convocate-sessions.slice` (uses `docker inspect`).
2. Compute the delta the new session would add (from the operator's
   port/protocol/DNS spec; CPU/RAM are baseline-per-session).
3. If `existing + delta > 0.9 × host capacity`, refuse with a
   quantitative error message:

   ```
   admission denied: would push convocate-sessions.slice to 92% of host
     CPU (8.6 of 9.0 cores allowed); current usage 7.4 cores across 6
     sessions
   ```

This catches greedy creates *before* they happen, gives a clear error
to the operator, and prevents the kernel-level cap from having to
intervene.

## Layer 2 — kernel-enforced cgroup cap

On every agent, `convocate-host init-agent` writes
`/etc/systemd/system/convocate-sessions.slice`:

```ini
[Unit]
Description=Slice for all convocate session containers (90% host cap)

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
CPUQuota=<nproc * 90>%
MemoryMax=<MemTotal * 0.9>
```

Every session container is created with `--cgroup-parent
convocate-sessions.slice`, so the kernel enforces the aggregate cap
regardless of what Layer 1 did. If a session manages to spawn beyond
its planned quota (e.g. a process forks aggressively), the kernel
caps it.

This is **belt-and-braces by design**:

- Layer 1 gives nice errors but only catches what comes through the
  RPC path. A session created directly via `docker run` (out-of-band)
  bypasses Layer 1 entirely.
- Layer 2 is the kernel — it enforces no matter what, even on
  sessions we didn't admit.

## Why 90%, not 100%?

The 10% reserve goes to:

- The host operating system, journald, sshd, etc.
- The `convocate-agent` process itself
- The Docker daemon
- Any non-convocate workload on the same machine

Without this reserve, a hot session can push the host into OOM
territory where the kernel starts reaping random processes, and
"random" in OOM-killer language often means "the agent" — at which
point your control plane is dead and you can't even tell the rogue
session to stop.

## Why per-host, not global?

The cap is **per-agent**, not cluster-wide. There's no global
capacity tracker. Reasons:

- Each agent host has its own physical resources; aggregate cap
  doesn't translate cleanly across machines.
- Distributed capacity tracking introduces a coordinator that becomes
  a single point of failure.
- Operators can already see global utilization in the TUI's session
  list and route new sessions to a less-loaded agent themselves.

## Other isolation primitives

### Per-session container

Each session has its own `convocate-session-<uuid>` container. Sessions
can't see each other's processes, files, or network namespaces.

### Per-session home dir

Each session has its own `/home/claude/<uuid>/` directory on the
agent. That directory is bind-mounted into the container as
`/home/claude/`. State (Claude conversation history, project files,
git checkouts) is per-session, persists across detach/reattach, and
is destroyed on `(D)elete`.

### Read-only shares

The host's `claude` user's `~/.claude/`, `~/.ssh/`, and `~/.gitconfig`
are bind-mounted **read-only** into every container. Sessions get the
same Claude account, the same SSH identity, the same git identity —
but can't modify the source-of-truth files.

### No host network

Sessions use Docker's default bridge network. They can reach the
internet (for `apt install`, `git clone`, etc.) and the host's
loopback (for the dnsmasq integration to resolve their own DNS
names), but they can't see other containers' interfaces or other
hosts' private networks.
