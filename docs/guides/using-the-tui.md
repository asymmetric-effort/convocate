# Using the TUI

The `convocate` binary is a `tcell`-based terminal UI. After
`convocate-host install` sets `convocate` as the `convocate` user's
login shell, you SSH in and you're in it — no extra command needed.

```
ssh convocate@<shell-host>
```

## Layout

```
┌─ Title bar ──────────────────────────────────────────── load + clock ─┐
│ convocate                                       0.42, 0.31, 0.28  …    │
├──────────────────────────────────────────────────────────────────────┤
│ Status  UUID                                  Name           Port/Proto│
│   C     5a2c...   alpha-task                  refactor       8080/tcp │
│   R     74e3...   beta-build                  build-pipeline    -    │
│   -     901e...   gamma-explore               explore-deps      -    │
│   L     abf3...   stuck-thing                  stale-lock        -    │
├──────────────────────────────────────────────────────────────────────┤
│ (N)ew (C)lone (E)dit (B)ackground (D)elete (K)ill (O)verride (S)e... │
└──────────────────────────────────────────────────────────────────────┘
```

- **Title bar** — project name on the left, system load + clock on the right.
- **Session table** — one row per session across every agent. Sorted by `LastAccessed` (most recent first).
- **Menu bar** — single-key shortcuts.

## Status indicators

The leftmost column shows a single character per session:

| Char | Meaning |
|---|---|
| `-` | Stopped (no container, no lock) |
| `L` | Locked but not running (stale lock; use `(O)verride`) |
| `R` | Running detached (no operator currently attached) |
| `C` | Connected — at least one operator has a PTY attached now |
| `O` | Orphan (no agent claims this session — only present after upgrading from pre-v2) |

## Key bindings

### From the menu (no dialog open)

| Key | Action | When usable |
|---|---|---|
| `Up` / `Down` | Move cursor in the session list | Always |
| `Enter` | Attach to the highlighted session (creates the container if stopped) | Always (when a session is highlighted) |
| `N` | New session (opens the create dialog; first asks which agent if more than one) | Always |
| `C` | Clone the highlighted session (copies its home dir to a new UUID) | Highlighted session exists |
| `E` | Edit the highlighted session's name / port / protocol / DNS name | Highlighted session exists |
| `B` | Background the highlighted session (detach without stopping the container) | Highlighted session is running |
| `D` | Delete the highlighted session permanently (after confirmation) | Highlighted session exists |
| `K` | Kill the highlighted session's container (stops it; doesn't delete) | Highlighted session is running |
| `O` | Override a stale lock on the highlighted session | Highlighted session is in `L` state |
| `S` | Open the settings dialog | Always |
| `R` | Restart the highlighted session in detached mode | Highlighted session exists |
| `Q` | Quit the TUI (sessions keep running) | Always |

Keys are case-insensitive — `n` and `N` both open the create dialog.

### Inside a confirmation dialog (Delete, Kill, Override, Background, Restart)

| Key | Action |
|---|---|
| `Y` | Confirm |
| `N` / `Esc` | Cancel and return to the menu |

### Inside the create / edit form

| Key | Action |
|---|---|
| `Tab` / `Shift-Tab` | Cycle between Name → Protocol → Port → DNS Name |
| Letters / digits | Type into the active field |
| `Backspace` | Delete the last character of the active field |
| Space / `t` / `u` | Toggle the Protocol field (when active) |
| `Enter` | Submit |
| `Esc` | Cancel back to the menu |

### Once attached to a session (inside tmux)

You're in tmux running Claude. tmux's chord prefix is `Ctrl-B`:

| Sequence | Action |
|---|---|
| `Ctrl-B` then `D` | Detach without stopping the container (state goes `C` → `R`) |
| `Ctrl-B` then `[` | Enter scroll mode (vim keys; `q` to exit) |
| `Ctrl-B` then `?` | Show tmux's full keymap |
| `Ctrl-D` (at an empty Claude prompt) | Exit Claude; tmux session stays alive in the background |

## Auto-refresh

- **1-second tick** redraws the clock, load, and status indicators.
- **15-second tick** reloads the full session list across all agents.

If you want to see a session's state change immediately, press any key
to force a redraw.

## Multi-agent flow

If you have multiple agents registered:

1. Press `(N)ew`
2. The TUI shows a picker dialog: "Select agent for new session"
3. Up/Down to move; Enter to confirm
4. Once you pick an agent, the create form opens and the session is
   created on that agent

Sessions only ever live on one agent — there's no automatic
rebalancing or migration. To move a session to a different agent,
use `(D)elete` + `(N)ew`, or `convocate-host migrate-session` if it's
a pre-v2 orphan.

## Single-agent flow

If only one agent is registered, `(N)ew` skips the picker and goes
straight to the create form.

If zero agents are registered, `(N)ew` shows the "No agents
registered" dialog with a hint to run `convocate-host init-agent`.

## Common patterns

**"I want to do three things in parallel."** Press `(N)`, name them,
attach with `Enter`, do work, `Ctrl-B D` to detach, repeat. The TUI
shows them all in the list with `R` indicators when detached.

**"I closed my laptop on a long-running task."** That's fine — the
session container kept running on the agent. SSH back in, the TUI
shows it as `R`, press Enter to reattach.

**"I need to start over with the same files."** Highlight the
session, press `(C)`, give it a name. The new session has a copy of
the original's home dir but a fresh tmux session and fresh Claude
state.

**"This session got into a bad state and won't respond."** Press
`(K)` to stop the container, then `(R)` to restart it. The session
directory is preserved across restart so your conversation history
+ project files are intact.
