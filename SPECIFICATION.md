# Convocate — Build Specification

> Status: draft v0.1 · Source of truth for implementation
> Companion artifact: `openapi.yaml` (API contract)

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
| **Node**  | A K8s cluster node (virtual or physical machine). Lifecycle managed via K8s API: `Ready → SchedulingDisabled (cordoned) → drained`. Carries resource capacity (CPU, memory, disk), labels (location, arch, os), taints, and write-once notes. |
| **Agent-container** | A K8s pod in the `convocate-agents` namespace running the Claude CLI in permission-bypass mode. Has a logical `project` name (label), owner (label), status (maps to K8s pod phase: `Running`/`Pending`/`Succeeded`/`Failed`), and optional K8s Service for network exposure. |
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
- **Control plane** — the Convocate API + SpecifyJS UI running in the `convocate`
  K8s namespace. Stateless-ish; owns all persistent state in a database.
- **Data plane** — **Agent-containers** running as K8s pods in the
  `convocate-agents` namespace. The control plane manages them via the K8s API
  (`k8s.io/client-go`). **Nodes** are K8s cluster nodes (virtual or physical
  machines). Node provisioning means adding machines to the K8s cluster. Agent
  migration, scheduling, and resource management are handled by K8s natively.

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
- Paginated node list sourced from the **K8s API** (`k8s.io/client-go`): node
  name, location (label), IP, status (Ready/NotReady/SchedulingDisabled),
  agent-container count, resource usage (CPU, memory, disk), controls.
- **Location** is stored as a K8s node label, inline-editable (double-click the
  cell; updates via K8s API).
- **Provision Node**: adding a machine to the K8s cluster. The dialog captures
  connection information; the backend orchestrates `kubeadm join` or equivalent.
- **Cordon** (SchedulingDisabled): prevents new agent-container pods from being
  scheduled. **Drain**: evicts existing pods; K8s reschedules them to other nodes
  with capacity automatically.
- **Uncordon**: re-enables scheduling on a cordoned node.
- **Detail** dialog: K8s node conditions, resource capacity/allocatable,
  agent-container pod list, labels/taints, and **write-once / read-many notes**.
- **Delete**: confirm → drain → remove node from the K8s cluster.

### 5.2 Agent Manager (`amgr`)
- Accordion grouped by node; each agent row shows the **K8s pod** status:
  id (pod name), project (label), status (Running/Pending/Succeeded/Failed),
  node assignment, owner (label), controls (Start/Stop/Config/Shell).
- Agent-containers run as pods in the **`convocate-agents` namespace**, separate
  from core Convocate services in the `convocate` namespace. Each pod runs the
  `convocate/agent` container image (see §12 for full container specification).
- **Shell**: opens a terminal window streaming the agent's **Claude CLI**
  stdout/stderr via WebSocket and accepting stdin input — relayed by the Go
  wrapper inside the agent-container.
- **Create Agent**: creates a pod in `convocate-agents`. The create API accepts
  K8s pod configuration (resource limits, node selector, security overrides)
  and Claude CLI flags (e.g. `--dangerously-skip-permissions`). See §12.5.
- **Configure Agent**: updates pod labels, node affinity, resource limits,
  security overrides (admin-only), network policy rules, logging settings,
  and **Expose** (creates a K8s Service for the agent pod). Configuration
  changes that affect the Claude CLI (e.g. CLAUDE.md guardrails) are applied
  via ConfigMap update; the Go wrapper detects the change and restarts Claude
  CLI without restarting the pod.
- **Start/Stop**: creates or deletes the agent pod. PVC persists across
  stop/start cycles, preserving Claude memories and session state.
- **Migration**: handled natively by K8s pod eviction and rescheduling.

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

- **Frontend**: **SpecifyJS** (declarative TypeScript, `@asymmetric-effort/specifyjs`)
  running on **Bun**. Desktop shell + 7 applets + cross-cutting auth/RBAC/API-client/
  window stores. Real-time via WebSocket (`/api/v1/events/{applet}/{channel}`).
- **Backend**: **Go 1.26+** control-plane API implementing `openapi.yaml`. Owns
  persistent state; manages K8s nodes and agent-container pods via `k8s.io/client-go`;
  integrates GitHub (repos, Actions) and an LLM (spec → board).
