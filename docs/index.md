# convocate

> **convocate** *(verb, archaic)* — to call together; to convoke. From Latin
> *com-* "together" + *vocare* "to call."

**convocate** is a Go-based system for running and managing containerized
[Claude CLI](https://docs.claude.com/en/docs/claude-code) sessions across
one or many Linux hosts. The operator runs a TUI on their laptop; sessions
live on remote agent hosts as ephemeral Docker containers; you attach,
detach, and re-attach as the work demands.

## What it solves

You're using Claude Code seriously. You want:

- **Per-task isolation** — each Claude session in its own container, with
  its own home directory, its own state, no cross-contamination between
  parallel work streams.
- **Background sessions that survive your laptop closing** — start a
  refactor, close the lid, drive home, re-attach in the evening from a
  different machine.
- **Multi-host fan-out** — the work runs on agent hosts (your beefy
  workstation, a rented bare-metal box, a private cloud VM), not your
  laptop's battery.
- **Capacity-aware admission** — the agent refuses new containers when
  CPU or memory would push the host past 90%, so a runaway prompt can't
  brick the whole machine.
- **A single-pane TUI** — list every session across every agent, attach
  with `Enter`, detach with `Ctrl-b d`, kill with `K`.

## The three binaries

| Binary | Where it runs | What it does |
|---|---|---|
| **`convocate`** | Operator's laptop | TUI session manager; talks to agents via SSH |
| **`convocate-host`** | Operator's laptop | One-time provisioning: install agents, generate keys, build & distribute the container image |
| **`convocate-agent`** | Each agent host (Linux + Docker) | SSH server on tcp/222; runs the actual session containers |

## Get started

[**→ Bootstrap from a base Ubuntu install**](getting-started.md)

[**→ Architecture and design**](architecture.md)

[**→ Provision a fresh KVM hypervisor with `convocate-host create-vm`**](guides/create-vm.md)

## Project links

- [Source on GitHub](https://github.com/asymmetric-effort/convocate)
- [Issue tracker](https://github.com/asymmetric-effort/convocate/issues)
- [Releases](https://github.com/asymmetric-effort/convocate/releases)
- [Contributing](contributing.md) · [Security](security.md) · [Code of conduct](code-of-conduct.md)
