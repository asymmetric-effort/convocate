/**
 * Agent Manager Applet
 *
 * Displays agent-container pods grouped by node in an accordion layout.
 * Supports creating, starting, stopping, configuring, and deleting agents.
 *
 * Data source:
 *   - Initial load: GET /api/v1/amgr/agent (paginated)
 *   - Live updates: SSE /api/v1/events/amgr/status
 */

import { createElement, useState, useEffect, useCallback, useRef } from "@asymmetric-effort/specifyjs";
import {
  Accordion,
  BuildableList,
  Button,
  Modal,
  Select,
  TextField,
  Spinner,
  Tag,
  DataGrid,
} from "@asymmetric-effort/specifyjs/components";
import { useWebSocket, ServerEvent } from "./use-websocket";
import { useMenuBar } from "./use-menu-bar";
import { fetchProjects, createProject, validateProjectName } from "./shared-projects";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Agent {
  id: string;
  project: string;
  nodeId: string;
  status: string;
  expose: string;
  owner: string;
}

interface PageResponse {
  items: Agent[];
  offset: number;
  limit: number;
  total: number;
}

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

async function fetchAgents(offset: number, limit: number): Promise<PageResponse> {
  const res = await fetch(
    `/api/v1/amgr/agent?offset=${offset}&limit=${limit}`,
    { headers: authHeaders() }
  );
  if (!res.ok) throw new Error(`Failed to fetch agents: ${res.status}`);
  return res.json();
}

async function fetchAgent(id: string): Promise<Agent> {
  const res = await fetch(`/api/v1/amgr/agent/${id}`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch agent: ${res.status}`);
  return res.json();
}

async function createAgent(data: {
  project: string;
  nodeId: string;
  image?: string;
  command?: string;
}): Promise<Agent> {
  const res = await fetch("/api/v1/amgr/agent", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `Create failed: ${res.status}`);
  }
  return res.json();
}

async function stopAgent(id: string): Promise<void> {
  const res = await fetch(`/api/v1/amgr/agent/${id}/stop`, {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Stop failed: ${res.status}`);
}

async function deleteAgent(id: string): Promise<void> {
  const res = await fetch(`/api/v1/amgr/agent/${id}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Delete failed: ${res.status}`);
}

// ---------------------------------------------------------------------------
// Status tag helper
// ---------------------------------------------------------------------------

function statusTag(status: string) {
  switch (status) {
    case "running":
      return h(Tag, { label: "Running", color: "green", variant: "solid" as const, size: "sm" as const });
    case "connected":
      return h(Tag, { label: "Connected", color: "#2563eb", variant: "solid" as const, size: "sm" as const });
    case "stopped":
      return h(Tag, { label: "Stopped", color: "#6b7280", variant: "solid" as const, size: "sm" as const });
    case "migrating":
      return h(Tag, { label: "Migrating", color: "#b45309", variant: "solid" as const, size: "sm" as const });
    case "stopping":
      return h(Tag, { label: "Stopping", color: "#b45309", variant: "solid" as const, size: "sm" as const });
    default:
      return h(Tag, { label: status, color: "gray", variant: "subtle" as const, size: "sm" as const });
  }
}

// ---------------------------------------------------------------------------
// Create Agent Dialog
// ---------------------------------------------------------------------------