- **Storage**: tiered — **file-based JSON** for data blobs (boards, specs, card
  content, docs), **Redis** (`redis/go-redis`) for ephemeral/cache (JWT sessions,
  refresh tokens, stats cache, event pub/sub), **PostgreSQL** (`database/sql` +
  `jackc/pgx`) for searchable records and file references.
- **Data plane**: Agent-containers as K8s pods in the `convocate-agents` namespace,
  scheduled across cluster worker nodes. K8s handles migration, scaling, and
  resource management natively. Each agent-container runs a Go wrapper binary
  managing a single Claude CLI instance (see §12).
- **Node metrics**: `node-metrics` DaemonSet on every cluster node reads
  `/proc` and filesystem stats, pushes to the API every 3 seconds.
- **Secrets**: **OpenBao** (`openbao/openbao`) for secret storage (JWT signing
  keys, database credentials, OAuth client secrets). Filesystem-backed persistence.
- **Orchestration**: **K8s API** (`k8s.io/client-go`) for node management
  (cordon/drain/uncordon), agent-container pod lifecycle (create/delete/exec),
  and service exposure.
- **Containers**: all services run in **distroless** containers built via
  multi-stage Docker builds (build stage: `ubuntu:24.04`). Local development
  via **Docker Compose** (UI, API, Redis, PostgreSQL, OpenBao).

---

## 8. Build sequence

1. **Contracts & types** — finalize `openapi.yaml`; hand-write shared TS types to match the OpenAPI schemas.
2. **Walking skeleton** — auth/session + RBAC + shell (dock, window manager) +
   one vertical slice (Node Manager: list → provision → detail) proving
   UI ↔ API ↔ datastore ↔ auth end-to-end.
3. **Real-time** — event channel for async job progress + agent-shell streaming.
4. **Applets** — Agent Manager, Access Control, Repo Manager, Support Tool, then
   Code IDE, then **Project Board** last (most custom: canvas/drag/graph/JSON).
5. **The loop** — wire IDE spec → render board → implement → PR → merge.
6. **Hardening** — validation parity, authz tests, CI/CD, deployment.

---

## 9. Build automation (Makefile)

The project uses a top-level `Makefile` to drive all build, test, lint and
clean operations. All targets are designed to run locally and in CI.

| Target        | Description |
|---------------|-------------|
| `make clean`  | Remove all container images and built artifacts in `build/`, then recreate the empty `build/` directory. |
| `make lint`   | Run all linters: Go (`gofmt`, `go vet`), TypeScript, SQL, Markdown, Makefiles, JS/CSS/HTML, YAML, JSON, and Dockerfiles (`hadolint`). |
| `make test`   | Run all unit, integration and end-to-end tests, including Playwright browser tests, locally. |
| `make build`  | Build all container images (UI, API, Redis, PostgreSQL), the GitHub Pages website artifacts, and any other distributable outputs. |
| `make cover`  | Run code coverage across Go and TypeScript and **fail if overall test coverage is below 98%**. |

---

## 10. Open decisions

- LLM provider/runtime for spec→board rendering and the agent CLI.
- Node provisioning mechanism (kubeadm join, cloud provider API, manual).

---

## 11. User interface specification

### 11.1 Convocate User Desktop

The "Convocate User Desktop" is a SpecifyJS SPA UI with hash-based routing, which looks like an Ubuntu unity desktop. In its "locked" state, the UI has a black screen with a modal form in the center prompting the user to login with username, password and MFA token. In its "unlocked" state, the UI has a dock to the left of the screen with the various convocate applets (as determined by user permissions), a black background, a menu bar at the top of the screen with the current date/time in the center of the menu bar and the currently logged in user to the right of the menu bar with a dropdown menu item on the right for user logout, screen lock and settings. The UI menubar will change its context like macOS based on the menu items of the currently active (focused) applet.

The "Convocate User Desktop" applets shall include the following: (1) Convocate Node Manager, (2) Convocate Agent Manager, (3) Convocate Project Board, (4) Convocate Code IDE, (5) Convocate Access Control, (6) Convocate Repo Manager, (7) Convocate Support Tool. Each of these applets shall be built using the SpecifyJS Application components. Access to each applet will require a user to belong to its associated user group. Each applet will enforce feature-level RBAC based on feature roles (e.g. node-create, node-view, node-delete, node-update, repo-create, etc.).

