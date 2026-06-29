import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { UnityDesktop, TextField, Button, Card } from "@asymmetric-effort/specifyjs/components";
import { createRestClient } from "@asymmetric-effort/specifyjs/client";

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
// Convocate Desktop
// ---------------------------------------------------------------------------

function ConvocateDesktop({ principal, onLogout }: { principal: any; onLogout: () => void }) {
  return h(UnityDesktop, {
    apps: [],
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
