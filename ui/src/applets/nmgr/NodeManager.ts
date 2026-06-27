import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import { DataGrid, Modal, Toolbar } from "@asymmetric-effort/specifyjs/components";
import type { DataGridColumn, ToolbarItem } from "@asymmetric-effort/specifyjs/components";
import type { Node, Page, NodeDetail, Note } from "../../types/api";
import { apiGet, apiPost, apiDelete } from "../../lib/api";
import { hasRole } from "../../lib/auth";

const h = createElement;

export function NodeManager() {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [selectedNode, setSelectedNode] = useState<NodeDetail | null>(null);
  const [showProvision, setShowProvision] = useState(false);
  const [loading, setLoading] = useState(true);

  const limit = 25;
  const canUpdate = hasRole("node-update");
  const canCreate = hasRole("node-create");
  const canDelete = hasRole("node-delete");

  useEffect(() => { loadNodes(); }, [offset]);

  async function loadNodes() {
    setLoading(true);
    try {
      const page = await apiGet<Page<Node>>(`/nmgr/node?offset=${offset}&limit=${limit}`);
      setNodes(page.items);
      setTotal(page.total);
    } catch (e) {
      console.error("Failed to load nodes", e);
    }
    setLoading(false);
  }

  async function handleNodeClick(nodeId: string) {
    try {
      const detail = await apiGet<NodeDetail>(`/nmgr/node/${nodeId}`);
      setSelectedNode(detail);
    } catch (e) {
      console.error("Failed to load node detail", e);
    }
  }

  async function handleStart(nodeId: string) {
    await apiPost(`/nmgr/node/${nodeId}/start`);
    loadNodes();
  }

  async function handleStop(nodeId: string) {
    await apiPost(`/nmgr/node/${nodeId}/stop`);
    loadNodes();
  }

  async function handleDelete(nodeId: string) {
    if (!confirm("Delete this node? This will drain, uninstall, and power off the host.")) return;
    await apiDelete(`/nmgr/node/${nodeId}`);
    setSelectedNode(null);
    loadNodes();
  }

  function statusDot(status: string) {
    const colors: Record<string, string> = {
      online: "#4caf50",
      draining: "#ff9800",
      offline: "#f44336",
    };
    return h("span", {
      className: "status-dot",
      style: { backgroundColor: colors[status] || "#999" },
    });
  }

  if (loading && nodes.length === 0) {
    return h("div", { className: "applet-loading" }, "Loading nodes...");
  }

  return h("div", { className: "nmgr" },
    // Toolbar
    h("div", { className: "applet-toolbar" },
      canCreate
        ? h("button", {
            className: "btn btn-primary",
            onClick: () => setShowProvision(true),
          }, "Provision Node")
        : null,
      h("span", { className: "applet-count" }, `${total} node${total !== 1 ? "s" : ""}`)
    ),

    // Node grid
    h("div", { className: "grid-list" },
      h("div", { className: "grid-header" },
        h("span", null, "Name"),
        h("span", null, "Location"),
        h("span", null, "IP"),
        h("span", null, "Status"),
        h("span", null, "Agents"),
        h("span", null, "Load (1/5/15)"),
        h("span", null, "Memory"),
        h("span", null, "Disk"),
        h("span", null, "Controls")
      ),
      nodes.map((node: Node, i: number) =>
        h("div", {
          key: node.id,
          className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`,
          onDblClick: () => handleNodeClick(node.id),
        },
          h("span", { className: "cell-id" }, node.id),
          h("span", null, node.location),
          h("span", null, node.ip),
          h("span", null, statusDot(node.status), " ", node.status),
          h("span", null, String(node.agents)),
          h("span", null,
            `${node.loadAvg.one.toFixed(1)} / ${node.loadAvg.five.toFixed(1)} / ${node.loadAvg.fifteen.toFixed(1)}`
          ),
          h("span", null,
            `${node.memUsedGB.toFixed(1)} / ${node.memTotalGB.toFixed(0)} GB`
          ),
          h("span", null,
            `${node.diskUsedGB.toFixed(0)} / ${node.diskTotalGB.toFixed(0)} GB`
          ),
          h("span", { className: "cell-controls" },
            canUpdate && node.status === "online"
              ? h("button", {
                  className: "btn btn-sm btn-warning",
                  onClick: (e: Event) => { e.stopPropagation(); handleStop(node.id); },
                }, "Stop")
              : null,
            canUpdate && node.status === "offline"
              ? h("button", {
                  className: "btn btn-sm btn-success",
                  onClick: (e: Event) => { e.stopPropagation(); handleStart(node.id); },
                }, "Start")
              : null
          )
        )
      )
    ),

    // Pagination
    total > limit
      ? h("div", { className: "pagination" },
          h("button", {
            disabled: offset === 0,
            onClick: () => setOffset(Math.max(0, offset - limit)),
          }, "Previous"),
          h("span", null, `${offset + 1}–${Math.min(offset + limit, total)} of ${total}`),
          h("button", {
            disabled: offset + limit >= total,
            onClick: () => setOffset(offset + limit),
          }, "Next")
        )
      : null,

    // Node detail modal
    selectedNode
      ? h("div", { className: "modal-overlay", onClick: () => setSelectedNode(null) },
          h("div", { className: "modal", onClick: (e: Event) => e.stopPropagation() },
            h("div", { className: "modal-header" },
              h("h2", null, `Node: ${selectedNode.id}`),
              h("button", { className: "modal-close", onClick: () => setSelectedNode(null) }, "\u00D7")
            ),
            h("div", { className: "modal-body" },
              h("div", { className: "detail-grid" },
                h("div", null, h("strong", null, "Location:"), ` ${selectedNode.location}`),
                h("div", null, h("strong", null, "IP:"), ` ${selectedNode.ip}`),
                h("div", null, h("strong", null, "Status:"), " ", statusDot(selectedNode.status), ` ${selectedNode.status}`),
                h("div", null, h("strong", null, "Agents:"), ` ${selectedNode.agents}`),
                h("div", null, h("strong", null, "Load:"),
                  ` ${selectedNode.loadAvg.one} / ${selectedNode.loadAvg.five} / ${selectedNode.loadAvg.fifteen}`),
                h("div", null, h("strong", null, "Memory:"),
                  ` ${selectedNode.memUsedGB} / ${selectedNode.memTotalGB} GB`),
                h("div", null, h("strong", null, "Disk:"),
                  ` ${selectedNode.diskUsedGB} / ${selectedNode.diskTotalGB} GB`),
                h("div", null, h("strong", null, "Tags:"), ` ${(selectedNode.tags || []).join(", ")}`)
              ),
              h("h3", null, "Notes"),
              selectedNode.notes && selectedNode.notes.length > 0
                ? selectedNode.notes.map((note: Note, i: number) =>
                    h("div", { key: i, className: "note" },
                      h("span", { className: "note-author" }, note.author),
                      h("span", { className: "note-time" }, ` — ${note.createdAt}`),
                      h("p", null, note.text)
                    )
                  )
                : h("p", { className: "muted" }, "No notes"),
              h("div", { className: "modal-actions" },
                canUpdate && selectedNode.status === "offline"
                  ? h("button", { className: "btn btn-success", onClick: () => handleStart(selectedNode.id) }, "Start")
                  : null,
                canUpdate && selectedNode.status === "online"
                  ? h("button", { className: "btn btn-warning", onClick: () => handleStop(selectedNode.id) }, "Stop")
                  : null,
                canDelete
                  ? h("button", { className: "btn btn-danger", onClick: () => handleDelete(selectedNode.id) }, "Delete")
                  : null
              )
            )
          )
        )
      : null,

    // Provision dialog
    showProvision
      ? h(ProvisionDialog, {
          onClose: () => setShowProvision(false),
          onCreated: () => { setShowProvision(false); loadNodes(); },
        })
      : null
  );
}

function ProvisionDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [host, setHost] = useState("");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [location, setLocation] = useState("");
  const [tags, setTags] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await apiPost("/nmgr/node", {
        host,
        user,
        password: password || undefined,
        location,
        tags: tags ? tags.split(",").map((t: string) => t.trim()) : [],
      });
      onCreated();
    } catch (err: any) {
      setError(err?.message || "Failed to provision node");
    }
    setSubmitting(false);
  }

  return h("div", { className: "modal-overlay", onClick: onClose },
    h("div", { className: "modal", onClick: (e: Event) => e.stopPropagation() },
      h("div", { className: "modal-header" },
        h("h2", null, "Provision Node"),
        h("button", { className: "modal-close", onClick: onClose }, "\u00D7")
      ),
      h("form", { className: "modal-body", onSubmit: handleSubmit },
        h("label", null, "Host (IPv4/IPv6/FQDN)"),
        h("input", { type: "text", value: host, onInput: (e: any) => setHost(e.target.value), required: true }),
        h("label", null, "SSH User"),
        h("input", { type: "text", value: user, onInput: (e: any) => setUser(e.target.value), required: true }),
        h("label", null, "Password (optional, first connection only)"),
        h("input", { type: "password", value: password, onInput: (e: any) => setPassword(e.target.value) }),
        h("label", null, "Location"),
        h("input", { type: "text", value: location, onInput: (e: any) => setLocation(e.target.value) }),
        h("label", null, "Tags (comma-separated)"),
        h("input", { type: "text", value: tags, onInput: (e: any) => setTags(e.target.value), placeholder: "cpu:amd64, os:linux" }),
        error ? h("div", { className: "error" }, error) : null,
        h("div", { className: "modal-actions" },
          h("button", { type: "button", className: "btn", onClick: onClose }, "Cancel"),
          h("button", { type: "submit", className: "btn btn-primary", disabled: submitting },
            submitting ? "Provisioning..." : "Provision"
          )
        )
      )
    )
  );
}
