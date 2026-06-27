import { createElement } from "@asymmetric-effort/specifyjs";
import { Tabs, DataGrid } from "@asymmetric-effort/specifyjs/components";
import { useRest } from "@asymmetric-effort/specifyjs/client";
import type { TabDefinition, DataGridColumn } from "@asymmetric-effort/specifyjs/components";
import { api } from "../../lib/client";

const h = createElement;

const userColumns: DataGridColumn[] = [
  { key: "name", header: "Name", width: "150px" },
  { key: "email", header: "Email", width: "200px" },
  { key: "status", header: "Status", width: "80px" },
  { key: "groups", header: "Groups", width: "200px",
    render: (val: unknown) => ((val as string[]) || []).join(", ") },
];

const groupColumns: DataGridColumn[] = [
  { key: "name", header: "Name", width: "150px" },
  { key: "builtin", header: "Type", width: "80px",
    render: (val: unknown) => val ? "Built-in" : "Custom" },
  { key: "userCount", header: "Users", width: "60px" },
  { key: "roles", header: "Roles", width: "300px",
    render: (val: unknown) => ((val as string[]) || []).join(", ") },
];

export function AccessControl() {
  const users = useRest(api, "/ac/user?limit=200");
  const groups = useRest(api, "/ac/group?limit=200");
  const settings = useRest(api, "/ac/settings");

  const tabs: TabDefinition[] = [
    {
      id: "users", label: "Users",
      content: users.loading
        ? h("div", null, "Loading...")
        : h(DataGrid, { columns: userColumns, data: (users.data?.items || []) as any, striped: true, compact: true }),
    },
    {
      id: "groups", label: "Groups",
      content: groups.loading
        ? h("div", null, "Loading...")
        : h(DataGrid, { columns: groupColumns, data: (groups.data?.items || []) as any, striped: true, compact: true }),
    },
    {
      id: "settings", label: "Global Settings",
      content: settings.loading
        ? h("div", null, "Loading...")
        : h("div", { style: { padding: "16px" } },
            h("div", null, h("strong", null, "Require MFA: "), String(settings.data?.requireMfa)),
            h("div", null, h("strong", null, "Session Timeout: "), `${settings.data?.sessionTimeoutMinutes} min`),
            h("div", null, h("strong", null, "Min Password Length: "), String(settings.data?.passwordMinLength)),
            h("div", null, h("strong", null, "Password Rotation: "), `${settings.data?.passwordRotationDays} days`)
          ),
    },
  ];

  return h("div", { style: { padding: "8px" } }, h(Tabs, { tabs, variant: "line" }));
}