When the user first visits the "Convocate User Desktop" user interface, the interface must be in its "locked state" unless a valid JWT token is presented. In the "locked" state, the DOM will consist of a blank background with a login form in the center of the screen prompting the user for username, password and MFA token. When the user enters their credentials, the login UI will submit the authentication tokens via HTTPS to the backend POST /api/v1/auth/login API endpoint. If this API returns HTTP/401, the locked state will remain and the user will be presented with red text at the bottom of the login form saying "User login failed." If the API returns HTTP/200, the UI will transition from "locked" state to "unlocked" state, and the DOM will be replaced with the "Convocate User Desktop." In the "unlocked" state, the dock will present only the applets for which the user has authorization (e.g. a view role at minimum).

When a user clicks an applet in the dock, the current user's group/role object will be passed to the applet; and the applet will use this information to determine what features of an applet will be available (visible) to the user.

### 11.2 Convocate Node Manager

The top-most applet icon in the dock should be "Convocate Node Manager" applet. When a user clicks this icon in the dock, the applet will start and evaluate the user's permission object; and if the user has at least a node-view role, the applet will load — otherwise the applet will only display a modal message box that reads "Permission denied."

When an authorized user (holding a node-* role) loads the application, an applet window will load, the UI menu bar will load the Convocate Node Manager dropdown menu context, and the user will see a paginated spreadsheet-like grid-list of rows and columns with alternating colors, where each row represents a "Convocate Network Node" (virtual machines and physical hosts which run containerized agents). The grid-list of convocate network nodes will lazy load into the UI from /api/v1/nmgr/node?offset=M&limit=N. Each row will identify the node 'name' (uuid), user-defined location information, IP address and running status (online, draining, offline) and stats (number agent containers running) cpu load, memory used/free, disk used/free. The right-most column of a row contains context-specific control buttons. If a node is online, the 'stop' button will appear and when clicked it will mark the node as 'status:draining' and the scheduler will no longer dispatch agents to the node, and once all agents are stopped, the node will be transitioned to 'status:offline' and the node's convocate services will be terminated. The underlying host will remain running but Convocate will not be available. If a node is offline, the node's row in the applet UI will display the 'start' button; and when the user clicks 'start,' the convocate UI will call the API at POST /api/v1/nmgr/node/{nodeId}/start to start Convocate services and mark the node as online so the scheduler can begin dispatching tasks to the node. To start/stop a node, the user must hold the 'node-update' role.

When an authorized user (holding the node-*) role double clicks a node's row in the UI, a modal dialog will open in the applet, displaying the node's details, including detailed statistics and other information about the agents provisioned on the node. On this detailed screen will also be control buttons: start and stop (described above) and a delete button. When the user clicks the delete button, a confirmation dialog will appear; and once the user confirms the delete operation, the UI will send DELETE /api/v1/nmgr/node/{nodeId}. The API will then drain the node, terminate all Convocate services and uninstall Convocate software and power down the node.

The Convocate Node Manager Applet UI will have a "Provision Node" button, and when the user clicks, a modal dialog form will appear which the user will use to specify the connection information to allow the system to connect to the new node virtual machine or physical host via SSH and provision the new Convocate Node. When the form is submitted, the UI will invoke POST /api/v1/nmgr/node, and the Convocate API server will connect to the new host via SSH, install the Convocate Node software, start the services, verify the services and mark the new node as 'online' so that node is available to the scheduler for new agents to be dispatched.

### 11.3 Convocate Agent Manager

The "Convocate Agent Manager" applet will appear below the Convocate Node Manager in the dock. The Convocate Agent Manager will operate much like the Convocate Node Manager, only it will focus on the agent level of control. Convocate Node Manager manages the underlying virtual machines and physical hosts which run convocate agents. The Convocate Agent Manager is used to manage the containers on a convocate node which run claude agents. Each agent runs inside a golang wrapper program within a linux container with full permission bypass giving an agent complete and unfettered control over its container. The Agent Manager applet lists all agents in an accordion list of grid-list containers, where each accordion section represents a Convocate Node (host) containing a grid-list of agent containers running on the given host.

