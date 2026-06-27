import { createElement, useState, useEffect, useRef } from "@asymmetric-effort/specifyjs";
import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { UnityDesktop, UnityApp, TextField, Button, Card } from "@asymmetric-effort/specifyjs/components";
import { createRestClient, useRest } from "@asymmetric-effort/specifyjs/client";
import type { UnityDesktopApp } from "@asymmetric-effort/specifyjs/components";

const h = createElement;

// ---------------------------------------------------------------------------
// REST client — all API calls go through this
// ---------------------------------------------------------------------------

const api = createRestClient({
  baseURL: "/api/v1",
  headers: {},
  interceptors: {
    request: [
      (config: any) => {
        const token = localStorage.getItem("accessToken");
        if (token) {
          config.headers = { ...config.headers, Authorization: `Bearer ${token}` };
        }
        return config;
      },
    ],
  },
});

// ---------------------------------------------------------------------------
// Login Screen (locked state)
// ---------------------------------------------------------------------------

function LoginScreen({ onSuccess }: { onSuccess: (principal: any) => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [mfaToken, setMfaToken] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const res = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password, mfaToken }),
      });
      if (res.status === 401) {
        setError("User login failed.");
        setLoading(false);
        return;
      }
      const session = await res.json();
      localStorage.setItem("accessToken", session.accessToken);
      onSuccess(session.principal);
    } catch {
      setError("Connection error.");
    }
    setLoading(false);
  }

  return h("div", {
    style: {
      width: "100%", height: "100vh", display: "flex",
      alignItems: "center", justifyContent: "center",
      background: "#000", color: "#fff",
    },
  },
    h("form", { onSubmit: handleSubmit, style: { width: "340px" } },
      h(Card, null,
        h("div", { style: { padding: "32px", textAlign: "center" } },
          h("h1", { style: { fontSize: "24px", fontWeight: "300", marginBottom: "24px" } }, "Convocate"),
          h(TextField, { placeholder: "Username", value: username, onChange: (v: string) => setUsername(v) }),
          h("div", { style: { height: "8px" } }),
          h(TextField, { placeholder: "Password", type: "password", value: password, onChange: (v: string) => setPassword(v) }),
          h("div", { style: { height: "8px" } }),
          h(TextField, { placeholder: "MFA Token", value: mfaToken, onChange: (v: string) => setMfaToken(v), maxLength: 6 }),
          h("div", { style: { height: "16px" } }),
          h(Button, { type: "submit" as const, variant: "primary", fullWidth: true, disabled: loading },
            loading ? "Signing in..." : "Sign In"),
          error ? h("div", { style: { color: "#e55", fontSize: "13px", marginTop: "12px" } }, error) : null,
        )
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Convocate Desktop (unlocked state)
// ---------------------------------------------------------------------------

const DOCK_APPS: UnityDesktopApp[] = [
  { id: "nmgr", icon: "/img/icons/node-manager.png", label: "Node Manager" },
  { id: "amgr", icon: "/img/icons/agent-manager.png", label: "Agent Manager" },
  { id: "pb", icon: "/img/icons/productboard.png", label: "Project Board" },
  { id: "ide", icon: "/img/icons/ide-monkey.png", label: "Code IDE" },
  { id: "ac", icon: "/img/icons/access-control.png", label: "Access Control" },
  { id: "repo", icon: "/img/icons/repo-man.png", label: "Repo Manager" },
  { id: "sup", icon: "/img/icons/support-tool.png", label: "Support Tool" },
];

function ConvocateDesktop({ principal, onLogout }: { principal: any; onLogout: () => void }) {
  const authorizedApps = DOCK_APPS.filter((app) =>
    principal.authorizedApplets.includes(app.id)
  );

  return h(UnityDesktop, {
    apps: authorizedApps,
    user: { name: principal.name },
    onLogout,
    theme: "dark" as const,
  },
    // All authorized UnityApp children are always mounted.
    // Each UnityApp registers with WindowManagerProvider on mount.
    // The dock click focuses/restores existing windows.
    authorizedApps.map((app: UnityDesktopApp) =>
      h(UnityApp, {
        key: app.id,
        id: app.id,
        title: app.label,
        icon: app.icon,
        defaultSize: { width: 900, height: 600 },
        resizable: true,
      },
        h(AppletContent, { appId: app.id, api })
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Applet content — each applet fetches data from the API
// ---------------------------------------------------------------------------

function AppletContent({ appId, api }: { appId: string; api: any }) {
  switch (appId) {
    case "nmgr": return h(NodeManagerApplet, { api });
    case "amgr": return h(AgentManagerApplet, { api });
    case "ac": return h(AccessControlApplet, { api });
    case "repo": return h(RepoManagerApplet, { api });
    case "sup": return h(SupportToolApplet, { api });
    case "ide": return h("div", { style: { padding: "16px", color: "#888" } }, "Code IDE — coming soon");
    case "pb": return h("div", { style: { padding: "16px", color: "#888" } }, "Project Board — coming soon");
    default: return h("div", { style: { padding: "16px" } }, `Unknown applet: ${appId}`);
  }
}

// ---------------------------------------------------------------------------
// Node Manager — DataGrid of K8s nodes from API
// ---------------------------------------------------------------------------

function NodeManagerApplet({ api }: { api: any }) {
  const { data, loading, error } = useRest(api, "/nmgr/node?limit=100");
  if (loading) return h("div", { style: { padding: "16px", color: "#888" } }, "Loading nodes...");
  if (error) return h("div", { style: { padding: "16px", color: "#e55" } }, `Error: ${error.message}`);
  const nodes = data?.items || [];
  return h("div", { style: { padding: "8px", fontSize: "13px" } },
    h("div", { style: { marginBottom: "8px", color: "#888" } }, `${data?.total || 0} nodes`),
    h("table", { style: { width: "100%", borderCollapse: "collapse" } },
      h("thead", null,
        h("tr", { style: { borderBottom: "1px solid #444", textAlign: "left", fontSize: "11px", color: "#aaa", textTransform: "uppercase" } },
          h("th", { style: { padding: "6px" } }, "Name"),
          h("th", { style: { padding: "6px" } }, "IP"),
          h("th", { style: { padding: "6px" } }, "Status"),
          h("th", { style: { padding: "6px" } }, "Agents"),
          h("th", { style: { padding: "6px" } }, "Memory"),
        )
      ),
      h("tbody", null,
        ...nodes.map((n: any, i: number) =>
          h("tr", { key: n.id, style: { borderBottom: "1px solid #333", background: i % 2 === 0 ? "rgba(255,255,255,0.02)" : "transparent" } },
            h("td", { style: { padding: "6px", color: "#7eb8da", fontFamily: "monospace" } }, n.id),
            h("td", { style: { padding: "6px" } }, n.ip),
            h("td", { style: { padding: "6px" } }, n.status),
            h("td", { style: { padding: "6px" } }, String(n.agents)),
            h("td", { style: { padding: "6px" } }, `${n.memUsedGB?.toFixed(1)} / ${n.memTotalGB?.toFixed(0)} GB`),
          )
        )
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Agent Manager
// ---------------------------------------------------------------------------

function AgentManagerApplet({ api }: { api: any }) {
  const { data, loading } = useRest(api, "/amgr/agent?limit=200");
  if (loading) return h("div", { style: { padding: "16px", color: "#888" } }, "Loading agents...");
  const agents = data?.items || [];
  if (agents.length === 0) {
    return h("div", { style: { padding: "16px", color: "#888" } },
      `No agent-containers running at ${new Date().toLocaleString()}`);
  }
  return h("div", { style: { padding: "8px", fontSize: "13px" } },
    h("div", { style: { marginBottom: "8px", color: "#888" } }, `${data?.total || 0} agents`),
    ...agents.map((a: any) =>
      h("div", { key: a.id, style: { padding: "8px", borderBottom: "1px solid #333" } },
        h("span", { style: { color: "#7eb8da", fontFamily: "monospace" } }, a.id),
        " — ", a.project || "no project", " — ", a.status)
    )
  );
}

// ---------------------------------------------------------------------------
// Access Control
// ---------------------------------------------------------------------------

function AccessControlApplet({ api }: { api: any }) {
  const users = useRest(api, "/ac/user?limit=200");
  const groups = useRest(api, "/ac/group?limit=200");
  const settings = useRest(api, "/ac/settings");

  if (users.loading) return h("div", { style: { padding: "16px", color: "#888" } }, "Loading...");

  return h("div", { style: { padding: "8px", fontSize: "13px" } },
    h("h3", null, "Users"),
    ...(users.data?.items || []).map((u: any) =>
      h("div", { key: u.id, style: { padding: "4px 0", borderBottom: "1px solid #333" } },
        `${u.name} — ${u.email} — ${u.status}`)
    ),
    h("h3", { style: { marginTop: "16px" } }, "Groups"),
    ...(groups.data?.items || []).map((g: any) =>
      h("div", { key: g.id, style: { padding: "4px 0", borderBottom: "1px solid #333" } },
        `${g.name} (${g.builtin ? "built-in" : "custom"}) — ${(g.roles || []).join(", ")}`)
    ),
    h("h3", { style: { marginTop: "16px" } }, "Settings"),
    settings.data ? h("div", null,
      h("div", null, `Session timeout: ${settings.data.sessionTimeoutMinutes} min`),
      h("div", null, `Require MFA: ${settings.data.requireMfa}`),
      h("div", null, `Min password length: ${settings.data.passwordMinLength}`),
    ) : null,
  );
}

// ---------------------------------------------------------------------------
// Repo Manager
// ---------------------------------------------------------------------------

function RepoManagerApplet({ api }: { api: any }) {
  const { data, loading } = useRest(api, "/repo/repo?limit=200");
  if (loading) return h("div", { style: { padding: "16px", color: "#888" } }, "Loading repos...");
  return h("div", { style: { padding: "8px", fontSize: "13px" } },
    h("div", { style: { marginBottom: "8px", color: "#888" } }, `${data?.total || 0} repositories`),
    ...(data?.items || []).map((r: any) =>
      h("div", { key: r.id, style: { padding: "6px 0", borderBottom: "1px solid #333" } },
        h("span", { style: { fontWeight: "600" } }, r.name),
        ` — ${r.description || ""} — ${r.visibility}`)
    )
  );
}

// ---------------------------------------------------------------------------
// Support Tool
// ---------------------------------------------------------------------------

function SupportToolApplet({ api }: { api: any }) {
  const { data, loading } = useRest(api, "/sup/ticket?limit=200");
  if (loading) return h("div", { style: { padding: "16px", color: "#888" } }, "Loading tickets...");
  return h("div", { style: { padding: "8px", fontSize: "13px" } },
    h("div", { style: { marginBottom: "8px", color: "#888" } }, `${data?.total || 0} tickets`),
    ...(data?.items || []).map((t: any) =>
      h("div", { key: t.id, style: { padding: "6px 0", borderBottom: "1px solid #333" } },
        h("span", { style: { fontFamily: "monospace", color: "#7eb8da" } }, t.id),
        ` — ${t.subject} — ${t.priority} — ${t.status}`)
    )
  );
}

// ---------------------------------------------------------------------------
// Root App — manages locked/unlocked state
// ---------------------------------------------------------------------------

function App() {
  const [principal, setPrincipal] = useState<any>(null);

  useEffect(() => {
    const token = localStorage.getItem("accessToken");
    if (token) {
      fetch("/api/v1/auth/me", { headers: { Authorization: `Bearer ${token}` } })
        .then((r) => r.ok ? r.json() : Promise.reject())
        .then((p) => setPrincipal(p))
        .catch(() => { localStorage.removeItem("accessToken"); });
    }
  }, []);

  function handleLogout() {
    localStorage.removeItem("accessToken");
    setPrincipal(null);
  }

  if (!principal) {
    return h(LoginScreen, { onSuccess: (p: any) => setPrincipal(p) });
  }

  return h(ConvocateDesktop, { principal, onLogout: handleLogout });
}

// ---------------------------------------------------------------------------
// Mount
// ---------------------------------------------------------------------------

const container = document.getElementById("app");
if (container) {
  createRoot(container).render(h(App, null));
}
