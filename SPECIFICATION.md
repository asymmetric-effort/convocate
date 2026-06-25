# Convocate — Build Specification

> Status: draft v0.1 · Source of truth for implementation
> Companion artifacts: `api/openapi.yaml` (API contract), `Unity Desktop.dc.html` (interactive reference prototype)

---

## 1. Idea

Convocate is an agentic software-engineering platform. A user describes an
application end-state in natural language; Convocate decomposes that
specification into an execution graph, runs AI agents on managed compute to
implement it, and manages the resulting code, reviews and deployments — all
from a single desktop-style web UI.

The user-facing product is a **Unity/GNOME-style desktop** in the browser: a top
bar, an application dock, and draggable/resizable windows. Each dock icon opens
an **applet**. The desktop, login, and window manager are application-agnostic
chrome; the product value lives in the applets.

The **production UI must be built in SpecifyJS** (the declarative TypeScript UI
framework). The existing `Unity Desktop.dc.html` is an interactive
visual + interaction specification, **not** the shippable artifact.

---

## 2. Core concepts (domain model)

| Concept   | Summary |
| --------- | ------------------------------------------------------------------------------------------------- |
| **Node**  | A compute host (arbitrary SSH-reachable host) running the Convocate runtime. Lifecycle: `online → draining → offline`. Carries load average (1/5/15), memory, disk, tags (`cpu:amd64`, `os:linux`, …), and write-once notes. |
| **Agent-container** | A container on a Node running the Claude CLI in permission-bypass mode behind a golang wrapper. Has a logical `project` name, owner, status (`running`/`connected`/`stopped`/`migrating`), and an exposed `FQDN:port`. |
| **Project** | A unit of work; owns a git **Repository** and a `SPECIFICATION.md`. |
| **Project Board** | A free canvas of **Containers** and **Cards** wired by **Edges** into an execution DAG. Persisted as `ProjectBoard.json` in the repo, linked to the spec. |
| **Card** | A task. Status `todo → active → done`/`fail`, plus `note` (inert). Holds title, content, source-file refs, an implementation note, and links. |
| **Container** | Groups cards; mapped to an agent-container that executes its cards. |
| **Edge** | A typed link between cards: `DependsOn` or `RelatesTo`. |
| **Repository / PullRequest** | Git repo (GitHub-backed) with feature branches and PRs; PRs carry CI checks and a merge gate. |
| **User / Group / Role** | RBAC principals. `admin` role grants all features/roles. |
| **Ticket** | Support request. |

---

## 3. Architecture

### 3.1 Two planes
- **Control plane** — the Convocate API + SpecifyJS UI. Stateless-ish; owns all
  persistent state in a database.
- **Data plane** — the **Nodes**: arbitrary hosts the control plane provisions
  over SSH and on which it orchestrates agent-containers. Nodes are *not* part of
  the backend host; the control plane reaches them via outbound SSH.

### 3.2 Layers
1. **Shell** — desktop chrome: top bar (Activities, clock, user menu), dock,
   window manager (open/focus/drag/resize/min/max), Activities overview,
   lock/login. Hosts applets; per-applet context menu bar.
2. **Applets** — Node Manager, Agent Manager, Project Board, Code IDE,
   Access Control, Repo Manager, Support Tool. Each is a self-contained SpecifyJS
   module mounted into a window.
3. **Cross-cutting services** — auth/session, RBAC enforcement, typed API
   client, window/router store, real-time event channel.

### 3.3 API conventions
- All endpoints: `/api/v1/<applet_shortname>/...`
  (`nmgr`, `amgr`, `pb`, `ide`, `repo`, `ac`, `sup`, plus `auth`).
- Bearer JWT auth; RBAC per operation (`x-required-role`); `admin` implies all.
- Resource-oriented REST; cursor/offset `Page<T>` lists.
- Long-running actions (provision, drain, board implement, render) return
  `202 Accepted`; progress is delivered over the real-time channel.
- Streaming (agent shell) is a WebSocket upgrade.
- **Full contract: `api/openapi.yaml`.**

---

## 4. Authentication & authorization

### 4.1 AuthN
- **Local**: username + password + MFA (TOTP, 6-digit). `POST /auth/login`.
- **GitHub OIDC**: authorization-code flow (`/auth/oidc/github/start` →
  `/auth/oidc/github/callback`), mapping the federated identity to a Convocate
  account.