function CreateAgentDialog({
  open,
  onClose,
  onCreated,
  availableNodes,
  isAdmin,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
  availableNodes: string[];
  isAdmin: boolean;
}) {
  const [project, setProject] = useState("");
  const [isNewProject, setIsNewProject] = useState(false);
  const [claudeFlags, setClaudeFlags] = useState<string[]>(["--dangerously-skip-permissions"]);
  const [cpuLimit, setCpuLimit] = useState("2");
  const [memoryLimit, setMemoryLimit] = useState("2Gi");
  const [storageSize, setStorageSize] = useState("2Gi");
  const [capabilities, setCapabilities] = useState<string[]>([]);
  const [additionalEgress, setAdditionalEgress] = useState<string[]>([]);
  const [claudeMd, setClaudeMd] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [logging, setLogging] = useState(false);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [existingProjects, setExistingProjects] = useState<string[]>([]);

  // Fetch existing project names when dialog opens
  useEffect(() => {
    if (!open) return;
    fetchProjects().then((projects) => {
      setExistingProjects(projects.map((p) => p.name));
    }).catch(() => {});
  }, [open]);

  const handleSubmit = useCallback(async () => {
    if (!project.trim()) { setError("Please select or create a project."); return; }

    // For new projects: validate name and create via unified API
    if (isNewProject) {
      const nameError = validateProjectName(project);
      if (nameError) { setError(nameError); return; }
      try {
        await createProject(project.trim());
      } catch (err: any) {
        setError(err.message || "Failed to create project.");
        return;
      }
    }
    setError("");
    setSubmitting(true);
    try {
      const req: any = {
        project: project.trim(),
        nodeId: "", // Let K8s scheduler decide placement
        claudeFlags: claudeFlags.length > 0 ? claudeFlags : undefined,
        resources: { cpuLimit, memoryLimit, storageSize },
        claudeMd: claudeMd.trim() || undefined,
        anthropicApiKey: apiKey.trim() || undefined,
        logging,
      };
      // Admin-only security fields
      if (isAdmin && capabilities.length > 0) {
        req.security = { capabilities };
      }
      // Network policy
      if (additionalEgress.length > 0) {
        req.network = { additionalEgress };
      }
      await createAgent(req);
      setProject(""); setIsNewProject(false); setClaudeFlags(["--dangerously-skip-permissions"]);
      setCpuLimit("2"); setMemoryLimit("2Gi"); setStorageSize("2Gi");
      setCapabilities([]); setAdditionalEgress([]);
      setClaudeMd(""); setApiKey(""); setLogging(false);
      onCreated();
      onClose();
    } catch (err: any) {
      setError(err.message || "Create failed.");
    } finally {
      setSubmitting(false);
    }
  }, [project, claudeFlags, cpuLimit, memoryLimit, storageSize, capabilities, additionalEgress, claudeMd, apiKey, logging, isAdmin, existingProjects, onCreated, onClose]);

  if (!open) return null;

  const darkStyle = { display: "flex", flexDirection: "column" as const, gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px", maxHeight: "60vh", overflowY: "auto" as const };
  const sectionLabel = (text: string) => h("div", { style: { fontSize: "11px", color: "#aaa", marginTop: "4px", textTransform: "uppercase", letterSpacing: "1px" } }, text);

  return h(
    Modal,
    { open: true, onClose, title: "Create Agent", size: "lg" as const },
    h("div", { style: darkStyle },
      // Core fields — select existing project or create new
      sectionLabel("Project"),
      h("div", { style: { backgroundColor: "#f8f9fa", borderRadius: "4px", padding: "4px" } },
        h(Select, {
          options: [
            ...existingProjects.map((name) => ({ value: name, label: name })),
            { value: "__new__", label: "+ Create new project..." },
          ],
          value: isNewProject ? "__new__" : project,
          onChange: (val: string) => {
            if (val === "__new__") {
              setIsNewProject(true);
              setProject("");
            } else {
              setIsNewProject(false);
              setProject(val);
            }
          },
          placeholder: "Select a project...",
          searchable: true,
          label: "Project",
        })
      ),
      // Show name input when creating a new project
      isNewProject ? h("div", { style: { marginTop: "8px" } },
        h(TextField, {
          placeholder: "New project name (letters, digits, hyphens, underscores)",
          value: project,
          onChange: (v: string) => setProject(v),
        })
      ) : null,

      // Claude CLI flags
      h("div", { style: { backgroundColor: "#f8f9fa", borderRadius: "4px", padding: "4px" } },
        h(BuildableList, {
          value: claudeFlags,
          onChange: (items: string[]) => setClaudeFlags(items),
          placeholder: "e.g. --dangerously-skip-permissions",
          label: "Claude CLI flags",
        })
      ),
      h(TextField, { placeholder: "Anthropic API Key (optional, for headless auth)", type: "password", value: apiKey, onChange: (v: string) => setApiKey(v) }),

      // Resources
      sectionLabel("Resources"),
      h("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "8px" } },
        h(TextField, { placeholder: "CPU limit (e.g. 2)", value: cpuLimit, onChange: (v: string) => setCpuLimit(v) }),
        h(TextField, { placeholder: "Memory limit (e.g. 2Gi)", value: memoryLimit, onChange: (v: string) => setMemoryLimit(v) }),
        h(TextField, { placeholder: "Storage (e.g. 2Gi)", value: storageSize, onChange: (v: string) => setStorageSize(v) }),
      ),

      // K8s Capabilities (admin-only)
      isAdmin ? sectionLabel("K8s Capabilities (admin)") : null,
      isAdmin ? h("div", { style: { backgroundColor: "#f8f9fa", borderRadius: "4px", padding: "4px" } },
        h(BuildableList, {
          value: capabilities,
          onChange: (items: string[]) => setCapabilities(items),
          placeholder: "e.g. NET_ADMIN",
          label: "Linux capabilities",
        })
      ) : null,

      // Network Policy
      sectionLabel("Network Policy"),
      h("div", { style: { backgroundColor: "#f8f9fa", borderRadius: "4px", padding: "4px" } },
        h(BuildableList, {
          value: additionalEgress,
          onChange: (items: string[]) => setAdditionalEgress(items),
          placeholder: "e.g. my-registry.io",
          label: "Additional egress hosts",
        })
      ),
      h("div", { style: { fontSize: "11px", color: "#666" } },
        "Default: Anthropic API, GitHub, npm, PyPI. Add extra hosts above."
      ),

      // CLAUDE.md guardrails
      sectionLabel("CLAUDE.md Guardrails"),
      h("textarea", {
        value: claudeMd,
        onInput: (e: Event) => setClaudeMd((e.target as HTMLTextAreaElement).value),
        placeholder: "# Agent Instructions\nCustom guardrails for this agent...",
        style: { width: "100%", minHeight: "60px", backgroundColor: "#2d2d2d", color: "#e0e0e0", border: "1px solid #444", borderRadius: "4px", padding: "8px", fontSize: "12px", fontFamily: "monospace", resize: "vertical" },
      }),

      // Logging toggle
      h("label", { style: { display: "flex", alignItems: "center", gap: "8px", fontSize: "13px", cursor: "pointer" } },
        h("input", { type: "checkbox", checked: logging, onChange: () => setLogging(!logging) }),
        "Enable I/O logging (stdout/stderr forwarded to K8s logging)"
      ),

      error ? h("div", { style: { color: "#ff8888", fontSize: "13px" } }, error) : null,
      h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
        h(Button, { variant: "secondary" as const, onClick: onClose, disabled: submitting }, "Cancel"),
        h(Button, { variant: "primary" as const, onClick: handleSubmit, disabled: submitting },
          submitting ? "Creating..." : "Create"
        )
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Agent Detail Dialog
// ---------------------------------------------------------------------------

function AgentDetailDialog({
  agentId,
  onClose,
  onRefresh,
}: {
  agentId: string | null;
  onClose: () => void;
  onRefresh: () => void;
}) {
  const [agent, setAgent] = useState<Agent | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    if (!agentId) return;
    setLoading(true);
    setError("");
    fetchAgent(agentId)
      .then((a) => setAgent(a))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [agentId]);

  const handleStop = useCallback(async () => {
    if (!agentId) return;
    setActionLoading(true);
    try {
      await stopAgent(agentId);
      const updated = await fetchAgent(agentId).catch(() => null);
      if (updated) setAgent(updated);
      onRefresh();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActionLoading(false);
    }
  }, [agentId, onRefresh]);

  const handleDelete = useCallback(async () => {
    if (!agentId) return;
    setActionLoading(true);
    try {
      await deleteAgent(agentId);
      onRefresh();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActionLoading(false);
      setConfirmDelete(false);
    }
  }, [agentId, onRefresh, onClose]);

  if (!agentId) return null;

  // Dark content style for modal
  const darkContent: Record<string, string> = {
    backgroundColor: "#1e1e1e",
    color: "#e0e0e0",
    borderRadius: "0 0 8px 8px",
    padding: "16px",
  };

  if (loading) {
    return h(Modal, { open: true, onClose, title: "Agent Detail", size: "md" as const },
      h("div", { style: { ...darkContent, textAlign: "center", padding: "32px" } }, h(Spinner, null))
    );
  }

  if (error && !agent) {
    return h(Modal, { open: true, onClose, title: "Agent Detail", size: "md" as const },
      h("div", { style: { ...darkContent, color: "#ff8888" } }, error)
    );
  }

  if (!agent) return null;

  const deleteModal = confirmDelete
    ? h(Modal, { open: true, onClose: () => setConfirmDelete(false), title: "Confirm Delete", size: "sm" as const },
        h("div", { style: { ...darkContent } },
          h("p", null, `Delete agent ${agent.id}? This will terminate the container.`),
          h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end", marginTop: "16px" } },
            h(Button, { variant: "secondary" as const, onClick: () => setConfirmDelete(false) }, "Cancel"),
            h(Button, { variant: "danger" as const, onClick: handleDelete, disabled: actionLoading },
              actionLoading ? "Deleting..." : "Delete"
            )
          )
        )
      )
    : null;

  return h("div", null,
    h(Modal, { open: true, onClose, title: `Agent: ${agent.id}`, size: "md" as const },
      h("div", { style: { ...darkContent, display: "flex", flexDirection: "column", gap: "12px" } },
        // Agent info grid
        h("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px" } },
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "STATUS"),
            statusTag(agent.status)
          ),
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "PROJECT"),
            h("div", null, agent.project || "—")
          ),
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "NODE"),
            h("div", null, agent.nodeId || "—")
          ),
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "OWNER"),
            h("div", null, agent.owner || "—")
          ),
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "EXPOSE"),
            h("div", null, agent.expose || "—")
          )
        ),
        // Action buttons
        h("div", { style: { display: "flex", gap: "8px", marginTop: "8px" }, "data-testid": "agent-detail-actions" },
          agent.status === "running" || agent.status === "connected"
            ? h(Button, { variant: "warning" as const, onClick: handleStop, disabled: actionLoading }, "Stop")
            : null,
          h(Button, { variant: "danger" as const, onClick: () => setConfirmDelete(true), disabled: actionLoading }, "Delete")
        ),
        error ? h("div", { style: { color: "#ff8888", fontSize: "13px" } }, error) : null
      )
    ),
    deleteModal
  );
}