Each row of the agent-container instances identifies the agent-container name (uuid), logical name (project name), status (running, connected, stopped, migrating, stopping), ip:port (ip address and port on which the project may expose a listening service to the local area network), owner (user:group which owns/maintains the project). The right most column for the agent-container contains button-controls for starting/stopping/configuring the agent-container.

A user may double-click an agent-container row in the displayed list which will cause a window ("Agent Shell") to appear containing details about the container as well as a terminal screen the user may use to view the agent-container's stdout/stderr and interact with the container directly via stdin much like any shell.

The Convocate Agent Manager applet screen will also have a button ("Create"); when clicked this button will cause a new window to appear which will allow the user to create/configure a new agent-container instance. This same window will be used when the user later clicks the agent-container's configure button to edit the running container configuration.

### 11.4 Convocate Project Board

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

### 11.5 Convocate Code IDE

The "Convocate Code IDE" will appear fourth in the dock, below the Convocate Project Board. This IDE applet will behave much like VS Code, providing a full-featured integrated development environment which can be used to edit code generated by convocate, configurations created by Project Board or other applets or otherwise allow users to create content.

### 11.6 Convocate Access Control

The "Convocate Access Control" will appear fifth in the dock, below the Convocate Code IDE. This applet will have a tabbed UI with tabs for "users," "groups," and "global settings."

The "users" tab will display a list of users and buttons to create, disable, enable, edit and delete user records. A user will be uniquely identified by their email address and a userId (UUID).

The "groups" tab will display a list of built-in and user-defined groups and buttons to create, edit, delete groups. A user can right-click a group (row in the list) and select "map users" from the context menu then map/unmap users to the given group. A user can right-click a group (row in the list) and select "map roles" and map/unmap roles to the given group.

### 11.7 Convocate Repo Manager

The "Convocate Repo Manager" will appear sixth in the dock, below the Convocate Access Control applet. This applet will create and manage git repositories for Convocate projects using the /api/v1/repo API endpoint. Using this applet a user should be able to list all Convocate project repositories, double click the repository to view its contents, double click files in the repository to open them in Convocate Code IDE or Convocate Project Board (as appropriate) or to download files. Convocate Repo Manager will act as a front-end for Github initially with plans to expand to other git-based version control systems.

### 11.8 Convocate Support Tool

The "Convocate Support Tool" will appear seventh in the dock, below Convocate Repo Manager. This tool will provide a dialog and use /api/v1/sup/ticket to create, read, update support tickets for the convocate system's internal administrators. The applet will also display convocate online documentation.

---

## 12. Agent-Container Specification (`convocate/agent`)

An **agent-container** is the unit of AI compute in Convocate. Each agent-container
runs as a K8s pod in the `convocate-agents` namespace, encapsulating a single
Claude CLI instance managed by a Go wrapper binary. Convocate can run many
agent-containers concurrently, each operating independently on a dedicated task
or project.

### 12.1 Container Image

The `convocate/agent` image is built via a multi-stage Docker process:

| Stage | Base Image | Purpose |
|-------|-----------|---------|
| Build | `ubuntu:24.04` | Install Go 1.26+, Node.js, npm. Compile the Go wrapper binary. Install Claude CLI via `npm install -g @anthropic-ai/claude-code@<pinned-version>`. |
| Runtime | `ubuntu:24.04` (minimal) | Copy Go binary, Claude CLI, Node.js runtime. Strip unnecessary packages, docs, caches to minimize attack surface and image size. No compilers, no dev tools, no package manager caches. |

The Claude CLI version is **pinned at build time**. Upgrading Claude Code requires
rebuilding the container image, ensuring the CLI lifecycle is managed through the
container lifecycle.

**Runtime user**: `claude` (UID 1337, GID 1337). The Go wrapper and Claude CLI
both run as this non-root user.

**ENTRYPOINT**: `/usr/bin/convocate-agent-wrapper`

### 12.2 Go Wrapper Binary (`convocate-agent-wrapper`)

The Go wrapper is the agent-container's main process and sole ENTRYPOINT. It:

