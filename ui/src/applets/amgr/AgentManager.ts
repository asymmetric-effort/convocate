import { createElement, useState } from "@asymmetric-effort/specifyjs";
import { Accordion, DataGrid, Toolbar } from "@asymmetric-effort/specifyjs/components";
import { useRest } from "@asymmetric-effort/specifyjs/client";
import type { AccordionSection, DataGridColumn } from "@asymmetric-effort/specifyjs/components";
import type { Agent, Node, Page } from "../../types/api";
import { api } from "../../lib/client";

const h = createElement;

const agentColumns: DataGridColumn[] = [
  { key: "id", header: "ID", width: "150px" },
  { key: "project", header: "Project", width: "120px" },
  { key: "status", header: "Status", width: "100px",
    render: (val: unknown) => {
      const s = val as string;
      const colors: Record<string, string> = { running: "#4caf50", connected: "#2196f3", stopped: "#999", migrating: "#ff9800", stopping: "#f44336" };
      return h("span", null, h("span", { style: { display: "inline-block", width: "8px", height: "8px", borderRadius: "50%", backgroundColor: colors[s] || "#999", marginRight: "6px" } }), s);
    },
  },
  { key: "owner", header: "Owner", width: "120px" },
];

export function AgentManager() {
  const agents = useRest(api, "/amgr/agent?limit=200");
  const nodes = useRest(api, "/nmgr/node?limit=200");

  if (agents.loading || nodes.loading) return h("div", { style: { padding: "20px", color: "#888" } }, "Loading agents...");

  const agentList: Agent[] = agents.data?.items || [];
  const nodeList: Node[] = nodes.data?.items || [];

  if (agentList.length === 0) {
    return h("div", { style: { padding: "20px" } },
      h(Toolbar, { items: [{ id: "create", label: "Create Agent", onClick: () => {} }], size: "sm" as any }),
      h("div", { style: { color: "#888", padding: "20px", textAlign: "center" } },
        `No agent-containers running at ${new Date().toLocaleString()}`)
    );
  }

  const byNode: Record<string, Agent[]> = {};
  for (const a of agentList) {
    const nid = a.nodeId || "unassigned";
    if (!byNode[nid]) byNode[nid] = [];
    byNode[nid].push(a);
  }

  const sections: AccordionSection[] = Object.entries(byNode).map(([nodeId, nodeAgents]) => ({
    id: nodeId,
    header: `${nodeId} (${nodeAgents.length} agents)`,
    content: h(DataGrid, { columns: agentColumns, data: nodeAgents as any, striped: true, compact: true }),
  }));

  return h("div", { style: { padding: "8px" } },
    h(Toolbar, { items: [{ id: "create", label: "Create Agent", onClick: () => {} }], size: "sm" as any }),
    h(Accordion, { sections, allowMultiple: true, defaultExpanded: sections.map((s: AccordionSection) => s.id) })
  );
}
