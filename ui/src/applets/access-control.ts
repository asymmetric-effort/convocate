/**
 * Access Control Applet
 *
 * RBAC administration: Users, Groups, and Global Settings tabs.
 * Data source: /api/v1/ac/
 */

import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import { Button, Modal, TextField, Spinner, Tabs, Tag, DataGrid, Toggle } from "@asymmetric-effort/specifyjs/components";
import { useMenuBar } from "./use-menu-bar";
import { hasRole, APPLET_ROLES } from "./use-rbac";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface User { id: string; email: string; name: string; status: string; groups: string[] }
interface Group { id: string; name: string; builtin: boolean; userCount: number; roles: string[] }
interface GlobalSettings { requireMfa: boolean; sessionTimeoutMinutes: number; passwordMinLength: number }
interface MFAEnrollResult { url: string; barcode: string }

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

async function createGroup(name: string, roles: string[]): Promise<Group> {
  const res = await fetch("/api/v1/ac/group", { method: "POST", headers: authHeaders(), body: JSON.stringify({ name, roles }) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function setGroupRoles(groupId: string, roles: string[]): Promise<Group> {
  const res = await fetch(`/api/v1/ac/group/${groupId}/role`, { method: "PUT", headers: authHeaders(), body: JSON.stringify({ roles }) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function updateGroupName(groupId: string, name: string): Promise<Group> {
  const res = await fetch(`/api/v1/ac/group/${groupId}`, { method: "PATCH", headers: authHeaders(), body: JSON.stringify({ name }) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function fetchRoles(): Promise<{ id: string; description: string }[]> {
  const res = await fetch("/api/v1/ac/role?limit=200", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return (await res.json()).items || [];
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

async function enrollMFA(userId: string): Promise<MFAEnrollResult> {
  const res = await fetch(`/api/v1/ac/user/${userId}/mfa/enroll`, { method: "POST", headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function destroyMFA(userId: string): Promise<void> {
  const res = await fetch(`/api/v1/ac/user/${userId}/mfa`, { method: "DELETE", headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
}

async function fetchMFAStatus(userId: string): Promise<boolean> {
  const res = await fetch(`/api/v1/ac/user/${userId}/mfa/status`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  const data = await res.json();
  return data.enrolled === true;
}

// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------

export function AccessControl({ principal }: { principal?: any } = {}) {
  const canUpdate = hasRole(principal, APPLET_ROLES.ac.update);
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
  const [newGroupRoles, setNewGroupRoles] = useState<string[]>([]);
  const [allRoles, setAllRoles] = useState<{ id: string; description: string }[]>([]);
  const [showEditGroup, setShowEditGroup] = useState(false);
  const [editingGroup, setEditingGroup] = useState<Group | null>(null);
  const [editGroupName, setEditGroupName] = useState("");
  const [editGroupRoles, setEditGroupRoles] = useState<string[]>([]);
  const [mfaStatuses, setMfaStatuses] = useState<Record<string, boolean>>({});
  const [showMfaEnroll, setShowMfaEnroll] = useState(false);
  const [mfaEnrollResult, setMfaEnrollResult] = useState<MFAEnrollResult | null>(null);
  const [mfaEnrolling, setMfaEnrolling] = useState(false);
  const [showEditUser, setShowEditUser] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [editUserName, setEditUserName] = useState("");
  const [editUserEmail, setEditUserEmail] = useState("");
  const [editUserGroups, setEditUserGroups] = useState<string[]>([]);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; userId: string } | null>(null);
  const [groupContextMenu, setGroupContextMenu] = useState<{ x: number; y: number; groupId: string } | null>(null);

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const [u, g, s, r] = await Promise.all([fetchUsers(), fetchGroups(), fetchSettings(), fetchRoles()]);
      setUsers(u); setGroups(g); setSettings(s); setAllRoles(r);
      // Load MFA statuses for all users in parallel
      const statusEntries = await Promise.all(
        u.map(async (user: User) => {
          try {
            const enrolled = await fetchMFAStatus(user.id);
            return [user.id, enrolled] as [string, boolean];
          } catch {
            return [user.id, false] as [string, boolean];
          }
        })
      );
      const statuses: Record<string, boolean> = {};
      for (const [id, enrolled] of statusEntries) { statuses[id] = enrolled; }
      setMfaStatuses(statuses);
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
      await createGroup(newGroupName.trim(), newGroupRoles);
      setNewGroupName(""); setNewGroupRoles([]); setShowAddGroup(false);
      await loadAll();
    } catch (err: any) { setError(err.message); }
  }, [newGroupName, newGroupRoles, loadAll]);

  const openEditGroup = useCallback((group: Group) => {
    setEditingGroup(group);
    setEditGroupName(group.name);
    setEditGroupRoles([...group.roles]);
    setShowEditGroup(true);
  }, []);

  const handleSaveEditGroup = useCallback(async () => {
    if (!editingGroup) return;
    if (!editGroupName.trim()) { setError("Group name is required."); return; }
    try {
      const nameChanged = editGroupName.trim() !== editingGroup.name;
      const rolesChanged = JSON.stringify([...editGroupRoles].sort()) !== JSON.stringify([...editingGroup.roles].sort());
      if (nameChanged) {
        await updateGroupName(editingGroup.id, editGroupName.trim());
      }
      if (rolesChanged) {
        await setGroupRoles(editingGroup.id, editGroupRoles);
      }
      setShowEditGroup(false);
      await loadAll();
    } catch (err: any) { setError(err.message); }
  }, [editingGroup, editGroupName, editGroupRoles, loadAll]);

  const handleSaveSettings = useCallback(async () => {
    if (!settings) return;
    try { await saveSettings(settings); } catch (err: any) { setError(err.message); }
  }, [settings]);

  const handleEnrollMFA = useCallback(async (userId: string) => {
    setMfaEnrolling(true);
    try {
      const result = await enrollMFA(userId);
      setMfaEnrollResult(result);
      setShowMfaEnroll(true);
    } catch (err: any) { setError(err.message); }
    finally { setMfaEnrolling(false); }
  }, []);

  const handleResetMFA = useCallback(async (userId: string) => {
    if (!confirm("Are you sure you want to reset MFA for this user?")) return;
    try {
      await destroyMFA(userId);
      await loadAll();
    } catch (err: any) { setError(err.message); }
  }, [loadAll]);

  const openEditUser = useCallback((row: any) => {
    const user = users.find((u) => u.id === row.id);
    if (!user) return;
    setEditingUser(user);
    setEditUserName(user.name);
    setEditUserEmail(user.email);
    setEditUserGroups([...user.groups]);
    setShowEditUser(true);
  }, [users]);

  const handleSaveEditUser = useCallback(async () => {
    if (!editingUser) return;
    if (!editUserName.trim()) { setError("Name is required."); return; }
    if (!editUserEmail.trim()) { setError("Email is required."); return; }
    try {
      await fetch(`/api/v1/ac/user/${editingUser.id}`, {
        method: "PATCH",
        headers: authHeaders(),
        body: JSON.stringify({ name: editUserName.trim(), email: editUserEmail.trim(), groups: editUserGroups }),
      });
      setShowEditUser(false);
      await loadAll();
    } catch (err: any) { setError(err.message); }
  }, [editingUser, editUserName, editUserEmail, editUserGroups, loadAll]);

  if (loading) {
    return h("div", { style: { display: "flex", flexDirection: "column", alignItems: "stretch", justifyContent: "flex-start", width: "100%", height: "100%", backgroundColor: "#1e1e1e", color: "#e0e0e0" }, "data-testid": "access-control" },
      h("div", { style: { flex: 1, display: "flex", alignItems: "center", justifyContent: "center" } }, h(Spinner, null))
    );
  }

  // Users tab
  const usersTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    canUpdate ? h("div", { style: { display: "flex", justifyContent: "flex-end", marginBottom: "8px" } },
      h(Button, { variant: "primary" as const, onClick: () => setShowAddUser(true) }, "Add User")
    ) : null,
    h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
      h(DataGrid, {
        columns: (() => {
          const userCell = (row: any, ...children: any[]) => h("div", {
            style: { width: "100%", height: "100%", display: "flex", alignItems: "center", margin: "-4px", padding: "4px", boxSizing: "border-box" as const },
            onContextMenu: (e: MouseEvent) => { e.preventDefault(); setContextMenu({ x: e.clientX, y: e.clientY, userId: row.id }); setGroupContextMenu(null); },
          }, ...children);
          return [
          { key: "name", header: "Name", width: 150, render: (v: string, row: any) => userCell(row, v) },
          { key: "email", header: "Email", width: 200, render: (v: string, row: any) => userCell(row, v) },
          { key: "status", header: "Status", width: 100, render: (v: string, row: any) => userCell(row, h(Tag, { label: v, color: v === "active" ? "green" : "red", variant: "solid" as const, size: "sm" as const })) },
          { key: "mfa", header: "MFA", width: 100, render: (v: string, row: any) => userCell(row, h(Tag, { label: v, color: v === "Enrolled" ? "green" : "gray", variant: "solid" as const, size: "sm" as const })) },
          { key: "groups", header: "Groups", width: 150, render: (v: string, row: any) => userCell(row, v) },
        ]; })(),
        data: (() => {
          const groupNameMap: Record<string, string> = {};
          for (const g of groups) { groupNameMap[g.id] = g.name; }
          return users.map((u) => ({ id: u.id, name: u.name, email: u.email, status: u.status, mfa: mfaStatuses[u.id] ? "Enrolled" : "Not Enrolled", groups: u.groups.map((gid: string) => groupNameMap[gid] || gid).join(", "), actions: "" }));
        })(),
        striped: true,
      })
    )
  );

  // Groups tab
  const groupsTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    canUpdate ? h("div", { style: { display: "flex", justifyContent: "flex-end", marginBottom: "8px" } },
      h(Button, { variant: "primary" as const, onClick: () => setShowAddGroup(true) }, "Add Group")
    ) : null,
    h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
      h(DataGrid, {
        columns: (() => {
          const groupCell = (row: any, ...children: any[]) => h("div", {
            style: { width: "100%", height: "100%", display: "flex", alignItems: "center", margin: "-4px", padding: "4px", boxSizing: "border-box" as const },
            onContextMenu: (e: MouseEvent) => { e.preventDefault(); setGroupContextMenu({ x: e.clientX, y: e.clientY, groupId: row.id }); setContextMenu(null); },
          }, ...children);
          return [
          { key: "name", header: "Name", width: 200, render: (v: string, row: any) => groupCell(row, v) },
          { key: "userCount", header: "Users", width: 80, render: (v: string, row: any) => groupCell(row, v) },
          { key: "roles", header: "Roles", width: 300, render: (v: string, row: any) => groupCell(row, v) },
          { key: "builtin", header: "Built-in", width: 80, render: (v: string, row: any) => groupCell(row, v) },
        ]; })(),
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
      h("div", { style: { display: "flex", justifyContent: "flex-end" } },
        h(Button, { variant: "primary" as const, onClick: handleSaveSettings }, "Save Settings")
      )
    ) : null
  );

  const menuItemStyle = { padding: "6px 12px", cursor: "pointer", fontSize: "13px", color: "#e0e0e0", whiteSpace: "nowrap" as const };

  return h("div", { style: { display: "flex", flexDirection: "column", alignItems: "stretch", justifyContent: "flex-start", width: "100%", height: "100%", backgroundColor: "#1e1e1e", color: "#e0e0e0" }, "data-testid": "access-control", onClick: () => { setContextMenu(null); setGroupContextMenu(null); } },
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
    showAddGroup ? h(Modal, { open: true, onClose: () => setShowAddGroup(false), title: "Add Group", size: "md" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h(TextField, { placeholder: "Group name", value: newGroupName, onChange: (v: string) => setNewGroupName(v) }),
        h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Assign Roles:"),
        h("div", { style: { maxHeight: "200px", overflowY: "auto", display: "flex", flexDirection: "column", gap: "4px", padding: "4px", backgroundColor: "#2a2a2a", borderRadius: "4px" } },
          ...allRoles.map((role) =>
            h("label", { key: role.id, style: { display: "flex", alignItems: "center", gap: "6px", fontSize: "12px", cursor: "pointer", padding: "2px 4px" } },
              h("input", {
                type: "checkbox",
                checked: newGroupRoles.includes(role.id),
                onChange: () => {
                  if (newGroupRoles.includes(role.id)) {
                    setNewGroupRoles(newGroupRoles.filter((r) => r !== role.id));
                  } else {
                    setNewGroupRoles([...newGroupRoles, role.id]);
                  }
                },
              }),
              h("span", null, role.id),
              h("span", { style: { color: "#888", marginLeft: "4px" } }, `— ${role.description}`),
            )
          )
        ),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setShowAddGroup(false) }, "Cancel"),
          h(Button, { variant: "primary" as const, onClick: handleCreateGroup }, "Create")
        )
      )
    ) : null,
    // MFA Enrollment dialog
    showMfaEnroll && mfaEnrollResult ? h(Modal, { open: true, onClose: () => { setShowMfaEnroll(false); setMfaEnrollResult(null); loadAll(); }, title: "Enroll MFA", size: "sm" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "16px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px", alignItems: "center" } },
        h("p", { style: { margin: 0, fontSize: "13px" } }, "Scan this QR code with your authenticator app:"),
        mfaEnrollResult.barcode
          ? h("img", { src: `data:image/png;base64,${mfaEnrollResult.barcode}`, alt: "TOTP QR Code", style: { width: "200px", height: "200px", imageRendering: "pixelated" } })
          : null,
        h("div", { style: { fontSize: "11px", color: "#aaa", wordBreak: "break-all", textAlign: "center" } },
          h("p", { style: { margin: "0 0 4px 0" } }, "Or enter this URL manually:"),
          h("code", { style: { fontSize: "10px" } }, mfaEnrollResult.url)
        ),
        h("div", { style: { display: "flex", justifyContent: "flex-end", width: "100%" } },
          h(Button, { variant: "primary" as const, onClick: () => { setShowMfaEnroll(false); setMfaEnrollResult(null); loadAll(); } }, "Done")
        )
      )
    ) : null,
    // Edit User dialog
    showEditUser && editingUser ? h(Modal, { open: true, onClose: () => setShowEditUser(false), title: `Edit User: ${editingUser.name}`, size: "md" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h("div", null,
          h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Name"),
          h(TextField, { value: editUserName, onChange: (v: string) => setEditUserName(v) })
        ),
        h("div", null,
          h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Email"),
          h(TextField, { value: editUserEmail, onChange: (v: string) => setEditUserEmail(v) })
        ),
        h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Group Membership:"),
        h("div", { style: { maxHeight: "200px", overflowY: "auto", display: "flex", flexDirection: "column", gap: "4px", padding: "4px", backgroundColor: "#2a2a2a", borderRadius: "4px" } },
          ...groups.map((group) =>
            h("label", { key: group.id, style: { display: "flex", alignItems: "center", gap: "6px", fontSize: "12px", cursor: "pointer", padding: "2px 4px" } },
              h("input", {
                type: "checkbox",
                checked: editUserGroups.includes(group.id),
                onChange: () => {
                  if (editUserGroups.includes(group.id)) {
                    setEditUserGroups(editUserGroups.filter((g) => g !== group.id));
                  } else {
                    setEditUserGroups([...editUserGroups, group.id]);
                  }
                },
              }),
              h("span", null, group.name),
            )
          )
        ),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setShowEditUser(false) }, "Cancel"),
          h(Button, { variant: "primary" as const, onClick: handleSaveEditUser }, "Save")
        )
      )
    ) : null,
    // Edit Group dialog
    showEditGroup && editingGroup ? h(Modal, { open: true, onClose: () => setShowEditGroup(false), title: `Edit Group: ${editingGroup.name}`, size: "md" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h("div", null,
          h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Group Name"),
          h(TextField, { value: editGroupName, onChange: (v: string) => setEditGroupName(v) })
        ),
        h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px" } }, "Assign Roles:"),
        h("div", { style: { maxHeight: "300px", overflowY: "auto", display: "flex", flexDirection: "column", gap: "4px", padding: "4px", backgroundColor: "#2a2a2a", borderRadius: "4px" } },
          ...allRoles.map((role) =>
            h("label", { key: role.id, style: { display: "flex", alignItems: "center", gap: "6px", fontSize: "12px", cursor: "pointer", padding: "2px 4px" } },
              h("input", {
                type: "checkbox",
                checked: editGroupRoles.includes(role.id),
                onChange: () => {
                  if (editGroupRoles.includes(role.id)) {
                    setEditGroupRoles(editGroupRoles.filter((r) => r !== role.id));
                  } else {
                    setEditGroupRoles([...editGroupRoles, role.id]);
                  }
                },
              }),
              h("span", null, role.id),
              h("span", { style: { color: "#888", marginLeft: "4px" } }, `— ${role.description}`),
            )
          )
        ),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setShowEditGroup(false) }, "Cancel"),
          h(Button, { variant: "primary" as const, onClick: handleSaveEditGroup }, "Save")
        )
      )
    ) : null,
    // User row context menu
    contextMenu ? h("div", {
      style: {
        position: "fixed", left: contextMenu.x + "px", top: contextMenu.y + "px",
        backgroundColor: "#2d2d2d", border: "1px solid #555", borderRadius: "4px",
        padding: "4px 0", zIndex: 1000, minWidth: "150px",
      },
      onClick: (e: MouseEvent) => e.stopPropagation(),
    },
      h("div", { style: { ...menuItemStyle }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => {
        const user = users.find((u) => u.id === contextMenu.userId);
        const newStatus = user && user.status === "active" ? "disabled" : "active";
        fetch(`/api/v1/ac/user/${contextMenu.userId}`, { method: "PATCH", headers: authHeaders(), body: JSON.stringify({ status: newStatus }) }).then(loadAll);
        setContextMenu(null);
      } }, (() => { const u = users.find((u) => u.id === contextMenu.userId); return u && u.status === "active" ? "Disable User" : "Enable User"; })()),
      h("div", { style: { height: "1px", backgroundColor: "#555", margin: "4px 0" } }),
      h("div", { style: { ...menuItemStyle }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => { openEditUser({ id: contextMenu.userId }); setContextMenu(null); } }, "Edit User"),
      h("div", { style: { ...menuItemStyle }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => {
        const newPass = prompt("Enter new password for this user:");
        if (newPass) {
          fetch(`/api/v1/ac/user/${contextMenu.userId}`, { method: "PATCH", headers: authHeaders(), body: JSON.stringify({ password: newPass }) }).then(loadAll);
        }
        setContextMenu(null);
      } }, "Reset Password"),
      h("div", { style: { height: "1px", backgroundColor: "#555", margin: "4px 0" } }),
      mfaStatuses[contextMenu.userId]
        ? h("div", { style: { ...menuItemStyle }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => { handleResetMFA(contextMenu.userId); setContextMenu(null); } }, "Reset MFA")
        : h("div", { style: { ...menuItemStyle }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => { handleEnrollMFA(contextMenu.userId); setContextMenu(null); } }, "Enroll MFA"),
      h("div", { style: { height: "1px", backgroundColor: "#555", margin: "4px 0" } }),
      h("div", { style: { ...menuItemStyle, color: "#ff6666" }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => { if (confirm("Are you sure you want to delete this user?")) deleteUser(contextMenu.userId).then(loadAll); setContextMenu(null); } }, "Delete User"),
    ) : null,
    // Group row context menu
    groupContextMenu ? h("div", {
      style: {
        position: "fixed", left: groupContextMenu.x + "px", top: groupContextMenu.y + "px",
        backgroundColor: "#2d2d2d", border: "1px solid #555", borderRadius: "4px",
        padding: "4px 0", zIndex: 1000, minWidth: "160px",
      },
      onClick: (e: MouseEvent) => e.stopPropagation(),
    },
      (() => { const g = groups.find((gr) => gr.id === groupContextMenu.groupId); return g && g.builtin; })()
        ? h("div", { style: { ...menuItemStyle, color: "#666", cursor: "default" } }, "Built-in group (read-only)")
        : [
          h("div", { style: { ...menuItemStyle }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => {
            const g = groups.find((gr) => gr.id === groupContextMenu.groupId);
            if (g) {
              const newName = prompt("Rename group:", g.name);
              if (newName && newName.trim() && newName.trim() !== g.name) {
                updateGroupName(g.id, newName.trim()).then(loadAll);
              }
            }
            setGroupContextMenu(null);
          } }, "Rename Group"),
          h("div", { style: { ...menuItemStyle }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => {
            const g = groups.find((gr) => gr.id === groupContextMenu.groupId);
            if (g) openEditGroup(g);
            setGroupContextMenu(null);
          } }, "Edit Group Membership"),
          h("div", { style: { height: "1px", backgroundColor: "#555", margin: "4px 0" } }),
          h("div", { style: { ...menuItemStyle, color: "#ff6666" }, onMouseOver: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = "#3a3a3a"; }, onMouseOut: (e: MouseEvent) => { (e.currentTarget as HTMLElement).style.backgroundColor = ""; }, onClick: () => {
            if (confirm("Are you sure you want to delete this group?")) {
              fetch(`/api/v1/ac/group/${groupContextMenu.groupId}`, { method: "DELETE", headers: authHeaders() }).then(loadAll);
            }
            setGroupContextMenu(null);
          } }, "Delete Group"),
        ]
    ) : null,
  );
}
