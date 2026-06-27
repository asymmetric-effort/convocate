import { createElement, useState } from "@asymmetric-effort/specifyjs";
import { DataGrid, Toolbar, Card } from "@asymmetric-effort/specifyjs/components";
import { useRest } from "@asymmetric-effort/specifyjs/client";
import type { DataGridColumn } from "@asymmetric-effort/specifyjs/components";
import { api } from "../../lib/client";

const h = createElement;

const boardColumns: DataGridColumn[] = [
  { key: "name", header: "Name", width: "200px" },
  { key: "repoId", header: "Repository", width: "150px" },
  { key: "updatedAt", header: "Updated", width: "120px",
    render: (val: unknown) => (val as string)?.split("T")[0] || "" },
];

export function ProjectBoard() {
  const { data, loading } = useRest(api, "/pb/board?limit=200");

  if (loading) return h("div", { style: { padding: "20px", color: "#888" } }, "Loading boards...");

  return h("div", { style: { padding: "8px" } },
    h(Toolbar, {
      items: [{ id: "new", label: "New Board", onClick: () => {} }],
      size: "sm" as any,
    }),
    h("div", { style: { fontSize: "12px", color: "#888", padding: "4px 0" } }, `${data?.total || 0} boards`),
    h(DataGrid, { columns: boardColumns, data: (data?.items || []) as any, striped: true, compact: true })
  );
}
