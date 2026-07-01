/**
 * Access Control Applet
 *
 * RBAC administration: Users, Groups, and Global Settings tabs.
 * Data source: /api/v1/ac/
 */

import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import { Button, Modal, TextField, Spinner, Tabs, Tag, DataGrid, Toggle } from "@asymmetric-effort/specifyjs/components";
import { useMenuBar } from "./use-menu-bar";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface User { id: string; email: string; name: string; status: string; groups: string[] }
interface Group { id: string; name: string; builtin: boolean; userCount: number; roles: string[] }
interface GlobalSettings { requireMfa: boolean; sessionTimeoutMinutes: number; passwordMinLength: number; passwordRotationDays: number }

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) };
}

async function fetchUsers(): Promise<User[]> {
  const res = await fetch("/api/v1/ac/user?limit=200", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return (await res.json()).items || [];
}

async function createUser(data: { email: string; name: string; password: string }): Promise<User> {
  const res = await fetch("/api/v1/ac/user", { method: "POST", headers: authHeaders(), body: JSON.stringify(data) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function deleteUser(id: string): Promise<void> {
  const res = await fetch(`/api/v1/ac/user/${id}`, { method: "DELETE", headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
}

async function fetchGroups(): Promise<Group[]> {
  const res = await fetch("/api/v1/ac/group?limit=200", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return (await res.json()).items || [];
}

async function createGroup(name: string): Promise<Group> {
  const res = await fetch("/api/v1/ac/group", { method: "POST", headers: authHeaders(), body: JSON.stringify({ name }) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function fetchSettings(): Promise<GlobalSettings> {
  const res = await fetch("/api/v1/ac/settings", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function saveSettings(s: GlobalSettings): Promise<GlobalSettings> {
  const res = await fetch("/api/v1/ac/settings", { method: "PUT", headers: authHeaders(), body: JSON.stringify(s) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------

export function AccessControl({ principal }: { principal?: any } = {}) {
  const [users, setUsers] = useState<User[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [settings, setSettings] = useState<GlobalSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showAddUser, setShowAddUser] = useState(false);

  useMenuBar("ac", [
    { label: "Admin", items: [
      { label: "Add User", onClick: () => setShowAddUser(true) },
      { label: "Add Group", onClick: () => setShowAddGroup(true) },
    ]},
  ]);
  const [showAddGroup, setShowAddGroup] = useState(false);
  const [newUserEmail, setNewUserEmail] = useState("");
  const [newUserName, setNewUserName] = useState("");
  const [newUserPassword, setNewUserPassword] = useState("");
  const [newGroupName, setNewGroupName] = useState("");

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const [u, g, s] = await Promise.all([fetchUsers(), fetchGroups(), fetchSettings()]);
      setUsers(u); setGroups(g); setSettings(s);
    } catch (err: any) { setError(err.message); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { loadAll(); }, [loadAll]);

  const handleCreateUser = useCallback(async () => {
    if (!newUserEmail.trim() || !newUserName.trim()) { setError("Email and name are required."); return; }
    try {
      await createUser({ email: newUserEmail.trim(), name: newUserName.trim(), password: newUserPassword || "changeme" });
      setNewUserEmail(""); setNewUserName(""); setNewUserPassword(""); setShowAddUser(false);
      await loadAll();
    } catch (err: any) { setError(err.message); }
  }, [newUserEmail, newUserName, newUserPassword, loadAll]);

  const handleCreateGroup = useCallback(async () => {
    if (!newGroupName.trim()) { setError("Group name is required."); return; }
    try {
      await createGroup(newGroupName.trim());
      setNewGroupName(""); setShowAddGroup(false);
      await loadAll();
    } catch (err: any) { setError(err.message); }
  }, [newGroupName, loadAll]);

  const handleSaveSettings = useCallback(async () => {
    if (!settings) return;
    try { await saveSettings(settings); } catch (err: any) { setError(err.message); }
  }, [settings]);

  if (loading) {
    return h("div", { style: { display: "flex", flexDirection: "column", alignItems: "stretch", justifyContent: "flex-start", width: "100%", height: "100%", backgroundColor: "#1e1e1e", color: "#e0e0e0" }, "data-testid": "access-control" },
      h("div", { style: { flex: 1, display: "flex", alignItems: "center", justifyContent: "center" } }, h(Spinner, null))
    );
  }

  // Users tab
  const usersTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    h("div", { style: { display: "flex", justifyContent: "flex-end", marginBottom: "8px" } },
      h(Button, { variant: "primary" as const, onClick: () => setShowAddUser(true) }, "Add User")
    ),
    h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
      h(DataGrid, {
        columns: [
          { key: "name", header: "Name", width: 150 },
          { key: "email", header: "Email", width: 200 },
          { key: "status", header: "Status", width: 100, render: (v: string) => h(Tag, { label: v, color: v === "active" ? "green" : "red", variant: "solid" as const, size: "sm" as const }) },
          { key: "groups", header: "Groups", width: 150 },
          { key: "actions", header: "", width: 160, render: (_: string, row: any) => h("div", { style: { display: "flex", gap: "4px" } },
            h(Button, {
              variant: (row.status === "active" ? "warning" : "primary") as any,
              onClick: () => {
                const newStatus = row.status === "active" ? "disabled" : "active";
                fetch(`/api/v1/ac/user/${row.id}`, { method: "PATCH", headers: authHeaders(), body: JSON.stringify({ status: newStatus }) }).then(loadAll);
              },
            }, row.status === "active" ? "Disable" : "Enable"),
            h(Button, { variant: "danger" as any, onClick: () => deleteUser(row.id).then(loadAll) }, "Delete"),
          ) },
        ],
        data: users.map((u) => ({ id: u.id, name: u.name, email: u.email, status: u.status, groups: u.groups.join(", "), actions: "" })),
        striped: true,
      })
    )
  );

  // Groups tab
  const groupsTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    h("div", { style: { display: "flex", justifyContent: "flex-end", marginBottom: "8px" } },
      h(Button, { variant: "primary" as const, onClick: () => setShowAddGroup(true) }, "Add Group")
    ),
    h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
      h(DataGrid, {
        columns: [
          { key: "name", header: "Name", width: 150 },
          { key: "userCount", header: "Users", width: 80 },
          { key: "roles", header: "Roles", width: 250 },
          { key: "builtin", header: "Built-in", width: 80 },
          { key: "actions", header: "", width: 80, render: (_: string, row: any) =>
            row.builtin === "Yes" ? null : h(Button, {
              variant: "danger" as any,
              onClick: () => fetch(`/api/v1/ac/group/${row.id}`, { method: "DELETE", headers: authHeaders() }).then(loadAll),
            }, "Delete")
          },
        ],
        data: groups.map((g) => ({ id: g.id, name: g.name, userCount: String(g.userCount), roles: g.roles.join(", "), builtin: g.builtin ? "Yes" : "No" })),
        striped: true,
      })
    )
  );

  // Settings tab
  const settingsTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "16px", display: "flex", flexDirection: "column", gap: "16px" } },
    settings ? h("div", { style: { display: "flex", flexDirection: "column", gap: "12px" } },
      h("div", { style: { display: "flex", alignItems: "center", justifyContent: "space-between" } },
        h("span", null, "Require MFA"),
        h(Toggle, { checked: settings.requireMfa, onChange: (v: boolean) => setSettings({ ...settings, requireMfa: v }) })
      ),
      h("div", null,
        h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Session Timeout (minutes)"),
        h(TextField, { value: String(settings.sessionTimeoutMinutes), onChange: (v: string) => setSettings({ ...settings, sessionTimeoutMinutes: parseInt(v) || 0 }) })
      ),
      h("div", null,
        h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Min Password Length"),
        h(TextField, { value: String(settings.passwordMinLength), onChange: (v: string) => setSettings({ ...settings, passwordMinLength: parseInt(v) || 0 }) })
      ),
      h("div", null,
        h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Password Rotation (days)"),
        h(TextField, { value: String(settings.passwordRotationDays), onChange: (v: string) => setSettings({ ...settings, passwordRotationDays: parseInt(v) || 0 }) })
      ),
      h("div", { style: { display: "flex", justifyContent: "flex-end" } },
        h(Button, { variant: "primary" as const, onClick: handleSaveSettings }, "Save Settings")
      )
    ) : null
  );

  return h("div", { style: { display: "flex", flexDirection: "column", alignItems: "stretch", justifyContent: "flex-start", width: "100%", height: "100%", backgroundColor: "#1e1e1e", color: "#e0e0e0" }, "data-testid": "access-control" },
    error ? h("div", { style: { padding: "4px 8px", backgroundColor: "#3d1c1c", color: "#ff8888", fontSize: "12px", flexShrink: 0 }, onClick: () => setError("") }, error) : null,
    h("div", { style: { flex: 1, minHeight: 0, overflow: "auto", display: "flex", flexDirection: "column", alignItems: "stretch", justifyContent: "flex-start" } },
      h(Tabs, { tabs: [
        { id: "users", label: "Users", content: usersTab },
        { id: "groups", label: "Groups", content: groupsTab },
        { id: "settings", label: "Settings", content: settingsTab },
      ] }),
    ),
    // Add User dialog
    showAddUser ? h(Modal, { open: true, onClose: () => setShowAddUser(false), title: "Add User", size: "sm" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h(TextField, { placeholder: "Email", value: newUserEmail, onChange: (v: string) => setNewUserEmail(v) }),
        h(TextField, { placeholder: "Name", value: newUserName, onChange: (v: string) => setNewUserName(v) }),
        h(TextField, { placeholder: "Password", type: "password", value: newUserPassword, onChange: (v: string) => setNewUserPassword(v) }),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setShowAddUser(false) }, "Cancel"),
          h(Button, { variant: "primary" as const, onClick: handleCreateUser }, "Add")
        )
      )
    ) : null,
    // Add Group dialog
    showAddGroup ? h(Modal, { open: true, onClose: () => setShowAddGroup(false), title: "Add Group", size: "sm" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h(TextField, { placeholder: "Group name", value: newGroupName, onChange: (v: string) => setNewGroupName(v) }),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setShowAddGroup(false) }, "Cancel"),
          h(Button, { variant: "primary" as const, onClick: handleCreateGroup }, "Add")
        )
      )
    ) : null,
  );
}