- JWT access token + refresh token; sessions revocable on logout; idle lock per
  `sessionTimeoutMinutes`.

### 4.2 AuthZ (RBAC)
- Roles are applet-scoped verbs, e.g. `node-create`, `node-view`, `node-update`,
  `node-delete`, `agent-view`, `agent-update`, `pb-view`, `pb-update`,
  `pb-execute`, `ide-view`, `ide-update`, `repo-view`, `repo-update`,
  `repo-merge`, `access-view`, `access-update`, `support-view`.
- The special role **`admin` grants every feature and role.**
- Users belong to Groups; Groups map to Roles; effective roles are the union.
- UI enforcement: the dock shows only applets the principal may view;
  unauthorized opens show "Permission denied"; action buttons (Stop/Start/Delete/
  Provision/Merge/…) are gated by the relevant role. Server is the authority;
  the UI gate is a convenience.

---

## 5. Applet specifications

### 5.1 Node Manager (`nmgr`)
- Paginated node list: id, location (default `unspecified`), IP, status dot,
  agent count, **load average 1/5/15**, memory, controls.
- **Location** is inline-editable (double-click the cell).
- **Provision Node** dialog: host (valid IPv4/IPv6/FQDN), SSH user (valid Linux
  username), optional password (first connection only), location, **tags**
  (comma-separated; seeded with `cpu:<arch>` and `os:<os>`). Backend generates
  its own SSH keypair, installs the public key to `authorized_keys`, then hardens
  SSH to disable password logins. Client- and server-side validation.
- **Start** (offline→online) and **Stop** (drain): on stop the scheduler stops
  dispatching new agents, running tasks drain, then agent-containers **migrate to
  other nodes with capacity**; any that cannot be placed are **stopped in place**.
  A system note records the outcome; node goes `offline`.
- **Detail** dialog: full stats incl. load average + tags, agent-container list,
  and **write-once / read-many notes** with an "Add Note" composer that includes
  a **Capture Stats** button (appends a structured stats snapshot to the note).
- **Delete**: confirm → drain, terminate services, uninstall, power off.

### 5.2 Agent Manager (`amgr`)
- Accordion grouped by node; each agent row: id, project, status, ip:port, owner,
  controls (Start/Stop/Config/Shell).
- **Shell**: opens a terminal window streaming the agent's **Claude CLI in
  permission-bypass mode** (`claude --dangerously-skip-permissions`).
- **Create Agent**: project, node, image, startup command.
- **Configure Agent**: project, node, and **Expose `FQDN:port`** — host must be a
  DNS name, **not** an IP address (validated). No startup command in configure.

### 5.3 Project Board (`pb`)
- Dotted canvas of **Containers** (mapped to agent-containers) and **Cards**.
- Cards nest inside containers; a container's body is **scrollable and pannable**
  (click-drag). Moving a container moves its nested cards. Drag a free card onto a
  container to **attach** (green drop highlight).
- Cards and containers are **draggable and resizable** (corner handle).
  Double-click a card title or container header to **minimize**.
- **Card editing**: `todo` cards — title and content editable (double-click),
  body editable. `active`/`done` cards are **read-only**.
- **Card colors**: yellow `todo`, blue `active`, green `done`, red `fail`,
  grey `note`.
- **Edges** carry visible relationship labels ("depends on"/"relates to"),
  drawn as the **shortest border-to-border segment**; an edge to a nested card
  terminates at the card when visible, else at the container border. Click to
  select (thicker + bold); right-click to **change type** or **delete**.
- **Edge rules**: TODO↔TODO → `DependsOn` or `RelatesTo`; ACTIVE/DONE↔ACTIVE/DONE
  → `RelatesTo` only; a TODO may `DependsOn` an ACTIVE/DONE card (oriented so the
  TODO depends on the executed card); other mixed → `RelatesTo`.
- **Link to…**: right-click a card → set link source → click target (default
  `RelatesTo`).
- Right-click canvas → **Create Card / Create Container**; Graph menu →
  New Card/Container, Rename Project, Save as Repository, Open from Repository.
- Window title shows `Project Board (<projectName>)`.
- **Implement** (Graph/Run menu) or right-click card → **Send to Agent**:
  dispatches container-attached, non-note cards to their agent-containers as one
  execution unit. Cards go `active` then `done` (PR opened) or `fail` (open
  question), annotated with source-file refs and an implementation note.