1. Starts the Claude CLI in a shell as a background process (via goroutine).
2. Manages stdin/stdout/stderr communication with Claude CLI via Go channels.
3. Exposes an HTTPS API for authenticated control and I/O relay.
4. Watches for configuration changes (CLAUDE.md) and restarts Claude CLI
   without restarting the pod.
5. Handles graceful shutdown on SIGTERM.

#### 12.2.1 I/O Architecture

The wrapper is a **transparent bidirectional relay** between the Convocate API
and the Claude CLI process. No parsing, no prompt detection, no transformation.
Raw bytes in, raw bytes out.

```
Convocate API                    Go Wrapper                     Claude CLI
                                 (convocate-agent-wrapper)       (shell)

POST /stdin    ──────────────►   go channel ──► process.stdin
WS   /stdout   ◄──────────────   go channel ◄── process.stdout
WS   /stderr   ◄──────────────   go channel ◄── process.stderr
POST /control  ──────────────►   (restart, signal, config)
```

- **stdin**: API posts raw bytes; wrapper writes them to the shell's stdin
  exactly as received.
- **stdout**: Wrapper reads the shell's stdout and streams raw bytes to
  subscribers via a dedicated WebSocket channel.
- **stderr**: Same as stdout but on a separate WebSocket channel.
- **control**: Separate API endpoints for lifecycle operations (restart
  Claude CLI, send OS signals, update configuration).

This design supports a future web-based terminal emulator applet in Convocate
that opens stdout/stderr WebSocket streams and posts keystrokes to the stdin
endpoint.

#### 12.2.2 Wrapper API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/healthz` | GET | Liveness probe — returns 200 if wrapper process is running |
| `/readyz` | GET | Readiness probe — returns 200 only when Claude CLI is running and stdin is writable |
| `/metrics` | GET | Usage and performance statistics (see §12.2.3) |
| `/stdin` | POST | Write raw bytes to Claude CLI's stdin |
| `/stdout` | GET (WebSocket) | Stream Claude CLI's stdout in real-time |
| `/stderr` | GET (WebSocket) | Stream Claude CLI's stderr in real-time |
| `/control/restart` | POST | Gracefully restart Claude CLI (finish current output, then restart) |
| `/control/signal` | POST | Send an OS signal to the Claude CLI process |

#### 12.2.3 Metrics Endpoint

The `/metrics` endpoint returns a JSON object with usage and performance
statistics:

| Field | Type | Description |
|-------|------|-------------|
| `stdinBytes` | int64 | Cumulative bytes written to stdin |
| `stdoutBytes` | int64 | Cumulative bytes read from stdout |
| `stderrBytes` | int64 | Cumulative bytes read from stderr |
| `stdinMessages` | int64 | Number of stdin write operations |
| `stdoutMessages` | int64 | Number of stdout read operations |
| `stderrMessages` | int64 | Number of stderr read operations |
| `claudeRestarts` | int | Number of Claude CLI restarts (config change, crash, etc.) |
| `wrapperVersion` | string | Go binary version (compiled in at build time) |
| `claudeCodeVersion` | string | Claude CLI version (read at startup) |
| `uptimeSeconds` | int64 | Wrapper process uptime |
| `claudeUptimeSeconds` | int64 | Current Claude CLI process uptime (resets on restart) |
| `activeConnections` | int | Number of open WebSocket subscribers |
| `podName` | string | K8s pod name |
| `nodeName` | string | K8s node the pod is running on |

#### 12.2.4 Configuration Watch

The Go wrapper monitors the CLAUDE.md configuration file (mounted read-only
from a K8s ConfigMap) for changes using `fsnotify`. When a change is detected:

1. The wrapper waits for any in-flight Claude CLI response to complete.
2. Sends SIGTERM to the Claude CLI process.
3. Waits for graceful exit (up to 10 seconds).
4. Starts a new Claude CLI process with the updated configuration.
5. Increments the `claudeRestarts` metric counter.

This allows the Agent Manager to update guardrails without restarting the pod
or losing the PVC state.

**Note**: `fsnotify` is an allowed third-party dependency, flagged for eventual
replacement with a stdlib-based solution.

### 12.3 Authentication & Security

Agent-containers enforce three layers of security:

#### 12.3.1 TLS (encryption in-flight)

