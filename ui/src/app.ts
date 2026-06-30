/**
 * Convocate SPA — Main Application Entry Point
 *
 * Architecture:
 *   - UnityDesktop provides the desktop shell (system tray, background,
 *     WindowManagerProvider context).
 *   - The built-in dock is populated with all 7 applet entries so the
 *     SpecifyJS dock handles icon display, tooltips, and active indicators.
 *   - When a dock icon is clicked the internal window manager opens a window.
 *     For applets with real implementations, the onAppOpen callback renders
 *     the applet component into the window's body via a DOM portal.
 *   - Each applet is isolated: own API calls, no shared state.
 */

import { createElement, useState, useEffect, useCallback, useRef } from "@asymmetric-effort/specifyjs";
import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { UnityDesktop, UnityApp, TextField, Button, Card } from "@asymmetric-effort/specifyjs/components";
import { createRestClient } from "@asymmetric-effort/specifyjs/client";
import { NodeManager } from "./applets/node-manager";
import { AgentManager } from "./applets/agent-manager";
import { CodeIDE } from "./applets/code-ide";
import { ProjectBoard } from "./applets/project-board";

const h = createElement;

// ---------------------------------------------------------------------------
// Applet registry — dock order per SPECIFICATION.md
// Each entry defines an applet's id (matches API shortname), display label,
// icon path (served from /img/icons/ in the UI container), and an optional
// component function for implemented applets.
// ---------------------------------------------------------------------------

const APPLETS: {
  id: string;
  label: string;
  icon: string;
  component?: () => ReturnType<typeof createElement>;
}[] = [
  { id: "nmgr", label: "Node Manager", icon: "/img/icons/node-manager.png", component: NodeManager },
  { id: "amgr", label: "Agent Manager", icon: "/img/icons/agent-manager.png", component: AgentManager },
  { id: "pb", label: "Convocate Project Board", icon: "/img/icons/productboard.png", component: ProjectBoard },
  { id: "ide", label: "Code IDE", icon: "/img/icons/ide-monkey.png", component: CodeIDE },
  { id: "ac", label: "Access Control", icon: "/img/icons/access-control.png" },
  { id: "repo", label: "Repo Manager", icon: "/img/icons/repo-man.png" },
  { id: "sup", label: "Support Tool", icon: "/img/icons/support-tool.png" },
];

// ---------------------------------------------------------------------------
// REST client (shared configuration — each applet still makes independent calls)
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
        h("div", { style: { padding: "32px", textAlign: "center", backgroundColor: "#1a1a2e", color: "#e0e0e0", borderRadius: "6px" } },
          h("h1", { style: { fontSize: "24px", fontWeight: "300", marginBottom: "24px", color: "#ffffff" } }, "Convocate"),
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
// AppletPortal — injects an applet component into an open window's body
//
// When UnityDesktop opens a window via the dock, the window body contains
// generic placeholder content.  This component watches for a window with
// the matching applet title and replaces the placeholder with the real
// applet component using a secondary render root.  The MutationObserver
// ensures we re-inject after any VDOM reconciliation that would restore
// the placeholder.
// ---------------------------------------------------------------------------

function AppletPortal({ appletId, label, Component }: {
  appletId: string;
  label: string;
  Component: () => ReturnType<typeof createElement>;
}) {
  const rootRef = useRef<any>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const observerRef = useRef<MutationObserver | null>(null);

  /** Find the window body for this applet and inject the component */
  const inject = useCallback(() => {
    // Find all dialog windows, match by aria-label containing the applet label
    const windows = document.querySelectorAll(`[role="dialog"][aria-label="${label}"]`);
    if (windows.length === 0) return;

    // Use the last matching window (most recently opened)
    const win = windows[windows.length - 1];
    const body = win.querySelector(".draggable-window__body");
    if (!body) return;

    // Check if our container is already in the body
    if (containerRef.current && body.contains(containerRef.current)) return;

    // Create a container div for the applet
    const container = document.createElement("div");
    container.setAttribute("data-applet", appletId);
    container.style.width = "100%";
    container.style.height = "100%";
    container.style.overflow = "auto";

    // Replace the body contents with our container
    body.innerHTML = "";
    body.appendChild(container);
    containerRef.current = container;

    // Create a new render root and mount the applet component
    rootRef.current = createRoot(container);
    rootRef.current.render(h(Component, null));
  }, [appletId, label, Component]);

  useEffect(() => {
    // Initial injection attempt
    inject();

    // Watch for DOM changes that might remove or replace our content
    // (e.g., VDOM reconciliation restoring placeholder text)
    observerRef.current = new MutationObserver(() => {
      // If our container was removed, re-inject
      if (containerRef.current && !document.body.contains(containerRef.current)) {
        containerRef.current = null;
        rootRef.current = null;
        inject();
      }
    });

    // Observe the workspace for structural changes
    const workspace = document.querySelector(".unity-desktop__workspace");
    if (workspace) {
      observerRef.current.observe(workspace, { childList: true, subtree: true });
    }

    return () => {
      if (observerRef.current) observerRef.current.disconnect();
    };
  }, [inject]);

  // This component renders nothing itself — it uses DOM portals
  return null;
}

// ---------------------------------------------------------------------------
// Convocate Desktop — renders the Unity-style desktop with all applets
// registered in the dock.  When an implemented applet's window is opened
// the AppletPortal injects real content into the window body.
// ---------------------------------------------------------------------------

function ConvocateDesktop({ principal, onLogout }: { principal: any; onLogout: () => void }) {
  // Track which applets have been opened via the dock
  const [openApplets, setOpenApplets] = useState<Set<string>>(new Set());

  /** Called when a dock icon is clicked and a window opens */
  const handleAppOpen = useCallback((appId: string) => {
    setOpenApplets((prev) => {
      if (prev.has(appId)) return prev;
      const next = new Set(prev);
      next.add(appId);
      return next;
    });
  }, []);

  // Build portal components for implemented applets that have been opened
  const portals = APPLETS
    .filter((a) => a.component && openApplets.has(a.id))
    .map((a) =>
      h(AppletPortal, {
        key: a.id,
        appletId: a.id,
        label: a.label,
        Component: a.component!,
      })
    );

  return h(
    UnityDesktop,
    {
      apps: APPLETS.map((a) => ({ id: a.id, label: a.label, icon: a.icon })),
      user: { name: principal.name },
      onAppOpen: handleAppOpen,
      onLogout,
      theme: "dark" as const,
    },
    ...portals
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
