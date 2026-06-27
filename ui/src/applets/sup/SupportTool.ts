import { createElement } from "@asymmetric-effort/specifyjs";
import { DataGrid, Toolbar } from "@asymmetric-effort/specifyjs/components";
import { useRest } from "@asymmetric-effort/specifyjs/client";
import type { DataGridColumn } from "@asymmetric-effort/specifyjs/components";
import { api } from "../../lib/client";

const h = createElement;

const ticketColumns: DataGridColumn[] = [
  { key: "id", header: "ID", width: "80px" },
  { key: "subject", header: "Subject", width: "250px" },
  { key: "priority", header: "Priority", width: "80px" },
  { key: "status", header: "Status", width: "100px" },
  { key: "updatedAt", header: "Updated", width: "120px",
    render: (val: unknown) => (val as string)?.split("T")[0] || "" },
];

export function SupportTool() {
  const { data, loading } = useRest(api, "/sup/ticket?limit=200");

  if (loading) return h("div", { style: { padding: "20px", color: "#888" } }, "Loading tickets...");

  return h("div", { style: { padding: "8px" } },
    h(Toolbar, { items: [{ id: "new", label: "New Ticket", onClick: () => {} }], size: "sm" as any }),
    h("div", { style: { fontSize: "12px", color: "#888", padding: "4px 0" } }, `${data?.total || 0} tickets`),
    h(DataGrid, { columns: ticketColumns, data: (data?.items || []) as any, striped: true, compact: true })
  );
}
