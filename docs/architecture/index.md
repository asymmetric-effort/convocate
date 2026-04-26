# Architecture

This section walks through how convocate is built. Start with the
[three-binary model](three-binaries.md) for the high-level shape; drop
into the others for specifics.

| Page | What it covers |
|---|---|
| [Three binaries](three-binaries.md) | What `convocate`, `convocate-host`, `convocate-agent` each do and where they run |
| [Control plane](control-plane.md) | The SSH subsystems, the ports, the auth model, the framing |
| [Session lifecycle](session-lifecycle.md) | States a session moves through, status indicators in the TUI, what each transition means |
| [Image distribution](image-distribution.md) | How `convocate:<v>` images get from the shell host to every agent |
| [Capacity and isolation](capacity-and-isolation.md) | The two-layer 90% cap (Layer 1 admission + Layer 2 cgroup), per-session container isolation |
| [Security posture](security-posture.md) | Trust boundaries, key types, what's authenticated, what's not |

## Why convocate exists

Anthropic's Claude CLI is great at single-user single-session work.
The moment you want to:

- Run several Claude sessions in parallel without their state stomping on each other,
- Detach from a long-running task and re-attach from a different machine later,
- Push the actual workload off your laptop to a beefy workstation or rented box,
- Cap the resource cost so a runaway session can't crash the whole machine,

…you want a session orchestrator. convocate is that orchestrator. It
runs each Claude CLI session inside its own Docker container on an
"agent" host, and gives you a TUI that lists, attaches, and detaches
sessions across one or many agents.

## Why "convocate"?

*Convocate* is an archaic English verb meaning "to call together; to
convoke." From Latin *com-* "together" + *vocare* "to call." The
TUI **calls together** ephemeral AI sessions running across many
agent hosts and gathers them into one operator pane.

The name is also clean across software namespaces — no GitHub, PyPI,
npm, Docker Hub, or trademark collisions. convocate is a deliberate
rename from the original `claude-shell` to avoid sitting on top of
Anthropic's CLAUDE® trademark.
