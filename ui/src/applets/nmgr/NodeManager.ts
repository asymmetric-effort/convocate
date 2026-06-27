import { createElement, useState, useCallback } from "@asymmetric-effort/specifyjs";
import { DataGrid, Modal, Toolbar, Button } from "@asymmetric-effort/specifyjs/components";
import { useRest } from "@asymmetric-effort/specifyjs/client";
import type { DataGridColumn } from "@asymmetric-effort/specifyjs/components";
import type { Node, Page } from "../../types/api";
import { api } from "../../lib/client";

const h = createElement;

const STATUS_COLORS: Record<string, string> = {
  online: "#4caf50", draining: "#ff9800", offline: "#f44336",
};

const columns: DataGridColumn[] = [
  { key: "id", header: "Name", width: "120px" },
  { key: "location", header: "Location", width: "120px" },
  { key: "ip", header: "IP", width: "130px" },
  {
    key: "status", header: "Status", width: "100px",
    render: (val: unknown) => {
      const s = val as string;
      return h("span", null,
        h("span", { style: { display: "inline-block", width: "8px", height: "8px", borderRadius: "50%", backgroundColor: STATUS_COLORS[s] || "#999", marginRight: "6px" } }),
        s
      );
    },
  },
  { key: "agents", header: "Agents", width: "70px" },
  {
    key: "loadAvg", header: "Load (1/5/15)", width: "140px",
    render: (val: unknown) => {
      const la = val as { one: number; five: number; fifteen: number };
      return `${la.one.toFixed(1)} / ${la.five.toFixed(1)} / ${la.fifteen.toFixed(1)}`;
    },
  },
  {
    key: "memUsedGB", header: "Memory", width: "120px",
    render: (_: unknown, row: Record<string, unknown>) =>
      `${(row.memUsedGB as number).toFixed(1)} / ${(row.memTotalGB as number).toFixed(0)} GB`,
  },
];

export function NodeManager() {
  const { data, loading, error } = useRest(api, "/nmgr/node?limit=100");
  const [page, setPage] = useState(0);
  const pageSize = 25;

  const nodes: Node[] = data?.items || [];
  const total: number = data?.total || 0;

  if (loading) return h("div", { style: { padding: "20px", color: "#888" } }, "Loading nodes...");
  if (error) return h("div", { style: { padding: "20px", color: "#e55" } }, `Error: ${error.message}`);

  return h("div", { style: { padding: "8px" } },
    h(Toolbar, {
      items: [
        { id: "provision", label: "Provision Node", onClick: () => {} },
      ],
      size: "sm" as any,
    }),
    h("div", { style: { fontSize: "12px", color: "#888", padding: "4px 0" } }, `${total} nodes`),
    h(DataGrid, {
      columns,
      data: nodes as any,
      pageSize,
      currentPage: page,
      onPageChange: setPage,
      striped: true,
      stickyHeader: true,
      compact: true,
    })
  );
}
