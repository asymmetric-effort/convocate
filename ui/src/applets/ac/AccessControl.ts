import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import type { User, Group, Role, GlobalSettings, Page } from "../../types/api";
import { apiGet, apiPost, apiPatch, apiDelete, apiPut } from "../../lib/api";
import { hasRole } from "../../lib/auth";

const h = createElement;

export function AccessControl() {
  const [tab, setTab] = useState<"users" | "groups" | "settings">("users");

  return h("div", { className: "ac" },
    h("div", { className: "tab-bar" },
      h("button", { className: `tab ${tab === "users" ? "active" : ""}`, onClick: () => setTab("users") }, "Users"),
      h("button", { className: `tab ${tab === "groups" ? "active" : ""}`, onClick: () => setTab("groups") }, "Groups"),
      h("button", { className: `tab ${tab === "settings" ? "active" : ""}`, onClick: () => setTab("settings") }, "Global Settings")
    ),
    tab === "users" ? h(UsersTab, null) : null,
    tab === "groups" ? h(GroupsTab, null) : null,
    tab === "settings" ? h(SettingsTab, null) : null
  );
}

function UsersTab() {
  const [users, setUsers] = useState<User[]>([]);
  const canUpdate = hasRole("access-update");

  useEffect(() => { loadUsers(); }, []);

  async function loadUsers() {
    const page = await apiGet<Page<User>>("/ac/user?limit=200");
    setUsers(page.items);
  }

  async function toggleStatus(user: User) {
    const newStatus = user.status === "active" ? "disabled" : "active";
    await apiPatch(`/ac/user/${user.id}`, { status: newStatus });
    loadUsers();
  }

  async function deleteUser(id: string) {
    if (!confirm("Delete this user?")) return;
    await apiDelete(`/ac/user/${id}`);
    loadUsers();
  }

  return h("div", null,
    h("div", { className: "grid-list" },
      h("div", { className: "grid-header", style: { gridTemplateColumns: "1fr 2fr 1fr 100px 120px" } },
        h("span", null, "Name"), h("span", null, "Email"), h("span", null, "Groups"), h("span", null, "Status"), h("span", null, "Actions")
      ),
      users.map((user: User, i: number) =>
        h("div", { key: user.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "1fr 2fr 1fr 100px 120px" } },
          h("span", null, user.name),
          h("span", null, user.email),
          h("span", null, (user.groups || []).join(", ")),
          h("span", null, h("span", { className: "status-dot", style: { backgroundColor: user.status === "active" ? "#4caf50" : "#999" } }), " ", user.status),
          h("span", { className: "cell-controls" },
            canUpdate ? h("button", { className: "btn btn-sm", onClick: () => toggleStatus(user) }, user.status === "active" ? "Disable" : "Enable") : null,
            canUpdate ? h("button", { className: "btn btn-sm btn-danger", onClick: () => deleteUser(user.id) }, "Delete") : null
          )
        )
      )
    )
  );
}

function GroupsTab() {
  const [groups, setGroups] = useState<Group[]>([]);

  useEffect(() => { loadGroups(); }, []);

  async function loadGroups() {
    const page = await apiGet<Page<Group>>("/ac/group?limit=200");
    setGroups(page.items);
  }

  return h("div", null,
    h("div", { className: "grid-list" },
      h("div", { className: "grid-header", style: { gridTemplateColumns: "1fr 1fr 80px 2fr 60px" } },
        h("span", null, "Name"), h("span", null, "Type"), h("span", null, "Users"), h("span", null, "Roles"), h("span", null, "")
      ),
      groups.map((group: Group, i: number) =>
        h("div", { key: group.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "1fr 1fr 80px 2fr 60px" } },
          h("span", null, group.name),
          h("span", null, group.builtin ? "Built-in" : "Custom"),
          h("span", null, String(group.userCount)),
          h("span", { style: { fontSize: "11px" } }, (group.roles || []).join(", ")),
          h("span", null)
        )
      )
    )
  );
}

function SettingsTab() {
  const [settings, setSettings] = useState<GlobalSettings | null>(null);
  const canUpdate = hasRole("access-update");

  useEffect(() => { loadSettings(); }, []);

  async function loadSettings() {
    const s = await apiGet<GlobalSettings>("/ac/settings");
    setSettings(s);
  }

  if (!settings) return h("div", { className: "applet-loading" }, "Loading settings...");

  return h("div", { style: { maxWidth: "400px", padding: "12px" } },
    h("div", { className: "detail-grid" },
      h("div", null, h("strong", null, "Require MFA: "), settings.requireMfa ? "Yes" : "No"),
      h("div", null, h("strong", null, "Session Timeout: "), `${settings.sessionTimeoutMinutes} min`),
      h("div", null, h("strong", null, "Min Password Length: "), String(settings.passwordMinLength)),
      h("div", null, h("strong", null, "Password Rotation: "), `${settings.passwordRotationDays} days`)
    )
  );
}
