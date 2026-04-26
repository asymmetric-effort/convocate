# Session management

Day-to-day workflow for creating, modifying, and tearing down
sessions. See [Using the TUI](using-the-tui.md) for the keybindings;
this page is about *what each operation does* and when to use it.

## Creating a session

`(N)ew` from the main menu.

If multiple agents are registered, you first pick which agent will
host the session. Then a four-field form:

| Field | Required? | Notes |
|---|---|---|
| **Name** | Yes (1+ char) | Display label. Letters, digits, hyphens, underscores |
| **Protocol** | Defaults to `tcp` | Toggle with `t` / `u` / Space when the field is active |
| **Port** | Optional | `0` means no published port; `-1` means "auto-pick a free port ≥1001" |
| **DNS Name** | Optional | Lowercase hostname chars only; registers `<name>.<domain>` in the shell's dnsmasq |

When you submit:

1. UUID generated (`uuid.NewRandom`).
2. Session directory created at `/home/claude/<uuid>/` on the chosen agent.
3. `session.json` metadata written.
4. Container *not* started yet — it spins up lazily when you attach.

The session shows up in the list immediately as `-` (stopped). Press
Enter to attach.

## Attaching to a session

`Enter` on a highlighted session.

- If the session is `-` (stopped): the agent runs `docker run` with
  the session's persisted port/protocol/DNS, then `docker exec -it
  ... tmux attach-session -t claude` over the SSH attach subsystem.
  State becomes `C`.
- If the session is `R` (running detached): the agent skips the
  `docker run` and just attaches. State becomes `C`.
- If the session is `C` (already connected by another operator):
  attach succeeds — multiple operators can share one tmux session.
  Useful for pair work.

Detach with `Ctrl-B D` (tmux's chord), or close the SSH connection,
or press `(B)` from the menu before pressing Enter. Detach takes you
back to the TUI menu without stopping the container.

## Backgrounding a session

`(B)` on a highlighted session.

This is the same as `Ctrl-B D` from inside tmux — closes the attach
channel without killing the container — but invoked from the menu.
Use it when:

- You attached but didn't run anything and want out without typing
  `Ctrl-B D` (the tmux chord can be muscle-memory-fail).
- A previous operator forgot to detach and you want to free up the
  PTY without disrupting their tmux session.

## Cloning a session

`(C)` on a highlighted session.

What gets copied:

- The session's `/home/claude/<uuid>/` directory (project files,
  conversation history, anything written to it).
- The session's metadata except UUID, timestamps, and any port (the
  clone gets `Port: 0` to avoid collision).

What does **not** get copied:

- The running tmux session (each session has a fresh tmux when its
  container starts).
- Active container processes.

The clone lives on the **same agent** as the source. To move a
session to a different agent, you'd need to `(C)` clone + manually
move the directory between agents (no built-in cross-agent clone).

## Editing a session

`(E)` on a highlighted session opens the edit form. Same fields as
create:

- **Name** — update freely.
- **Port + Protocol** — must be free on the agent (per-protocol
  uniqueness, so `tcp:53` and `udp:53` can coexist).
- **DNS Name** — must be globally unique across the cluster (the
  shell's dnsmasq is the authority).

If the session is currently running (`R` or `C`), changes don't take
effect until `(R)`estart — the running container keeps its old
docker-run flags. This is intentional: you can stage a change without
disrupting an active session.

## Restarting a session

`(R)` on a highlighted session.

What it does:

1. If running: refuses with "session is already running."
2. Otherwise: starts the container in detached mode (`docker run
   --detach` — no PTY attached).
3. State goes `-` → `R`.

Useful when:

- A session crashed and needs to come back up.
- You edited port/protocol/DNS and need a clean rerun.
- You want a "warm" session in the `R` state ready for someone to
  Enter-attach to without the create+attach latency.

## Killing a session

`(K)` on a highlighted session.

`docker stop -t 10` — sends SIGTERM, waits 10 seconds, then SIGKILL
if the container hasn't exited. The session **directory is preserved
on disk** — you can restart, clone, or re-attach.

Use Kill when:

- A session is stuck in a runaway loop and you want it down without
  losing its files.
- You're done with a session for the day but want to re-open it
  tomorrow without losing state.

## Deleting a session

`(D)` on a highlighted session, then `Y` to confirm.

Permanent. What gets removed:

- The container (kill if running).
- `/home/claude/<uuid>/` on the agent (project files, conversation
  history, everything).
- `session.json` metadata.
- The lock file.
- The DNS entry (if any).

Use Delete when:

- The session is genuinely done and you want the disk back.
- You created with the wrong name/agent and want to redo.

## Override a stale lock

`(O)` on a highlighted session, only useful when the session is in
the `L` (locked-but-not-running) state.

Convocate's session manager auto-detects stale locks (PID dead, mtime
> 24h) and clears them. Override is for the rare case when those
heuristics don't trigger but you know the lock is bogus — for
example, a previous agent process crashed and the lock file's
ownership PID is still technically alive but is some other unrelated
process.

Override removes the lock file. It does not stop containers, kill
processes, or modify session state.

## Settings dialog

`(S)` from anywhere in the menu opens a read-only settings panel
showing version, build info, and registered agents. No mutations
from this dialog — it's diagnostic.

## Quit

`(Q)` exits the TUI. **Sessions keep running** — convocate is just a
client; the agents host the actual work. SSH back in and pick up
where you left off.
