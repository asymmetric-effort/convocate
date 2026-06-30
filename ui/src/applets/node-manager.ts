/**
 * Node Manager Applet
 *
 * Displays a paginated data grid of K8s cluster nodes with CPU, memory, disk
 * stats. Supports provisioning new nodes, viewing node details, and
 * start/stop/delete operations.
 *
 * Data source:
 *   - Initial load: GET /api/v1/nmgr/node (paginated)
 *   - Live updates: WebSocket /api/v1/events/nmgr/status (node.metrics events)
 *
 * All mutations go through the API; the grid refreshes after each action.
 */

import { createElement, useState, useEffect, useCallback, useRef } from "@asymmetric-effort/specifyjs";
import { useWebSocket, ServerEvent } from "./use-websocket";
import { useMenuBar } from "./use-menu-bar";
import { hasRole, APPLET_ROLES } from "./use-rbac";
import {
  DataGrid,
  Button,
  Modal,
  TextField,
  Spinner,
  Tabs,
  Tag,
  Pagination,
} from "@asymmetric-effort/specifyjs/components";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Node {
  id: string;
  location: string;
  ip: string;
  status: "Ready" | "NotReady" | "SchedulingDisabled" | "Pending" | "Error";
  agents: number;
  loadAvg: { one: number; five: number; fifteen: number };
  memUsedGB: number;
  memTotalGB: number;
  swapUsedGB: number;
  swapTotalGB: number;
  diskUsedGB: number;
  diskTotalGB: number;
  uptimeSeconds: number;
  kubeletVersion: string;
  cpuCount: number;
  tags: string[];
}

interface NodeCondition {
  type: string;
  status: string;
  reason: string;
  message: string;
}

interface NodeTaint {
  key: string;
  value: string;
  effect: string;
}

interface NodeResources {
  cpuCores: number;
  memoryGB: number;
  ephemeralGB: number;
  pods: number;
}

interface NodeDetail extends Node {
  agentList: any[];
  notes: Note[];
  conditions: NodeCondition[];
  labels: Record<string, string>;
  taints: NodeTaint[];
  capacity: NodeResources;
  allocatable: NodeResources;
}

interface Note {
  author: string;
  createdAt: string;
  text: string;
}

interface PageResponse {
  items: Node[];
  offset: number;
  limit: number;
  total: number;
}

// ---------------------------------------------------------------------------
// API helpers — all requests use the JWT from localStorage
// ---------------------------------------------------------------------------

/** Build standard auth headers for API requests */
function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

/** Fetch a page of nodes from the API */
async function fetchNodes(offset: number, limit: number): Promise<PageResponse> {
  const res = await fetch(
    `/api/v1/nmgr/node?offset=${offset}&limit=${limit}`,
    { headers: authHeaders() }
  );
  if (!res.ok) throw new Error(`Failed to fetch nodes: ${res.status}`);
  return res.json();
}

/** Fetch detailed info for a single node */
async function fetchNodeDetail(nodeId: string): Promise<NodeDetail> {
  const res = await fetch(`/api/v1/nmgr/node/${nodeId}`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Failed to fetch node detail: ${res.status}`);
  return res.json();
}

/** Provision a new node via SSH */
async function provisionNode(data: {
  name?: string;
  host: string;
  user: string;
  password?: string;
  location?: string;
  tags?: string[];
}): Promise<Node> {
  const res = await fetch("/api/v1/nmgr/node", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(`Provision failed: ${res.status}`);
  return res.json();
}

/** Start a stopped node */
async function startNode(nodeId: string): Promise<void> {
  const res = await fetch(`/api/v1/nmgr/node/${nodeId}/start`, {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Start failed: ${res.status}`);
}

