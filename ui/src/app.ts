import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { UnityDesktop, UnityApp, TextField, Button, Card, Modal } from "@asymmetric-effort/specifyjs/components";
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
// Node Manager — fetches from API, renders node list with provision dialog
// ---------------------------------------------------------------------------

function NodeManagerApplet() {
  const { data, loading, error, refetch } = useRest(api, "/nmgr/node?limit=100");
  const [showProvision, setShowProvision] = useState(false);
  const [provHost, setProvHost] = useState("");
  const [provUser, setProvUser] = useState("");
  const [provPassword, setProvPassword] = useState("");
  const [provLocation, setProvLocation] = useState("");
  const [provError, setProvError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [liveNodes, setLiveNodes] = useState<any[] | null>(null);

  // Subscribe to nmgr/status SSE for real-time metric and status updates.
  // Auth token is passed as query param since EventSource cannot send headers.
  useEffect(() => {
    const token = localStorage.getItem("accessToken");
    if (!token) return;
    const es = new EventSource(`/api/v1/events/nmgr/status?token=${encodeURIComponent(token)}`);
    es.onmessage = (msg) => {
      try {
        const evt = JSON.parse(msg.data);
        if (evt.type === "node.metrics") {
          setLiveNodes(evt.payload);
        } else if (evt.type === "node.ready" || evt.type === "node.error" || evt.type === "node.pending") {
          refetch();
        }
      } catch { /* ignore parse errors */ }
    };
    return () => es.close();
  }, []);

  if (loading) return h("div", { style: { padding: "16px", color: "#888" } }, "Loading nodes...");
  if (error) return h("div", { style: { padding: "16px", color: "#e55" } }, `Error: ${error.message}`);

  // Merge live metrics (from SSE) over REST data for real-time updates.
  const apiNodes: any[] = data?.items || [];
  const nodes = apiNodes.map((n: any) => {
    if (!liveNodes) return n;
    const live = liveNodes.find((ln: any) => ln.id === n.id);
    return live ? { ...n, ...live } : n;
  });

  function resetForm() {
    setProvHost("");
    setProvUser("");
    setProvPassword("");
    setProvLocation("");
    setProvError("");
    setSubmitting(false);
  }

  async function handleProvision(e: Event) {
    e.preventDefault();
    setProvError("");

    if (!provHost.trim()) { setProvError("Host is required"); return; }
    if (!provUser.trim()) { setProvError("SSH User is required"); return; }

    setSubmitting(true);
    try {
      const token = localStorage.getItem("accessToken");
      const res = await fetch("/api/v1/nmgr/node", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({
          host: provHost.trim(),
          user: provUser.trim(),
          password: provPassword || undefined,
          location: provLocation || undefined,
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setProvError(body?.message || `Request failed (${res.status})`);
        setSubmitting(false);
        return;
      }
      resetForm();
      setShowProvision(false);
      // Refetch to pick up the pending node from the API
      refetch();
    } catch {
      setProvError("Connection error");
      setSubmitting(false);
    }
  }

  const statusColor = (s: string) => {
    if (s === "Ready") return "#4a4";
    if (s === "Pending") return "#da5";
    if (s === "NotReady") return "#e55";
    if (s === "SchedulingDisabled") return "#888";
    return "#ccc";
  };

  return h("div", { style: { padding: "8px", fontSize: "13px" } },
    // Toolbar
    h("div", { style: { display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "8px" } },
      h("div", { style: { color: "#888" } }, `${nodes.length} nodes`),
      h(Button, { variant: "primary", onClick: () => setShowProvision(true) }, "Provision Node"),
    ),
    // Node table
    h("table", { style: { width: "100%", borderCollapse: "collapse" } },
      h("thead", null,
        h("tr", { style: { borderBottom: "1px solid #444", textAlign: "left", fontSize: "11px", color: "#aaa", textTransform: "uppercase" } },
          h("th", { style: { padding: "6px" } }, "Name"),
          h("th", { style: { padding: "6px" } }, "IP"),
          h("th", { style: { padding: "6px" } }, "Status"),
          h("th", { style: { padding: "6px" } }, "Agents"),
          h("th", { style: { padding: "6px" } }, "Load Avg"),
          h("th", { style: { padding: "6px" } }, "Memory"),
          h("th", { style: { padding: "6px" } }, "Disk"),
          h("th", { style: { padding: "6px" } }, "Actions"),
        )
      ),
      h("tbody", null,
        ...nodes.map((n: any, i: number) =>
          h("tr", { key: n.id, style: { borderBottom: "1px solid #333", background: i % 2 === 0 ? "rgba(255,255,255,0.02)" : "transparent" } },
            h("td", { style: { padding: "6px", color: "#7eb8da", fontFamily: "monospace" } }, n.id),
            h("td", { style: { padding: "6px" } }, n.ip),
            h("td", { style: { padding: "6px", color: statusColor(n.status) } }, n.status),
            h("td", { style: { padding: "6px" } }, String(n.agents ?? 0)),
            h("td", { style: { padding: "6px", fontFamily: "monospace" } },
              n.loadAvg ? `${n.loadAvg.one?.toFixed(2)} / ${n.loadAvg.five?.toFixed(2)} / ${n.loadAvg.fifteen?.toFixed(2)}` : "--"),
            h("td", { style: { padding: "6px" } },
              n.memTotalGB ? `${n.memUsedGB?.toFixed(1)} / ${n.memTotalGB?.toFixed(0)} GB` : "--"),
            h("td", { style: { padding: "6px" } },
              n.diskTotalGB ? `${n.diskUsedGB?.toFixed(1)} / ${n.diskTotalGB?.toFixed(0)} GB` : "--"),
            h("td", { style: { padding: "6px" } },
              n.status === "Pending" ? h("div", { style: { color: "#888", fontSize: "11px" } }, "Provisioning...") :
              h("div", { style: { display: "flex", gap: "4px" } },
                h(Button, {
                  variant: "secondary" as const,
                  size: "sm" as const,
                  onClick: async () => {
                    const token = localStorage.getItem("accessToken");
                    const endpoint = n.status === "SchedulingDisabled" ? "start" : "stop";
                    await fetch(`/api/v1/nmgr/node/${n.id}/${endpoint}`, {
                      method: "POST",
                      headers: token ? { Authorization: `Bearer ${token}` } : {},
                    });
                    refetch();
                  },
                }, n.status === "SchedulingDisabled" ? "Uncordon" : "Cordon"),
                h(Button, {
                  variant: "danger" as const,
                  size: "sm" as const,
                  onClick: async () => {
                    const token = localStorage.getItem("accessToken");
                    await fetch(`/api/v1/nmgr/node/${n.id}`, {
                      method: "DELETE",
                      headers: token ? { Authorization: `Bearer ${token}` } : {},
                    });
                    refetch();
                  },
                }, "Delete"),
              ),
            ),
          )
        )
      )
    ),
    // Provision Node Modal
    h(Modal, {
      open: showProvision,
      onClose: () => { resetForm(); setShowProvision(false); },
      title: "Provision Node",
      size: "sm" as const,
    },
      h("form", { onSubmit: handleProvision, style: { display: "flex", flexDirection: "column", gap: "12px" } },
        h("label", { style: { color: "#ccc", fontSize: "12px" } }, "Host",
          h(TextField, { placeholder: "IPv4, IPv6, or FQDN", value: provHost, onChange: (v: string) => setProvHost(v) })),
        h("label", { style: { color: "#ccc", fontSize: "12px" } }, "SSH User",
          h(TextField, { placeholder: "Linux username", value: provUser, onChange: (v: string) => setProvUser(v) })),
        h("label", { style: { color: "#ccc", fontSize: "12px" } }, "Password",
          h(TextField, { placeholder: "Optional", type: "password", value: provPassword, onChange: (v: string) => setProvPassword(v) })),
        h("label", { style: { color: "#ccc", fontSize: "12px" } }, "Location",
          h(TextField, { placeholder: "e.g. us-east-1", value: provLocation, onChange: (v: string) => setProvLocation(v) })),
        provError ? h("div", { style: { color: "#e55", fontSize: "12px" } }, provError) : null,
        h("div", { style: { display: "flex", justifyContent: "flex-end", gap: "8px", marginTop: "4px" } },
          h(Button, { variant: "secondary" as const, onClick: () => { resetForm(); setShowProvision(false); } }, "Cancel"),
          h(Button, { type: "submit" as const, variant: "primary" as const, disabled: submitting },
            submitting ? "Provisioning..." : "Provision"),
        ),
      ),
    ),
  );
}

// ---------------------------------------------------------------------------
// Convocate Desktop (unlocked state) — only Node Manager for now
// ---------------------------------------------------------------------------

const DOCK_APPS: UnityDesktopApp[] = [
  { id: "nmgr", icon: "/img/icons/node-manager.png", label: "Node Manager" },
  { id: "amgr", icon: "/img/icons/agent-manager.png", label: "Agent Manager" },
  { id: "pb", icon: "/img/icons/productboard.png", label: "Convocate Project Board" },
  { id: "ide", icon: "/img/icons/ide-monkey.png", label: "Code IDE" },
  { id: "ac", icon: "/img/icons/access-control.png", label: "Access Control" },
  { id: "repo", icon: "/img/icons/repo-man.png", label: "Repo Manager" },
  { id: "sup", icon: "/img/icons/support-tool.png", label: "Support Tool" },
];

function ConvocateDesktop({ principal, onLogout }: { principal: any; onLogout: () => void }) {
  return h(UnityDesktop, {
    apps: DOCK_APPS,
    user: { name: principal.name },
    onLogout,
    theme: "dark" as const,
  });
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
