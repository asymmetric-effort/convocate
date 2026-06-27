import { createElement, useState } from "@asymmetric-effort/specifyjs";
import { DataGrid, Toolbar } from "@asymmetric-effort/specifyjs/components";
import { useRest } from "@asymmetric-effort/specifyjs/client";
import type { DataGridColumn } from "@asymmetric-effort/specifyjs/components";
import { api } from "../../lib/client";

const h = createElement;

const repoColumns: DataGridColumn[] = [
  { key: "name", header: "Name", width: "150px" },
  { key: "description", header: "Description", width: "250px" },
  { key: "visibility", header: "Visibility", width: "100px" },
  { key: "defaultBranch", header: "Branch", width: "80px" },
  { key: "updatedAt", header: "Updated", width: "120px",
    render: (val: unknown) => (val as string)?.split("T")[0] || "" },
];

export function RepoManager() {
  const { data, loading } = useRest(api, "/repo/repo?limit=200");

  if (loading) return h("div", { style: { padding: "20px", color: "#888" } }, "Loading repositories...");

  return h("div", { style: { padding: "8px" } },
    h(Toolbar, { items: [{ id: "create", label: "Create Repository", onClick: () => {} }], size: "sm" as any }),
    h("div", { style: { fontSize: "12px", color: "#888", padding: "4px 0" } }, `${data?.total || 0} repositories`),
    h(DataGrid, { columns: repoColumns, data: (data?.items || []) as any, striped: true, compact: true })
  );
}
