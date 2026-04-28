# `convocate`

The operator's TUI binary. Runs as the `convocate` user; `convocate-host
install` sets it as `claude`'s login shell so SSH'ing in lands you
directly in the menu.

When invoked with no arguments, it boots the TUI. With a subcommand,
it runs the named one-shot helper instead.

## Synopsis

```
convocate                          # boot the TUI
convocate install                  # one-time per-host setup
convocate status-serve             # internal: drive the tcp/223 listener
convocate version                  # print build version
convocate help                     # print usage
```

## TUI mode

```
convocate
```

Boots the menu. See [Using the TUI](../../guides/using-the-tui.md)
for the keybindings and screens.

Talks to all registered agents at startup. If no agents are
registered, you'll see the "No agents" prompt (and need to run
`convocate-host init-agent` first).

## `convocate install`

```
convocate install
```

Bootstraps the local host as a convocate shell host. Idempotent.
Typically invoked indirectly via `make install` (the Makefile runs
this for you), but you can run it directly if you've placed the
binary by hand.

**What it does:**

- Verifies Docker is present and the daemon is reachable.
- Creates the `convocate` user (UID 1337, group 1337) if missing.
- Builds the `convocate:<version>` container image from the
  embedded Dockerfile + entrypoint + skel.
- Sets `/usr/local/bin/convocate` as `claude`'s login shell.
- Provisions `/var/lib/convocate/dnsmasq-hosts` (empty file).
- Drops `/etc/dnsmasq.d/10-convocate.conf` if dnsmasq is
  installed.

**Requires:** `sudo` (touches `/etc/passwd`, `/etc/dnsmasq.d`,
`/var/lib/`).

**Exit codes:**

- `0` — success.
- non-zero — explanation printed to stderr; nothing partial left
  behind for fields that took the failed step.

## `convocate status-serve`

```
convocate status-serve [--listen :223] [--host-key PATH] [--auth-keys PATH]
```

Internal command. Drives the SSH listener on `tcp/223` for the
`convocate-status` subsystem. **You don't normally invoke this
directly** — `convocate-host init-shell` writes a systemd unit at
`/etc/systemd/system/convocate-status.service` that calls it.

Flag defaults:

| Flag | Default | Purpose |
|---|---|---|
| `--listen` | `:223` | TCP address to bind |
| `--host-key` | `/etc/convocate/status_host_ed25519_key` | Server host key path |
| `--auth-keys` | `/etc/convocate/status_authorized_keys` | One pubkey per line; one entry per registered agent |

Logs go to systemd-journal. Reload after editing `--auth-keys`:

```bash
sudo systemctl restart convocate-status
```

## `convocate version`

Prints `convocate version v0.0.x`. The version is stamped at compile
time via `-X main.Version=$(git describe --tags --dirty)`.

## `convocate help`

Prints a short usage summary. `--help` and `-h` are aliases.

## Environment variables

The TUI reads no environment variables for configuration. All state
comes from:

- `/home/convocate/<uuid>/session.json` — per-session metadata (on agents)
- `/etc/convocate/agent-keys/<id>/` — registered agents (on shell)
- `/var/lib/convocate/dnsmasq-hosts` — DNS registrations

Mutating any of these requires sudo and a service restart;
convocate doesn't reload them at runtime.
