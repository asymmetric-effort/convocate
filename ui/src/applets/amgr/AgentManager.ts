import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import type { Agent, Page, Node } from "../../types/api";
import { apiGet, apiPost, apiDelete } from "../../lib/api";
import { hasRole } from "../../lib/auth";

const h = createElement;

export function AgentManager() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [loading, setLoading] = useState(true);

  const [lastRefresh, setLastRefresh] = useState<string>(new Date().toISOString());
  const canUpdate = hasRole("agent-update");

  useEffect(() => {
    loadData();
    const interval = setInterval(() => {
      loadData();
      setLastRefresh(new Date().toISOString());
    }, 15000);
    return () => clearInterval(interval);
  }, []);

  async function loadData() {
    setLoading(true);
    try {
      const [agentPage, nodePage] = await Promise.all([
        apiGet<Page<Agent>>("/amgr/agent?limit=200"),
        apiGet<Page<Node>>("/nmgr/node?limit=200"),
      ]);
      setAgents(agentPage.items);
      setNodes(nodePage.items);
    } catch (e) { console.error("Failed to load agents", e); }
    setLoading(false);
  }

  async function handleStop(id: string) {
    await apiPost(`/amgr/agent/${id}/stop`);
    loadData();
  }

  async function handleDelete(id: string) {
    if (!confirm("Delete this agent?")) return;
    await apiDelete(`/amgr/agent/${id}`);
    loadData();
  }

  function statusColor(status: string) {
    const colors: Record<string, string> = { running: "#4caf50", connected: "#2196f3", stopped: "#999", migrating: "#ff9800", stopping: "#f44336" };
    return colors[status] || "#999";
  }

  if (loading) return h("div", { className: "applet-loading" }, "Loading agents...");

  const agentsByNode: Record<string, Agent[]> = {};
  for (const agent of agents) {
    const nodeId = agent.nodeId || "unassigned";
    if (!agentsByNode[nodeId]) agentsByNode[nodeId] = [];
    agentsByNode[nodeId].push(agent);
  }

  return h("div", { className: "amgr" },
    h("div", { className: "applet-toolbar" },
      canUpdate ? h("button", { className: "btn btn-primary", onClick: () => setShowCreate(true) }, "Create Agent") : null,
      h("span", { className: "applet-count" }, `${agents.length} agent${agents.length !== 1 ? "s" : ""}`)
    ),
    Object.keys(agentsByNode).length === 0
      ? h("div", { className: "muted", style: { padding: "20px" } }, `No agent-containers running at ${new Date(lastRefresh).toLocaleString()}`)
      : Object.entries(agentsByNode).map(([nodeId, nodeAgents]) =>
          h("div", { key: nodeId, className: "accordion-section" },
            h("div", { className: "accordion-header" }, `Node: ${nodeId} (${nodeAgents.length} agents)`),
            h("div", { className: "grid-list" },
              h("div", { className: "grid-header", style: { gridTemplateColumns: "1fr 1fr 100px 1fr 1fr 120px" } },
                h("span", null, "ID"), h("span", null, "Project"), h("span", null, "Status"),
                h("span", null, "Expose"), h("span", null, "Owner"), h("span", null, "Controls")
              ),
              nodeAgents.map((agent: Agent, i: number) =>
                h("div", { key: agent.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "1fr 1fr 100px 1fr 1fr 120px" } },
                  h("span", { className: "cell-id" }, agent.id),
                  h("span", null, agent.project),
                  h("span", null, h("span", { className: "status-dot", style: { backgroundColor: statusColor(agent.status) } }), " ", agent.status),
                  h("span", null, agent.expose || "—"),
                  h("span", null, agent.owner || "—"),
                  h("span", { className: "cell-controls" },
                    canUpdate && agent.status === "running" ? h("button", { className: "btn btn-sm btn-warning", onClick: () => handleStop(agent.id) }, "Stop") : null,
                    canUpdate ? h("button", { className: "btn btn-sm btn-danger", onClick: () => handleDelete(agent.id) }, "Delete") : null
                  )
                )
              )
            )
          )
        ),
    showCreate ? h(CreateAgentDialog, { nodes, onClose: () => setShowCreate(false), onCreated: () => { setShowCreate(false); loadData(); } }) : null
  );
}

function CreateAgentDialog({ nodes, onClose, onCreated }: { nodes: Node[]; onClose: () => void; onCreated: () => void }) {
  const [project, setProject] = useState("");
  const [nodeId, setNodeId] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await apiPost("/amgr/agent", { project, nodeId: nodeId || undefined });
      onCreated();
    } catch (err: any) { setError(err?.message || "Failed to create agent"); }
    setSubmitting(false);
  }

  return h("div", { className: "modal-overlay", onClick: onClose },
    h("div", { className: "modal", onClick: (e: Event) => e.stopPropagation() },
      h("div", { className: "modal-header" },
        h("h2", null, "Create Agent"),
        h("button", { className: "modal-close", onClick: onClose }, "\u00D7")
      ),
      h("form", { className: "modal-body", onSubmit: handleSubmit },
        h("label", null, "Project Name"),
        h("input", { type: "text", value: project, onInput: (e: any) => setProject(e.target.value), required: true }),
        h("label", null, "Node (optional)"),
        h("select", { value: nodeId, onInput: (e: any) => setNodeId(e.target.value) },
          h("option", { value: "" }, "Any node"),
          ...nodes.map((n: Node) => h("option", { key: n.id, value: n.id }, n.id))
        ),
        error ? h("div", { className: "error" }, error) : null,
        h("div", { className: "modal-actions" },
          h("button", { type: "button", className: "btn", onClick: onClose }, "Cancel"),
          h("button", { type: "submit", className: "btn btn-primary", disabled: submitting }, submitting ? "Creating..." : "Create")
        )
      )
    )
  );
}