All communication between the Convocate API and agent-container wrapper uses
TLS. Certificates are issued per-pod by **K8s cert-manager**. The wrapper
loads its TLS certificate and key from a K8s Secret volume mount provisioned
by cert-manager.

#### 12.3.2 K8s ServiceAccount Authentication

Container-to-container authenticity is verified using K8s projected service
account tokens. The Convocate API validates the agent pod's K8s SA token, and
the agent wrapper validates the API's K8s SA token. This ensures only
authorized Convocate components can communicate with agent-containers.

#### 12.3.3 Per-Agent JWT RBAC

The Go wrapper validates user JWT tokens to enforce that the requesting user
has permission to interact with the specific agent-container. This is
consistent with Convocate's RBAC model:

- `agent-view` — can open stdout/stderr streams, view metrics
- `agent-update` — can write to stdin, send control commands, modify config
- `admin` — can modify security context and relaxed settings

The wrapper requires access to the JWT signing public key (mounted from a
K8s Secret) to validate tokens.

#### 12.3.4 Claude CLI Authentication

Agent-containers support two methods for authenticating Claude CLI with
Anthropic:

- **API key mode**: `ANTHROPIC_API_KEY` environment variable set from a K8s
  Secret. Claude CLI starts immediately without user interaction.
- **OAuth mode**: No API key configured. Claude CLI initiates the OAuth device
  code flow. The Go wrapper relays the device code and URL through the
  stdout WebSocket. The user completes authentication in their browser. The
  OAuth token is stored on the PVC and persists across restarts.

### 12.4 K8s Pod Specification

#### 12.4.1 Filesystem Layout

```
/usr/bin/convocate-agent-wrapper     ← Go binary (ENTRYPOINT)
/usr/lib/node_modules/              ← Claude CLI (pinned version)
/home/claude/
├── CLAUDE.md                       ← ConfigMap (read-only, managed by Agent Manager)
├── workspace/                      ← PVC (per-instance, persistent, 2Gi default)
│   ├── .claude/                    ← Claude memories, sessions, projects
│   └── <project files>/            ← working directory
/tmp/                               ← tmpfs (writable, ephemeral)
```

- The PVC at `/home/claude/workspace/` is **dedicated to a single agent
  instance** and inaccessible to other agent-containers.
- Claude CLI's working directory is `/home/claude/workspace/`.
- Claude memories and sessions persist on the PVC across stop/start cycles.
- The CLAUDE.md ConfigMap is mounted at `/home/claude/CLAUDE.md` as a
  read-only file. It is **strictly immutable** from the agent-container's
  perspective — only the Agent Manager can update it.

#### 12.4.2 Resource Defaults

| Resource | Request | Limit | Configurable |
|----------|---------|-------|-------------|
| CPU | 500m | 2 cores | Yes, at create and update time |
| Memory | 512Mi | 2Gi | Yes, at create and update time |
| PVC storage | — | 2Gi | Yes, at create time |

All resource values are configurable per agent via the Agent Manager API,
both at creation time and during the agent's lifecycle.

#### 12.4.3 Security Context (Defaults)

The following security settings are applied to every agent pod by default.
These follow the K8s **restricted** Pod Security Standard:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1337
  runAsGroup: 1337
  seccompProfile:
    type: RuntimeDefault
containers:
  - securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop: ["ALL"]
