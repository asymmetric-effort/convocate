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

- Agent-container orchestration mechanism on Nodes (Docker/Podman/other).
- Whether Nodes are user-supplied hosts (prototype assumption) or provisioned via
  a cloud API.
- LLM provider/runtime for spec→board rendering and the agent CLI.

---

## 10. User interface specification

### 10.1 Convocate User Desktop

The "Convocate User Desktop" is a SpecifyJS SPA UI with hash-based routing, which looks like an Ubuntu unity desktop. In its "locked" state, the UI has a black screen with a modal form in the center prompting the user to login with username, password and MFA token. In its "unlocked" state, the UI has a dock to the left of the screen with the various convocate applets (as determined by user permissions), a black background, a menu bar at the top of the screen with the current date/time in the center of the menu bar and the currently logged in user to the right of the menu bar with a dropdown menu item on the right for user logout, screen lock and settings. The UI menubar will change its context like macOS based on the menu items of the currently active (focused) applet.

The "Convocate User Desktop" applets shall include the following: (1) Convocate Node Manager, (2) Convocate Agent Manager, (3) Convocate Project Board, (4) Convocate Code IDE, (5) Convocate Access Control, (6) Convocate Repo Manager, (7) Convocate Support Tool. Each of these applets shall be built using the SpecifyJS Application components. Access to each applet will require a user to belong to its associated user group. Each applet will enforce feature-level RBAC based on feature roles (e.g. node-create, node-view, node-delete, node-update, repo-create, etc.).

When the user first visits the "Convocate User Desktop" user interface, the interface must be in its "locked state" unless a valid JWT token is presented. In the "locked" state, the DOM will consist of a blank background with a login form in the center of the screen prompting the user for username, password and MFA token. When the user enters their credentials, the login UI will submit the authentication tokens via HTTPS to the backend POST /api/v1/auth/login API endpoint. If this API returns HTTP/401, the locked state will remain and the user will be presented with red text at the bottom of the login form saying "User login failed." If the API returns HTTP/200, the UI will transition from "locked" state to "unlocked" state, and the DOM will be replaced with the "Convocate User Desktop." In the "unlocked" state, the dock will present only the applets for which the user has authorization (e.g. a view role at minimum).

When a user clicks an applet in the dock, the current user's group/role object will be passed to the applet; and the applet will use this information to determine what features of an applet will be available (visible) to the user.

### 10.2 Convocate Node Manager

The top-most applet icon in the dock should be "Convocate Node Manager" applet. When a user clicks this icon in the dock, the applet will start and evaluate the user's permission object; and if the user has at least a node-view role, the applet will load — otherwise the applet will only display a modal message box that reads "Permission denied."

When an authorized user (holding a node-* role) loads the application, an applet window will load, the UI menu bar will load the Convocate Node Manager dropdown menu context, and the user will see a paginated spreadsheet-like grid-list of rows and columns with alternating colors, where each row represents a "Convocate Network Node" (virtual machines and physical hosts which run containerized agents). The grid-list of convocate network nodes will lazy load into the UI from /api/v1/nmgr/node?offset=M&limit=N. Each row will identify the node 'name' (uuid), user-defined location information, IP address and running status (online, draining, offline) and stats (number agent containers running) cpu load, memory used/free, disk used/free. The right-most column of a row contains context-specific control buttons. If a node is online, the 'stop' button will appear and when clicked it will mark the node as 'status:draining' and the scheduler will no longer dispatch agents to the node, and once all agents are stopped, the node will be transitioned to 'status:offline' and the node's convocate services will be terminated. The underlying host will remain running but Convocate will not be available. If a node is offline, the node's row in the applet UI will display the 'start' button; and when the user clicks 'start,' the convocate UI will call the API at POST /api/v1/nmgr/node/{nodeId}/start to start Convocate services and mark the node as online so the scheduler can begin dispatching tasks to the node. To start/stop a node, the user must hold the 'node-update' role.

When an authorized user (holding the node-*) role double clicks a node's row in the UI, a modal dialog will open in the applet, displaying the node's details, including detailed statistics and other information about the agents provisioned on the node. On this detailed screen will also be control buttons: start and stop (described above) and a delete button. When the user clicks the delete button, a confirmation dialog will appear; and once the user confirms the delete operation, the UI will send DELETE /api/v1/nmgr/node/{nodeId}. The API will then drain the node, terminate all Convocate services and uninstall Convocate software and power down the node.

The Convocate Node Manager Applet UI will have a "Provision Node" button, and when the user clicks, a modal dialog form will appear which the user will use to specify the connection information to allow the system to connect to the new node virtual machine or physical host via SSH and provision the new Convocate Node. When the form is submitted, the UI will invoke POST /api/v1/nmgr/node, and the Convocate API server will connect to the new host via SSH, install the Convocate Node software, start the services, verify the services and mark the new node as 'online' so that node is available to the scheduler for new agents to be dispatched.

### 10.3 Convocate Agent Manager