/** Stop (drain) a running node */
async function stopNode(nodeId: string): Promise<void> {
  const res = await fetch(`/api/v1/nmgr/node/${nodeId}/stop`, {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Stop failed: ${res.status}`);
}

/** Delete (decommission) a node */
async function deleteNode(nodeId: string): Promise<void> {
  const res = await fetch(`/api/v1/nmgr/node/${nodeId}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Delete failed: ${res.status}`);
}

/** Update a node's location or tags via PATCH */
async function updateNode(nodeId: string, data: { location?: string; tags?: string[] }): Promise<Node> {
  const res = await fetch(`/api/v1/nmgr/node/${nodeId}`, {
    method: "PATCH",
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(`Update failed: ${res.status}`);
  return res.json();
}

/** Add a note to a node */
async function addNote(nodeId: string, text: string): Promise<Note> {
  const res = await fetch(`/api/v1/nmgr/node/${nodeId}/note`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ text }),
  });
  if (!res.ok) throw new Error(`Add note failed: ${res.status}`);
  return res.json();
}

// ---------------------------------------------------------------------------
// Status badge helper
// ---------------------------------------------------------------------------

/** Render a node status as a color-coded Tag using K8s-native terms */
function statusBadge(status: string) {
  switch (status) {
    case "Ready":
      return h(Tag, { label: "Ready", color: "green", variant: "solid" as const, size: "sm" as const });
    case "SchedulingDisabled":
      return h(Tag, { label: "SchedulingDisabled", color: "#b45309", variant: "solid" as const, size: "sm" as const });
    case "NotReady":
      return h(Tag, { label: "NotReady", color: "#b91c1c", variant: "solid" as const, size: "sm" as const });
    case "Pending":
      return h(Tag, { label: "Pending", color: "#2563eb", variant: "solid" as const, size: "sm" as const });
    case "Error":
      return h(Tag, { label: "Error", color: "#b91c1c", variant: "solid" as const, size: "sm" as const });
    default:
      return h(Tag, { label: status, color: "gray", variant: "subtle" as const, size: "sm" as const });
  }
}

// ---------------------------------------------------------------------------
// Provision Node Dialog
// ---------------------------------------------------------------------------

function ProvisionDialog({
  open,
  onClose,
  onProvisioned,
  existingNodes,
}: {
  open: boolean;
  onClose: () => void;
  onProvisioned: () => void;
  existingNodes: Node[];
}) {
  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [location, setLocation] = useState("");
  const [tags, setTags] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    // --- Client-side validation ---

    // Required fields
    if (!host.trim()) {
      setError("Host is required.");
      return;
    }
    if (!user.trim()) {
      setError("SSH Username is required.");
      return;
    }

    // Node name format: lowercase alphanumeric + hyphens, no leading/trailing hyphens
    if (name.trim()) {
      if (!/^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(name.trim()) && name.trim().length > 1) {
        setError("Node name must contain only lowercase letters, digits, and hyphens, and must not start or end with a hyphen.");
        return;
      }
      if (name.trim().length === 1 && !/^[a-z0-9]$/.test(name.trim())) {
        setError("Node name must contain only lowercase letters, digits, and hyphens.");
        return;
      }
      if (name.trim().length > 63) {
        setError("Node name must be 63 characters or fewer.");
        return;
      }
      // Check uniqueness against existing nodes
      if (existingNodes.some((n) => n.id === name.trim())) {
        setError("A node with this name already exists.");
        return;
      }
    }

    // Host format: basic IP or FQDN validation
    const hostVal = host.trim();
    const ipPattern = /^(\d{1,3}\.){3}\d{1,3}$/;
    const fqdnPattern = /^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$/;
    if (!ipPattern.test(hostVal) && !fqdnPattern.test(hostVal)) {
      setError("Host must be a valid IPv4 address or FQDN.");
      return;
    }
    if (ipPattern.test(hostVal)) {
      const octets = hostVal.split(".").map(Number);
      if (octets.some((o) => o > 255)) {
        setError("Invalid IP address: octets must be 0-255.");
        return;
      }
    }

    // Check host uniqueness
    if (existingNodes.some((n) => n.ip === hostVal)) {
      setError("A node with this IP address already exists.");
      return;
    }

    // SSH username format
    if (!/^[a-zA-Z_][a-zA-Z0-9_.-]*$/.test(user.trim())) {
      setError("SSH Username must be a valid Linux username.");
      return;
    }

    setError("");
    setSubmitting(true);
    try {
      await provisionNode({
        name: name || undefined,
        host,
        user,
        password: password || undefined,
        location: location || undefined,
        tags: tags ? tags.split(",").map((t: string) => t.trim()) : undefined,
      });
      // Reset form and close dialog
      setName("");
      setHost("");
      setUser("");
      setPassword("");
      setLocation("");
      setTags("");
      onProvisioned();
      onClose();
    } catch (err: any) {
      setError(err.message || "Provision failed.");
    } finally {
      setSubmitting(false);
    }
  }, [name, host, user, password, location, tags, existingNodes, onProvisioned, onClose]);

  if (!open) return null;

  return h(
    Modal,
    {
      open: true,
      onClose,
      title: "Provision Node",
      size: "md" as const,
    },
    h(
      "div",
      { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, {
        placeholder: "Node Name (optional, auto-generated if empty)",
        value: name,
        onChange: (v: string) => setName(v),
      }),
      h(TextField, {
        placeholder: "Host (IP or FQDN)",
        value: host,
        onChange: (v: string) => setHost(v),
      }),
      h(TextField, {
        placeholder: "SSH Username",
        value: user,
        onChange: (v: string) => setUser(v),
        "data-testid": "provision-user",
      }),
      h(TextField, {
        placeholder: "Password (optional)",
        type: "password",
        value: password,
        onChange: (v: string) => setPassword(v),
        "data-testid": "provision-password",
      }),
      h(TextField, {
        placeholder: "Location (optional)",
        value: location,
        onChange: (v: string) => setLocation(v),
        "data-testid": "provision-location",
      }),
      h(TextField, {
        placeholder: "Tags (comma-separated, optional)",
        value: tags,
        onChange: (v: string) => setTags(v),
        "data-testid": "provision-tags",
      }),
      error
        ? h("div", { style: { color: "#e55", fontSize: "13px" } }, error)
        : null,
      h(
        "div",
        { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
        h(Button, { variant: "secondary" as const, onClick: onClose, disabled: submitting }, "Cancel"),
        h(
          Button,
          {
            variant: "primary" as const,
            onClick: handleSubmit,
            disabled: submitting,
          },
          submitting ? "Provisioning..." : "Provision"
        )
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Node Detail Dialog
// ---------------------------------------------------------------------------

function NodeDetailDialog({
  nodeId,
  onClose,
  onRefresh,
  canStopOrDelete,
}: {
  nodeId: string | null;
  onClose: () => void;
  onRefresh: () => void;
  canStopOrDelete: boolean;
}) {
  const [detail, setDetail] = useState<NodeDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [noteText, setNoteText] = useState("");
  const [addingNote, setAddingNote] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  // Fetch node detail when nodeId changes
  useEffect(() => {
    if (!nodeId) return;
    setLoading(true);
    setError("");
    fetchNodeDetail(nodeId)
      .then((d) => setDetail(d))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [nodeId]);

  /** Handle start/stop actions */
  const handleAction = useCallback(
    async (action: "start" | "stop") => {
      if (!nodeId) return;
      setActionLoading(true);
      try {
        if (action === "start") await startNode(nodeId);
        else await stopNode(nodeId);
        // Refresh detail and parent grid
        const updated = await fetchNodeDetail(nodeId);
        setDetail(updated);
        onRefresh();
      } catch (err: any) {
        setError(err.message);
      } finally {
        setActionLoading(false);
      }
    },
    [nodeId, onRefresh]
  );

  /** Handle delete with confirmation */
  const handleDelete = useCallback(async () => {
    if (!nodeId) return;
    setActionLoading(true);
    try {
      await deleteNode(nodeId);
      onRefresh();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActionLoading(false);
      setConfirmDelete(false);
    }
  }, [nodeId, onRefresh, onClose]);

  /** Handle adding a note */
  const handleAddNote = useCallback(async () => {
    if (!nodeId || !noteText.trim()) return;
    setAddingNote(true);
    try {
      await addNote(nodeId, noteText.trim());
      setNoteText("");
      // Refresh detail to show new note
      const updated = await fetchNodeDetail(nodeId);
      setDetail(updated);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setAddingNote(false);
    }
  }, [nodeId, noteText]);

  if (!nodeId) return null;

  // Dark background style for modal content (Modal has white bg internally)
  const darkContent: Record<string, string> = {
    backgroundColor: "#1e1e1e",
    color: "#e0e0e0",
    borderRadius: "0 0 8px 8px",
    minHeight: "200px",
  };

  // Loading state
  if (loading) {
    return h(
      Modal,
      { open: true, onClose, title: "Node Detail", size: "lg" as const },
      h("div", { style: { ...darkContent, padding: "32px", textAlign: "center" } }, h(Spinner, null))
    );
  }

  // Error state
  if (error && !detail) {
    return h(
      Modal,
      { open: true, onClose, title: "Node Detail", size: "lg" as const },
      h("div", { style: { ...darkContent, padding: "16px", color: "#ff8888" } }, error)
    );
  }

  if (!detail) return null;

  // Helper for labeled field
  const field = (label: string, value: string) =>
    h("div", null,
      h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, label),
      h("div", null, value || "—")
    );

  const fmtGB = (used: number, total: number) =>
    used >= 0 && total >= 0 ? `${used.toFixed(1)} / ${total.toFixed(1)} GB` : "—";

  // Build tab content — all tabs use dark background for consistent contrast
  const overviewTab = h(
    "div",
    { style: { ...darkContent, padding: "16px", display: "flex", flexDirection: "column", gap: "12px" } },
    // Resource summary
    h("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "12px" } },
      h("div", null,
        h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "STATUS"),
        statusBadge(detail.status)
      ),
      field("IP ADDRESS", detail.ip),
      field("LOCATION", detail.location || "unspecified"),
      field("AGENTS", String(detail.agents ?? 0)),
      field("K8S VERSION", detail.kubeletVersion || "—"),
      field("CPU COUNT", detail.cpuCount ? String(detail.cpuCount) : "—"),
      field("UPTIME", detail.uptimeSeconds > 0 ? `${Math.floor(detail.uptimeSeconds / 86400)}d ${Math.floor((detail.uptimeSeconds % 86400) / 3600)}h` : "—"),
      field("LOAD AVG (1/5/15m)", detail.loadAvg && detail.loadAvg.one >= 0
        ? `${detail.loadAvg.one.toFixed(2)} / ${detail.loadAvg.five.toFixed(2)} / ${detail.loadAvg.fifteen.toFixed(2)}`
        : "—"),
      field("MEMORY", fmtGB(detail.memUsedGB, detail.memTotalGB)),
      field("SWAP", fmtGB(detail.swapUsedGB, detail.swapTotalGB)),
      field("DISK", fmtGB(detail.diskUsedGB, detail.diskTotalGB)),
    ),
    // Capacity vs Allocatable
    detail.capacity ? h("div", null,
      h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px", marginTop: "8px" } }, "CAPACITY / ALLOCATABLE"),
      h("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr 1fr 1fr", gap: "8px", fontSize: "12px" } },
        field("CPU", `${detail.capacity.cpuCores} / ${detail.allocatable.cpuCores} cores`),
        field("Memory", `${detail.capacity.memoryGB.toFixed(1)} / ${detail.allocatable.memoryGB.toFixed(1)} GB`),
        field("Ephemeral", `${detail.capacity.ephemeralGB.toFixed(1)} / ${detail.allocatable.ephemeralGB.toFixed(1)} GB`),
        field("Pods", `${detail.capacity.pods} / ${detail.allocatable.pods}`)
      )
    ) : null,
    // Tags
    detail.tags && detail.tags.length > 0
      ? h("div", null,
          h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "TAGS"),
          h("div", { style: { display: "flex", gap: "4px", flexWrap: "wrap" } },
            ...detail.tags.map((t: string) => h(Tag, { label: t, color: "#4b5563", variant: "solid" as const, size: "sm" as const }))
          )
        )
      : null,
    // Action buttons — uses K8s terminology:
    //   Ready           → Cordon + Delete
    //   SchedulingDisabled → Uncordon + Delete
    //   Pending / NotReady → Uncordon (disabled) + Delete
    //   Error            → Delete only (cleanup)
    h(
      "div",
      {
        style: { display: "flex", gap: "8px", marginTop: "8px" },
        "data-testid": "node-detail-actions",
      },
      detail.status === "Error"
        ? null
        : detail.status === "Ready"
          ? h(Button, {
              variant: "warning" as const,
              onClick: () => handleAction("stop"),
              disabled: actionLoading || !canStopOrDelete,
            }, "Cordon")
          : h(Button, {
              variant: "primary" as const,
              onClick: () => handleAction("start"),
              disabled: actionLoading || detail.status === "Pending" || detail.status === "NotReady",
            }, "Uncordon"),
      h(Button, {
        variant: "danger" as const,
        onClick: () => setConfirmDelete(true),
        disabled: actionLoading || (!canStopOrDelete && detail.status !== "Error"),
        "data-testid": "node-delete-btn",
      }, "Delete")
    ),
    error ? h("div", { style: { color: "#e55", fontSize: "13px" } }, error) : null
  );

  // Conditions tab — K8s node conditions
  const conditionsTab = h(
    "div",
    { style: { ...darkContent, padding: "8px" } },
    detail.conditions && detail.conditions.length > 0
      ? h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
          h(DataGrid, {
            columns: [
              { key: "type", header: "Type", width: 150 },
              { key: "status", header: "Status", width: 80, render: (v: string) =>
                h(Tag, { label: v, color: v === "True" ? "green" : v === "False" ? "#b91c1c" : "gray", variant: "solid" as const, size: "sm" as const })
              },
              { key: "reason", header: "Reason", width: 150 },
              { key: "message", header: "Message", width: 250 },
            ],
            data: detail.conditions.map((c: NodeCondition) => ({ id: c.type, type: c.type, status: c.status, reason: c.reason, message: c.message })),
          })
        )
      : h("div", { style: { color: "#aaa", fontSize: "13px", padding: "16px" } }, "No conditions available.")
  );

  // Labels & Taints tab
  const labelsTab = h(
    "div",
    { style: { ...darkContent, padding: "16px", display: "flex", flexDirection: "column", gap: "16px" } },
    // Labels
    h("div", null,
      h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "8px" } }, "LABELS"),
      detail.labels && Object.keys(detail.labels).length > 0
        ? h("div", { style: { display: "flex", flexDirection: "column", gap: "4px" } },
            ...Object.entries(detail.labels).map(([k, v]) =>
              h("div", { key: k, style: { display: "flex", gap: "8px", fontSize: "12px" } },
                h("span", { style: { color: "#93c5fd", fontFamily: "monospace" } }, k),
                h("span", { style: { color: "#aaa" } }, "="),
                h("span", { style: { color: "#e0e0e0", fontFamily: "monospace" } }, v as string)
              )
            )
          )
        : h("div", { style: { color: "#aaa", fontSize: "13px" } }, "No labels.")
    ),
    // Taints
    h("div", null,
      h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "8px" } }, "TAINTS"),
      detail.taints && detail.taints.length > 0
        ? h("div", { style: { display: "flex", flexDirection: "column", gap: "4px" } },
            ...detail.taints.map((t: NodeTaint, i: number) =>
              h("div", { key: i, style: { fontSize: "12px" } },
                h("span", { style: { color: "#e0e0e0", fontFamily: "monospace" } }, `${t.key}=${t.value}`),
                h("span", { style: { color: "#aaa", marginLeft: "8px" } }, `(${t.effect})`)
              )
            )
          )
        : h("div", { style: { color: "#aaa", fontSize: "13px" } }, "No taints.")
    )
  );

  // Notes tab
  const notesTab = h(
    "div",
    { style: { ...darkContent, padding: "16px", display: "flex", flexDirection: "column", gap: "12px" } },
    // Existing notes
    detail.notes && detail.notes.length > 0
      ? h("div", { style: { display: "flex", flexDirection: "column", gap: "8px" } },
          ...detail.notes.map((note: Note, i: number) =>
            h("div", {
              key: i,
              style: {
                padding: "8px 12px",
                backgroundColor: "#2a2a2a",
                borderRadius: "4px",
                fontSize: "13px",
              },
            },
              h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } },
                `${note.author} — ${new Date(note.createdAt).toLocaleString()}`
              ),
              h("div", null, note.text)
            )
          )
        )
      : h("div", { style: { color: "#aaa", fontSize: "13px" } }, "No notes yet."),
    // Add note form
    h("div", { style: { display: "flex", gap: "8px" } },
      h(TextField, {
        placeholder: "Add a note...",
        value: noteText,
        onChange: (v: string) => setNoteText(v),
        "data-testid": "note-input",
      }),
      h(Button, {
        variant: "primary" as const,
        onClick: handleAddNote,
        disabled: addingNote || !noteText.trim(),
        "data-testid": "note-submit",
      }, addingNote ? "Adding..." : "Add")
    )
  );

  // Agents tab
  const agentsTab = h(
    "div",
    { style: { ...darkContent, padding: "16px" } },
    detail.agentList && detail.agentList.length > 0
      ? h(DataGrid, {
          columns: [
            { key: "id", header: "Agent ID", width: 200 },
            { key: "status", header: "Status", width: 120 },
          ],
          rows: detail.agentList.map((a: any) => ({
            id: a.id || a.agentId || "—",
            status: a.status || "unknown",
          })),
        })
      : h("div", { style: { color: "#aaa", fontSize: "13px" } }, "No agents on this node.")
  );

  // Delete confirmation overlay
  const deleteConfirmModal = confirmDelete
    ? h(
        Modal,
        {
          open: true,
          onClose: () => setConfirmDelete(false),
          title: "Confirm Delete",
          size: "sm" as const,
        },
        h(
          "div",
          { style: { padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
          h("p", null, `Are you sure you want to delete node ${detail.id}? This will drain all workloads and remove the node from the cluster.`),
          h(
            "div",
            { style: { display: "flex", gap: "8px", justifyContent: "flex-end", marginTop: "16px" } },
            h(Button, { variant: "secondary" as const, onClick: () => setConfirmDelete(false) }, "Cancel"),
            h(Button, {
              variant: "danger" as const,
              onClick: handleDelete,
              disabled: actionLoading,
              "data-testid": "confirm-delete-btn",
            }, actionLoading ? "Deleting..." : "Delete")
          )
        )
      )
    : null;

  return h(
    "div",
    null,
    h(
      Modal,
      {
        open: true,
        onClose,
        title: `Node: ${detail.id}`,
        size: "lg" as const,
      },
      h(Tabs, {
        tabs: [
          { id: "overview", label: "Overview", content: overviewTab },
          { id: "conditions", label: "Conditions", content: conditionsTab },
          { id: "labels", label: "Labels & Taints", content: labelsTab },
          { id: "notes", label: "Notes", content: notesTab },
          { id: "agents", label: "Agents", content: agentsTab },
        ],
      })
    ),
    deleteConfirmModal
  );
}

// ---------------------------------------------------------------------------
// Main Node Manager Component
// ---------------------------------------------------------------------------

// Stable reference for the WebSocket filter — avoids re-creating the
// EventSource on every render.
const METRICS_FILTER = ["node.metrics"];

export function NodeManager({ principal }: { principal?: any } = {}) {
  const roles = APPLET_ROLES.nmgr;
  const canCreate = hasRole(principal, roles.create);
  const canUpdate = hasRole(principal, roles.update);
  const canDelete = hasRole(principal, roles.delete);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [offset, setOffset] = useState(0);
  const [total, setTotal] = useState(0);
  const [showProvision, setShowProvision] = useState(false);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const limit = 25;

  // Register applet menu bar
  useMenuBar("nmgr", [
    { label: "Node", items: [
      { label: "Provision Node", onClick: () => setShowProvision(true) },
      { label: "Refresh", onClick: () => loadNodes() },
    ]},
  ]);

  // ---------------------------------------------------------------------------
  // Live metrics via WebSocket — merges incoming node.metrics events into
  // the grid state without requiring a manual refresh.
  // ---------------------------------------------------------------------------
  const nodesRef = useRef<Node[]>([]);
  nodesRef.current = nodes;

  const handleMetricsEvent = useCallback((evt: ServerEvent) => {
    if (evt.type !== "node.metrics" || !Array.isArray(evt.payload)) return;
    const incoming = evt.payload as Node[];
    // Merge incoming metrics into the existing node list by ID
    setNodes((prev) => {
      const updated = [...prev];
      for (const inNode of incoming) {
        const idx = updated.findIndex((n) => n.id === inNode.id);
        if (idx >= 0) {
          updated[idx] = inNode;
        }
      }
      // Update total if new nodes appeared
      if (incoming.length > updated.length) {
        setTotal(incoming.length);
      }
      return updated;
    });
  }, []);

  const { connected } = useWebSocket(
    "/api/v1/events/nmgr/status",
    METRICS_FILTER,
    handleMetricsEvent
  );

  /** Load nodes from the API */
  const loadNodes = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const page = await fetchNodes(offset, limit);
      setNodes(page.items || []);
      setTotal(page.total || 0);
    } catch (err: any) {
      setError(err.message || "Failed to load nodes.");
    } finally {
      setLoading(false);
    }
  }, [offset]);

  // Fetch nodes on mount and when offset changes
  useEffect(() => {
    loadNodes();
  }, [loadNodes]);

  /** Handle inline start/stop from the grid controls column */
  const handleQuickAction = useCallback(
    async (nodeId: string, action: "start" | "stop") => {
      try {
        if (action === "start") await startNode(nodeId);
        else await stopNode(nodeId);
        await loadNodes();
      } catch (err: any) {
        setError(err.message);
      }
    },
    [loadNodes]
  );

  // Build DataGrid columns — using `render` for interactive cells
  const columns = [
    {
      key: "id",
      header: "Name",
      width: 140,
      render: (value: string, row: any) =>
        h("a", {
          href: "#",
          style: { color: "#2563eb", textDecoration: "none", cursor: "pointer" },
          onClick: (e: Event) => { e.preventDefault(); setSelectedNodeId(value); },
          "data-testid": `node-link-${value}`,
        }, value),
    },
    {
      key: "location",
      header: "Location",
      width: 120,
      render: (value: string, row: any) =>
        h("span", {
          style: { cursor: "pointer", borderBottom: "1px dashed #666" },
          title: "Double-click to edit",
          onDblClick: () => {
            const newLoc = prompt("Edit location:", value === "unspecified" ? "" : value);
            if (newLoc !== null) {
              updateNode(row.id, { location: newLoc || "unspecified" })
                .then(loadNodes)
                .catch((err: any) => setError(err.message));
            }
          },
        }, value),
    },
    { key: "ip", header: "IP", width: 130 },
    {
      key: "status",
      header: "Status",
      width: 110,
      render: (value: string) => statusBadge(value),
    },
    { key: "agents", header: "Agents", width: 70 },
    { key: "cpu", header: "Load (1/5/15m)", width: 140 },
    { key: "memory", header: "Memory", width: 120 },
    { key: "disk", header: "Disk", width: 120 },
    {
      key: "controls",
      header: "",
      width: 80,
      render: (_: string, row: any) => {
        const st = row._status;
        // Error → Delete button for cleanup
        if (st === "Error") {
          return h(Button, {
            variant: "danger" as any,
            onClick: (e: Event) => {
              e.stopPropagation();
              deleteNode(row.id).then(loadNodes).catch((err: any) => setError(err.message));
            },
          }, "Delete");
        }
        // Ready → Cordon (disabled if fewer than 4 Ready nodes)
        if (st === "Ready") {
          return h(Button, {
            variant: "warning" as any,
            onClick: (e: Event) => { e.stopPropagation(); handleQuickAction(row.id, "stop"); },
            disabled: !canStopOrDelete,
          }, "Cordon");
        }
        // SchedulingDisabled → Uncordon
        if (st === "SchedulingDisabled") {
          return h(Button, {
            variant: "primary" as any,
            onClick: (e: Event) => { e.stopPropagation(); handleQuickAction(row.id, "start"); },
          }, "Uncordon");
        }
        // Pending / NotReady → Uncordon (disabled)
        return h(Button, {
          variant: "primary" as any,
          disabled: true,
        }, "Uncordon");
      },
    },
  ];

  // Count Ready nodes — used to disable Stop/Delete when fewer than 4 Ready
  const readyCount = nodes.filter((n) => n.status === "Ready").length;
  const canStopOrDelete = readyCount >= 4;

  // Transform nodes into DataGrid data array.
  // The API uses -1 as a sentinel for "no data" when metrics are unavailable.
  // We display "—" instead of showing impossible negative values.
  const data = nodes.map((node) => {
    const hasLoad = node.loadAvg && node.loadAvg.one >= 0;
    const hasMemUsed = node.memUsedGB != null && node.memUsedGB >= 0;
    const hasMemTotal = node.memTotalGB != null && node.memTotalGB >= 0;
    const hasDiskUsed = node.diskUsedGB != null && node.diskUsedGB >= 0;
    const hasDiskTotal = node.diskTotalGB != null && node.diskTotalGB >= 0;

    return {
      id: node.id,
      location: node.location || "unspecified",
      ip: node.ip || "—",
      status: node.status,
      _status: node.status, // raw status for controls render
      agents: String(node.agents ?? 0),
      cpu: hasLoad
        ? `${node.loadAvg.one.toFixed(2)} / ${node.loadAvg.five.toFixed(2)} / ${node.loadAvg.fifteen.toFixed(2)}`
        : "—",
      memory: hasMemUsed && hasMemTotal
        ? `${node.memUsedGB.toFixed(1)} / ${node.memTotalGB.toFixed(1)} GB`
        : "—",
      disk: hasDiskUsed && hasDiskTotal
        ? `${node.diskUsedGB.toFixed(1)} / ${node.diskTotalGB.toFixed(1)} GB`
        : "—",
      controls: "", // rendered by column render function
    };
  });

  // Current page for pagination (1-based)
  const currentPage = Math.floor(offset / limit) + 1;

  // Max grid height: 10 rows × 35px + header row (35px) = 385px
  // Total max height: toolbar (40px) + grid (385px) + footer (35px) = 460px
  const maxGridHeight = Math.min(data.length, 10) * 35 + 35;

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
      "data-testid": "node-manager",
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
        h("span", { style: { fontSize: "14px", fontWeight: "600" } }, "Nodes"),
        // WebSocket connection indicator
        h("span", {
          style: {
            width: "8px",
            height: "8px",
            borderRadius: "50%",
            backgroundColor: connected ? "#22c55e" : "#ef4444",
            display: "inline-block",
          },
          title: connected ? "Live updates active" : "Disconnected",
          "data-testid": "ws-indicator",
        })
      ),
      h(
        "div",
        { style: { display: "flex", gap: "8px", alignItems: "center" } },
        canCreate ? h(
          Button,
          {
            variant: "primary" as const,
            onClick: () => setShowProvision(true),
            "data-testid": "provision-btn",
          },
          "Provision Node"
        ) : null,
        h(
          Button,
          {
            variant: "secondary" as const,
            onClick: loadNodes,
            "data-testid": "refresh-btn",
          },
          "Refresh"
        )
      )
    ),
    // Error banner
    error
      ? h(
          "div",
          {
            style: {
              padding: "8px 12px",
              backgroundColor: "#3d1c1c",
              color: "#ff8888",
              fontSize: "13px",
            },
            "data-testid": "error-banner",
          },
          error
        )
      : null,
    // Main content: data grid — scrolls when content exceeds 10 rows
    h(
      "div",
      { style: { flex: 1, overflowY: "auto", overflowX: "auto", padding: "0", maxHeight: `${maxGridHeight}px` } },
      loading
        ? h("div", { style: { padding: "32px", textAlign: "center" } }, h(Spinner, null))
        : h("div", {
            style: { backgroundColor: "#ffffff", borderRadius: "4px" },
          }, h(DataGrid, {
            columns,
            data,
            striped: true,
            stickyHeader: true,
          }))
    ),
    // Pagination footer
    !loading && total > 0
      ? h(
          "div",
          {
            style: {
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              padding: "8px 12px",
              borderTop: "1px solid #333",
              fontSize: "12px",
              color: "#aaa",
              flexShrink: 0,
            },
          },
          h("span", null, `${total} node${total !== 1 ? "s" : ""}`),
          total > limit
            ? h(Pagination, {
                total,
                pageSize: limit,
                currentPage,
                onChange: (page: number) => setOffset((page - 1) * limit),
              })
            : null
        )
      : null,
    // Provision dialog
    h(ProvisionDialog, {
      open: showProvision,
      onClose: () => setShowProvision(false),
      onProvisioned: loadNodes,
      existingNodes: nodes,
    }),
    // Node detail dialog
    h(NodeDetailDialog, {
      nodeId: selectedNodeId,
      onClose: () => setSelectedNodeId(null),
      onRefresh: loadNodes,
      canStopOrDelete,
    })
  );
}