```

Writable paths: PVC at `/home/claude/workspace/` and tmpfs at `/tmp/`.

**Admin-only overrides**: Users holding the `admin` role can relax security
settings per agent via the Agent Manager API. This includes:

- Adding specific Linux capabilities
- Granting Docker/containerd socket access (for container image builds)
- Mounting additional host paths
- Modifying the seccomp profile

Non-admin users can create and manage agents but **cannot modify security
context, capabilities, or grant Docker/containerd access**.

All security overrides are audit-logged (who, what, when).

#### 12.4.4 Network Policy

Agent-containers follow **least-privilege** networking by default:

**Default egress (outbound) allow list**:
- `api.anthropic.com` — Anthropic API (Claude)
- Convocate API service — control plane communication
- `github.com`, `api.github.com` — Git operations (HTTPS + SSH)
- `npm.pkg.github.com`, `registry.npmjs.org` — npm packages
- `pypi.org`, `files.pythonhosted.org` — Python packages

**Default ingress (inbound) allow list**:
- Convocate API service pods only

**Default deny**: all other traffic, including inter-agent communication
and access to other cluster services.

The Agent Manager API can **add or remove egress rules per agent** (e.g.
allow a private registry, allow a database endpoint). Network policy changes
are applied as per-agent K8s NetworkPolicy objects.

### 12.5 Agent Manager API Extensions

The Agent Manager create and update APIs accept the following additional
fields for agent-container configuration:

#### 12.5.1 Create Agent Request

```json
{
  "project": "my-project",
  "nodeId": "convocate04",
  "image": "convocate/agent:latest",
  "claudeFlags": ["--dangerously-skip-permissions"],
  "resources": {
    "cpuRequest": "500m",
    "cpuLimit": "2",
    "memoryRequest": "512Mi",
    "memoryLimit": "2Gi",
    "storageSize": "2Gi"
  },
  "security": {
    "capabilities": [],
    "dockerAccess": false,
    "additionalMounts": []
  },
  "network": {
    "additionalEgress": []
  },
  "logging": false,
  "anthropicApiKey": "sk-...",
  "claudeMd": "# Custom guardrails\n..."
}
```

- `claudeFlags` — array of CLI flags passed to Claude CLI on startup.
- `resources` — overrides for CPU, memory, and storage defaults.
- `security` — admin-only fields; rejected with 403 if caller lacks `admin`.
- `network` — additional egress rules beyond the default allow list.
- `logging` — when `true`, Claude I/O is logged as structured JSON to the
  pod's stdout/stderr for collection by the cluster logging service.
- `anthropicApiKey` — stored as a K8s Secret, injected as env var.
- `claudeMd` — custom CLAUDE.md content for this agent's ConfigMap.

#### 12.5.2 Update Agent Request

All fields from the create request can be updated during the agent's
lifecycle, with the following behaviors:

- `resources` — applied via K8s pod patch (may require pod restart for
  some fields like PVC size).
- `security` — admin-only; applied via pod spec update.
- `network` — applied by updating the per-agent NetworkPolicy object.
- `logging` — toggled without restart (wrapper reads from config).
- `claudeMd` — applied by updating the ConfigMap; wrapper detects the
  change and restarts Claude CLI without restarting the pod.
- `claudeFlags` — requires Claude CLI restart (wrapper handles this).

### 12.6 Lifecycle

#### 12.6.1 Pod Startup Sequence

1. K8s schedules the pod on a node with available resources.
2. cert-manager provisions a TLS certificate for the pod.
3. The PVC is mounted (created if first run, reattached if existing).
4. The ConfigMap is mounted at `/home/claude/CLAUDE.md`.
5. The Go wrapper starts, loads TLS certs, and begins listening on HTTPS.
6. The wrapper spawns Claude CLI in a shell with the configured flags.
7. The wrapper reports readiness via `/readyz` once Claude CLI is accepting
   stdin.

#### 12.6.2 Graceful Shutdown

When K8s sends SIGTERM to the pod:

1. The Go wrapper catches SIGTERM.
2. Forwards SIGTERM to the Claude CLI shell process.
3. Waits for Claude CLI to exit (up to the `terminationGracePeriodSeconds`,
   default 30 seconds).
4. Closes all WebSocket connections.
5. Exits with code 0.

If Claude CLI does not exit within the grace period, K8s sends SIGKILL.

#### 12.6.3 Persistence

- **PVC**: Claude memories, sessions, project files, and OAuth tokens
  persist across stop/start cycles. Each agent-container has a dedicated
  PVC that is **not shared** with other agent-containers.
- **ConfigMap**: Guardrails (CLAUDE.md) are managed by the Agent Manager
  and updated independently of the pod lifecycle.
- **Metrics**: Wrapper metrics are ephemeral (reset on pod restart).
  Historical metrics are available through the K8s cluster monitoring
  stack if configured.

#### 12.6.4 Scaling

Convocate scales AI compute horizontally by spawning more agent-container
pods, not by multiplexing within a single pod. Each pod runs exactly one
Claude CLI instance. The K8s scheduler distributes pods across nodes based
on resource availability and any node selector constraints specified at
agent creation time.