// ---------------------------------------------------------------------------
// Agent Shell Terminal — stdin/stdout/stderr relay via API proxy
// ---------------------------------------------------------------------------

function AgentShellDialog({
  agentId,
  onClose,
}: {
  agentId: string | null;
  onClose: () => void;
}) {
  const [output, setOutput] = useState<string[]>([]);
  const [input, setInput] = useState("");
  const outputRef = useRef<HTMLDivElement | null>(null);

  // Subscribe to stdout via SSE
  useEffect(() => {
    if (!agentId) return;
    const token = localStorage.getItem("accessToken");
    const url = `/api/v1/amgr/agent/${agentId}/stdout?token=${encodeURIComponent(token || "")}`;
    const es = new EventSource(url);
    es.onmessage = (e: MessageEvent) => {
      setOutput((prev) => [...prev.slice(-500), e.data]); // keep last 500 lines
    };
    es.onerror = () => {
      setOutput((prev) => [...prev, "[connection lost]"]);
    };
    return () => es.close();
  }, [agentId]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight;
    }
  }, [output]);

  // Send stdin
  const handleSend = useCallback(async () => {
    if (!agentId || !input.trim()) return;
    try {
      await fetch(`/api/v1/amgr/agent/${agentId}/stdin`, {
        method: "POST",
        headers: { Authorization: `Bearer ${localStorage.getItem("accessToken")}`, "Content-Type": "application/octet-stream" },
        body: input + "\n",
      });
      setInput("");
    } catch {
      setOutput((prev) => [...prev, "[stdin send failed]"]);
    }
  }, [agentId, input]);

  if (!agentId) return null;

  return h(Modal, { open: true, onClose, title: `Shell: ${agentId}`, size: "lg" as const },
    h("div", { style: { backgroundColor: "#0d1117", color: "#c9d1d9", borderRadius: "0 0 8px 8px", display: "flex", flexDirection: "column", height: "400px" } },
      // Output area
      h("div", {
        ref: outputRef,
        style: { flex: 1, overflow: "auto", padding: "8px", fontFamily: "monospace", fontSize: "12px", whiteSpace: "pre-wrap", lineHeight: "1.4" },
      },
        output.length === 0
          ? h("span", { style: { color: "#666" } }, "Waiting for output...")
          : output.map((line, i) => h("div", { key: i }, line))
      ),
      // Input area
      h("div", { style: { display: "flex", borderTop: "1px solid #333", padding: "4px" } },
        h("span", { style: { padding: "4px 8px", color: "#22c55e", fontFamily: "monospace", fontSize: "12px" } }, "$"),
        h("input", {
          value: input,
          onInput: (e: Event) => setInput((e.target as HTMLInputElement).value),
          onKeyDown: (e: KeyboardEvent) => { if (e.key === "Enter") handleSend(); },
          style: { flex: 1, backgroundColor: "transparent", border: "none", outline: "none", color: "#c9d1d9", fontFamily: "monospace", fontSize: "12px", padding: "4px" },
          placeholder: "Type a command...",
        }),
        h(Button, { variant: "primary" as const, onClick: handleSend }, "Send")
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Main Agent Manager Component
// ---------------------------------------------------------------------------

// Stable filter reference for SSE subscription
const AGENT_METRICS_FILTER = ["agent.status"];

export function AgentManager({ principal }: { principal?: any } = {}) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [shellAgentId, setShellAgentId] = useState<string | null>(null);
  const isAdmin = principal?.roles?.includes("admin") || false;
  const [offset] = useState(0);
  const limit = 200; // fetch all agents (accordion view, not paginated)

  // Register applet menu bar
  useMenuBar("amgr", [
    { label: "Agent", items: [
      { label: "Create Agent", onClick: () => setShowCreate(true) },
      { label: "Refresh", onClick: () => loadAgents() },
    ]},
  ]);

  /** Load agents from the API */
  const loadAgents = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const page = await fetchAgents(offset, limit);
      setAgents(page.items || []);
    } catch (err: any) {
      setError(err.message || "Failed to load agents.");
    } finally {
      setLoading(false);
    }
  }, [offset]);

  useEffect(() => {
    loadAgents();
  }, [loadAgents]);

  // Live updates via SSE (when the API publishes agent status events)
  const handleAgentEvent = useCallback((evt: ServerEvent) => {
    // Refresh agent list on any status change
    loadAgents();
  }, [loadAgents]);

  const { connected } = useWebSocket(
    "/api/v1/events/amgr/status",
    AGENT_METRICS_FILTER,
    handleAgentEvent
  );

  // Group agents by nodeId for the accordion view
  const nodeGroups: Record<string, Agent[]> = {};
  for (const agent of agents) {
    const node = agent.nodeId || "unassigned";
    if (!nodeGroups[node]) nodeGroups[node] = [];
    nodeGroups[node].push(agent);
  }

  // Get available node IDs for the create dialog
  const availableNodes = Object.keys(nodeGroups);

  // Build accordion sections
  const accordionSections = Object.entries(nodeGroups).map(([nodeId, nodeAgents]) => ({
    id: nodeId,
    header: `${nodeId} (${nodeAgents.length} agent${nodeAgents.length !== 1 ? "s" : ""})`,
    content: h("div", { style: { backgroundColor: "#ffffff", borderRadius: "4px" } },
      h(DataGrid, {
        columns: [
          {
            key: "id",
            header: "Agent ID",
            width: 180,
            render: (value: string) =>
              h("a", {
                href: "#",
                style: { color: "#2563eb", textDecoration: "none", cursor: "pointer" },
                onClick: (e: Event) => { e.preventDefault(); setSelectedAgentId(value); },
              }, value),
          },
          { key: "project", header: "Project", width: 150 },
          {
            key: "status",
            header: "Status",
            width: 100,
            render: (value: string) => statusTag(value),
          },
          { key: "owner", header: "Owner", width: 120 },
          { key: "expose", header: "Expose", width: 150 },
          {
            key: "controls",
            header: "",
            width: 160,
            render: (_: string, row: any) => {
              const btns: any[] = [];
              // Shell button — always visible for running agents
              if (row._status === "running" || row._status === "connected") {
                btns.push(h(Button, {
                  variant: "secondary" as any,
                  onClick: (e: Event) => { e.stopPropagation(); setShellAgentId(row.id); },
                }, "Shell"));
                btns.push(h(Button, {
                  variant: "warning" as any,
                  onClick: (e: Event) => {
                    e.stopPropagation();
                    stopAgent(row.id).then(loadAgents).catch((err) => setError(err.message));
                  },
                }, "Stop"));
              }
              return btns.length > 0
                ? h("div", { style: { display: "flex", gap: "4px" } }, ...btns)
                : null;
            },
          },
        ],
        data: nodeAgents.map((a) => ({
          id: a.id,
          project: a.project || "—",
          status: a.status,
          _status: a.status,
          owner: a.owner || "—",
          expose: a.expose || "—",
          controls: "",
        })),
        striped: true,
      })
    ),
  }));

  return h(
    "div",
    {
      style: {
        display: "flex",
        flexDirection: "column",
        height: "100%",
        maxHeight: "460px",
        backgroundColor: "#1e1e1e",
        color: "#e0e0e0",
      },
      "data-testid": "agent-manager",
    },
    // Toolbar
    h(
      "div",
      {
        style: {
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "8px 12px",
          borderBottom: "1px solid #333",
          flexShrink: 0,
        },
      },
      h("div", { style: { display: "flex", alignItems: "center", gap: "8px" } },
        h("span", { style: { fontSize: "14px", fontWeight: "600" } }, "Agents"),
        h("span", {
          style: {
            width: "8px", height: "8px", borderRadius: "50%",
            backgroundColor: connected ? "#22c55e" : "#ef4444",
            display: "inline-block",
          },
          title: connected ? "Live updates active" : "Disconnected",
          "data-testid": "agent-ws-indicator",
        })
      ),
      h("div", { style: { display: "flex", gap: "8px", alignItems: "center" } },
        h(Button, { variant: "primary" as const, onClick: () => setShowCreate(true) }, "Create Agent"),
        h(Button, { variant: "secondary" as const, onClick: loadAgents }, "Refresh")
      )
    ),
    // Error banner
    error
      ? h("div", {
          style: { padding: "8px 12px", backgroundColor: "#3d1c1c", color: "#ff8888", fontSize: "13px" },
          "data-testid": "agent-error-banner",
        }, error)
      : null,
    // Main content
    h(
      "div",
      { style: { flex: 1, overflowY: "auto", overflowX: "auto", padding: "8px" } },
      loading
        ? h("div", { style: { padding: "32px", textAlign: "center" } }, h(Spinner, null))
        : agents.length === 0
          ? h("div", { style: { padding: "32px", textAlign: "center", color: "#aaa" } }, "No agents running.")
          : h(Accordion, {
              sections: accordionSections,
              allowMultiple: true,
              defaultExpanded: Object.keys(nodeGroups),
            })
    ),
    // Footer
    !loading
      ? h("div", {
          style: {
            padding: "8px 12px",
            borderTop: "1px solid #333",
            fontSize: "12px",
            color: "#aaa",
            flexShrink: 0,
          },
        }, `${agents.length} agent${agents.length !== 1 ? "s" : ""}`)
      : null,
    // Create dialog
    h(CreateAgentDialog, {
      open: showCreate,
      onClose: () => setShowCreate(false),
      onCreated: loadAgents,
      availableNodes,
      isAdmin,
    }),
    // Detail dialog
    h(AgentDetailDialog, {
      agentId: selectedAgentId,
      onClose: () => setSelectedAgentId(null),
      onRefresh: loadAgents,
    }),
    // Shell terminal
    h(AgentShellDialog, {
      agentId: shellAgentId,
      onClose: () => setShellAgentId(null),
    })
  );
}