- **Open in editor**: opens the card as a `<cardId>.json` document in the Code
  IDE, structured as `{ id, title, status, content, container, position, size,
  sourceRefs, note, links[] }`.

### 5.4 Code IDE (`ide`)
- File explorer, editor tabs, status bar.
- **Project menu**: New Project (prompts a name → creates a Code Repository and
  opens a `SPECIFICATION.md` template), Save, Save As, **Render Project Board**
  (LLM decomposes the spec into a board, stored as `ProjectBoard.json`),
  **Implement**.
- **Card JSON editing**: a card opened from the board renders as syntax-colored
  JSON; it is **editable with validate-before-save** (status enum, required
  fields, well-formed links). Saving updates the board card; editing the card on
  the board reflects live in the open JSON.

### 5.5 Access Control (`ac`)
- Tabs: **Users** (create/enable/disable/delete, group), **Groups**
  (create/delete, map users, map roles; builtin groups protected),
  **Global Settings** (require MFA, session timeout, password policy).

### 5.6 Repo Manager (`repo`)
- Repository list → open → **Files** and **Pull Requests** tabs.
- Files: browse; open in IDE, or a `ProjectBoard.json` in Project Board.
- PRs: created automatically when board cards complete (feature branch → main).
  PR detail shows **GitHub Actions checks** and changed files; **Merge to main**
  is **gated on passing checks** (merge/deployment gates).
- RepoMan manages CI/CD execution through connected GitHub Actions runners.

### 5.7 Support Tool (`sup`)
- **Tickets** (list + New Ticket composer: subject, priority, body) and
  **Documentation** browser.

---

## 6. Key end-to-end workflow (the Convocate loop)

1. **Code IDE → New Project** → names it → creates repo + `SPECIFICATION.md`.
2. User edits the spec; **Project | Render Project Board** sends it to the LLM,
   which produces `ProjectBoard.json` (Cards + Containers) linked to the spec.
3. User refines the board, maps containers to agent-containers, wires the DAG.
4. **Implement** (or per-card **Send to Agent**) dispatches cards to agents.
   Cards turn green (done, PR opened) or red (failed/needs review); cards gain
   source-file references and implementation notes.
5. Generated changes appear as **feature branches** in RepoMan; each completed
   run opens a **Pull Request**.
6. Review code in the IDE (via card file refs or RepoMan); when satisfied,
   **merge** the PR (gated on CI). RepoMan drives the CI/CD pipeline and gates.

---

## 7. Technology

- **Frontend**: **SpecifyJS** (declarative TypeScript). Desktop shell + 7 applets
  + cross-cutting auth/RBAC/API-client/window stores. Real-time via WebSocket.
- **Backend**: control-plane API implementing `api/openapi.yaml`. Owns persistent
  state; reaches Nodes over SSH; orchestrates agent-containers; integrates GitHub
  (repos, Actions) and an LLM (spec → board). (Stack TBD.)
- **Data plane**: Convocate runtime + agent-containers on provisioned Nodes.
- **Shared**: generate TypeScript types from the OpenAPI schemas for end-to-end
  type safety.

---

## 8. Build sequence

1. **Contracts & types** — finalize `api/openapi.yaml`; generate shared TS types.
2. **Walking skeleton** — auth/session + RBAC + shell (dock, window manager) +
   one vertical slice (Node Manager: list → provision → detail) proving
   UI ↔ API ↔ datastore ↔ auth end-to-end.
3. **Real-time** — event channel for async job progress + agent-shell streaming.
4. **Applets** — Agent Manager, Access Control, Repo Manager, Support Tool, then
   Code IDE, then **Project Board** last (most custom: canvas/drag/graph/JSON).
5. **The loop** — wire IDE spec → render board → implement → PR → merge.
6. **Hardening** — validation parity, authz tests, CI/CD, deployment.

---

## 9. Open decisions

- Backend language/runtime and datastore.
- Agent-container orchestration mechanism on Nodes (Docker/Podman/other).
- Whether Nodes are user-supplied hosts (prototype assumption) or provisioned via
  a cloud API.
- Real-time transport details (WebSocket topics vs SSE) and event schema —
  to be added to the OpenAPI/AsyncAPI contract.
- LLM provider/runtime for spec→board rendering and the agent CLI.
