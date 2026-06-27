import { createElement, useState, useEffect, useRef } from "@asymmetric-effort/specifyjs";
import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { UnityDesktop, UnityApp, TextField, Button, Card } from "@asymmetric-effort/specifyjs/components";
import { createRestClient, useRest } from "@asymmetric-effort/specifyjs/client";
import type { UnityDesktopApp } from "@asymmetric-effort/specifyjs/components";

const h = createElement;

// ---------------------------------------------------------------------------
// REST client
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
// Node Manager — fetches from API, renders node list
// ---------------------------------------------------------------------------

function NodeManagerApplet() {
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
// Convocate Desktop (unlocked state) — only Node Manager for now
// ---------------------------------------------------------------------------

const DOCK_APPS: UnityDesktopApp[] = [
  { id: "nmgr", icon: "/img/icons/node-manager.png", label: "Node Manager" },
];

function ConvocateDesktop({ principal, onLogout }: { principal: any; onLogout: () => void }) {
  const [openApps, setOpenApps] = useState<string[]>([]);

  function handleAppOpen(appId: string) {
    setOpenApps((prev: string[]) => {
      if (prev.includes(appId)) return prev;
      return [...prev, appId];
    });
  }

  function handleAppClose(appId: string) {
    setOpenApps((prev: string[]) => prev.filter((id: string) => id !== appId));
  }

  return h(UnityDesktop, {
    apps: DOCK_APPS,
    user: { name: principal.name },
    onAppOpen: handleAppOpen,
    onLogout,
    theme: "dark" as const,
  },
    openApps.map((appId: string) =>
      h(UnityApp, {
        key: appId,
        id: appId,
        title: "Node Manager",
        icon: "/img/icons/node-manager.png",
        defaultSize: { width: 900, height: 600 },
        resizable: true,
        onClose: () => handleAppClose(appId),
      },
        h(NodeManagerApplet, null)
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Root App
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
