# Changelog

Release history under the `convocate` name. The project was
previously published as `claude-shell`; that history is preserved
in [the v2.0.0 architectural snapshot](v2.0.0.md) and the
git history but is not included here as a release line — the
v3.0.0 rename effectively reset versioning.

## v0.0.1 — 2026-04-26

**First release under the new project name.**

Contents:

- Project renamed from `claude-shell` to `convocate` to avoid
  Anthropic's CLAUDE® trademark and to give the project a clean
  namespace across PyPI / npm / Docker Hub / crates.io / domain
  TLDs.
- Repository transferred from `sam-caldwell/claude-shell` to
  `asymmetric-effort/convocate`.
- All binary names, filesystem paths, systemd units, container
  prefixes, cgroup slice, and identifiers renamed:
  - `convocate` (TUI), `convocate-host` (deploy), `convocate-agent` (worker)
  - `convocate-session-<uuid>` containers under `convocate-sessions.slice`
  - `/etc/convocate/`, `/etc/convocate-agent/`, `/var/lib/convocate/`,
    `/var/log/convocate-agent/`
- Anthropic-tool integration points preserved: the `convocate` user,
  Anthropic's `/usr/local/bin/claude`, `~/.claude/`,
  `~/.claude-shared/`, `CLAUDE.md` filename convention,
  `CONVOCATE_UID` / `CONVOCATE_GID` entrypoint env.
- Project-wide rule added: **no recursion in Go** (Go has no TCO,
  so every recursive call grows the goroutine stack — unacceptable
  for an orchestrator that holds long-lived state). `session.copyDir`
  rewritten as iterative; CLAUDE.md and project memory codify the
  rule for future contributions.
- Documentation site at `convocate.asymmetric-effort.com` —
  this site, built with [@asymmetric-effort/specifyjs][specifyjs]
  and deployed via GitHub Actions on every push to main.

[specifyjs]: https://www.npmjs.com/package/@asymmetric-effort/specifyjs
- Tag history reset; previous tags (v0.0.1 through v3.0.0 under the
  `claude-shell` name) deleted.

## Pre-rename history

The full pre-rename release history (under `claude-shell`) is
recoverable from the git log. Tags were deleted as part of the
rename so they don't show up in `git tag -l`. Notable arcs:

- **v1.0.0 (2026-04-24)** — multi-host orchestration arc. Three-
  binary model introduced: shell + host + agent, SSH peering, TLS
  log forwarding.
- **v2.0.0 (2026-04-24)** — "shell is pure client" arc. Sessions
  moved from running locally on the shell to running on agents.
  Image distribution pipeline. Cgroup cap. Orphan migration tool.
  See [v2.0.0 architectural snapshot](v2.0.0.md).
- **v2.1.0 (2026-04-25)** — `create-vm` subcommand added; KVM
  hypervisor provisioning + cloud-init autoinstall.