The "Convocate Agent Manager" applet will appear below the Convocate Node Manager in the dock. The Convocate Agent Manager will operate much like the Convocate Node Manager, only it will focus on the agent level of control. Convocate Node Manager manages the underlying virtual machines and physical hosts which run convocate agents. The Convocate Agent Manager is used to manage the containers on a convocate node which run claude agents. Each agent runs inside a golang wrapper program within a linux container with full permission bypass giving an agent complete and unfettered control over its container. The Agent Manager applet lists all agents in an accordion list of grid-list containers, where each accordion section represents a Convocate Node (host) containing a grid-list of agent containers running on the given host.

Each row of the agent-container instances identifies the agent-container name (uuid), logical name (project name), status (running, connected, stopped, migrating, stopping), ip:port (ip address and port on which the project may expose a listening service to the local area network), owner (user:group which owns/maintains the project). The right most column for the agent-container contains button-controls for starting/stopping/configuring the agent-container.

A user may double-click an agent-container row in the displayed list which will cause a window ("Agent Shell") to appear containing details about the container as well as a terminal screen the user may use to view the agent-container's stdout/stderr and interact with the container directly via stdin much like any shell.

The Convocate Agent Manager applet screen will also have a button ("Create"); when clicked this button will cause a new window to appear which will allow the user to create/configure a new agent-container instance. This same window will be used when the user later clicks the agent-container's configure button to edit the running container configuration.

### 10.4 Convocate Project Board

The "Convocate Project Board" will appear third in the dock, below Convocate Agent Manager. This applet will present a free board on which the user may place containers representing agent-containers and cards representing work items (tasks) to be executed against an associated agent-container.

Agent-containers can be mapped to a Project Board container object by right-clicking the title of the Project Board container and selecting "Map to Agent" from the context window then when the fly-out list appears, selecting any of the existing agent-containers. Agent-containers can be created/configured from the Project Board by right clicking the Project Board container and selecting "New Agent" and configuring the proposed agent-container in the "New Agent Window" form which will appear.

If a Task (card) in a Project Board is not associated with a Container, it is considered an "inert" task which cannot be executed as it has no runtime context. These free-floating objects can exist as notes, documentation or intentionally omitted tasks. A free-floating object can be associated with a container when the user drags the task (card) into a container and drops it. An associated card can be freed from its container by right-clicking the card and selecting "detach card" (sets `containerId` to null via the card update API).

Project Board tasks (card) can be linked together to create Directed Acyclic Graphs (DAGs), where the DAG shows how two or more task-card objects relate to one another for purposes of execution order in the agent-containers — even across more than one agent-container. This DAG treats cards as vertices and supports the following edges:

**RelatesTo:**
LHS relates to RHS and both must be completed to reach the definition of done but not in any specific order.

**DependsOn:**
LHS depends on RHS such that RHS must be completed before LHS is completed to achieve definition of done.

A card and container have a title bar; and when double-clicked the card/container will minimize to display only the title bar.

A user can right click a container and select "View Agent" from the context menu and open the agent-container record in the "Convocate Agent Manager" applet.

A user can right click a task card and select "View in Editor" from the context menu and open the task card contents in the Convocate Code IDE editor window.

The Project Board uses /api/v1/pb/{card,container,edge} endpoints to operate on the different cards, containers and edge interconnects between cards. Graphs, subgraphs and information about a Project Board is stored as a JSON object. Each Project Board instance represents a single project, and the applet context menu will have a "Graph" dropdown item, within which will be menu items which will —

- Allow the user to open a Project Board from an existing Convocate Code Repository;
- Allow the user to save a Project Board as a new Convocate Code Repository;
- Allow the user to close a Project Board;
- Allow the user to create a new (blank) Project Board.

### 10.5 Convocate Code IDE

The "Convocate Code IDE" will appear fourth in the dock, below the Convocate Project Board. This IDE applet will behave much like VS Code, providing a full-featured integrated development environment which can be used to edit code generated by convocate, configurations created by Project Board or other applets or otherwise allow users to create content.

### 10.6 Convocate Access Control

The "Convocate Access Control" will appear fifth in the dock, below the Convocate Code IDE. This applet will have a tabbed UI with tabs for "users," "groups," and "global settings."

The "users" tab will display a list of users and buttons to create, disable, enable, edit and delete user records. A user will be uniquely identified by their email address and a userId (UUID).

The "groups" tab will display a list of built-in and user-defined groups and buttons to create, edit, delete groups. A user can right-click a group (row in the list) and select "map users" from the context menu then map/unmap users to the given group. A user can right-click a group (row in the list) and select "map roles" and map/unmap roles to the given group.

### 10.7 Convocate Repo Manager

The "Convocate Repo Manager" will appear sixth in the dock, below the Convocate Access Control applet. This applet will create and manage git repositories for Convocate projects using the /api/v1/repo API endpoint. Using this applet a user should be able to list all Convocate project repositories, double click the repository to view its contents, double click files in the repository to open them in Convocate Code IDE or Convocate Project Board (as appropriate) or to download files. Convocate Repo Manager will act as a front-end for Github initially with plans to expand to other git-based version control systems.

### 10.8 Convocate Support Tool

The "Convocate Support Tool" will appear seventh in the dock, below Convocate Repo Manager. This tool will provide a dialog and use /api/v1/sup/ticket to create, read, update support tickets for the convocate system's internal administrators. The applet will also display convocate online documentation.
