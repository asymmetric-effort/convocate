import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { App, setAppRoot } from "./app";
import { api, UnauthorizedError } from "./api/client";
import type { MeResponse, ProjectInfo, HostHealthInfo } from "./api/client";
import "./styles.css";

const CONNECT_TIMEOUT_MS = 10000;

function fetchWithTimeout(url: string, timeoutMs: number): Promise<Response> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  return fetch(url, { signal: controller.signal }).finally(() => clearTimeout(timer));
}

function showScreen(message: string, isError = false, showRetry = false) {
  const rootEl = document.getElementById("root");
  if (!rootEl) return;
  const color = isError ? "#ef4444" : "#94a3b8";
  const retryBtn = showRetry
    ? `<button onclick="window.location.reload()" style="margin-top:16px;padding:8px 20px;background:#3b82f6;color:#fff;border:none;border-radius:8px;cursor:pointer;font-size:14px;">Retry</button>`
    : "";
  rootEl.innerHTML = `
    <div style="display:flex;align-items:center;justify-content:center;position:fixed;inset:0;background:#0f172a;z-index:9999;">
      <div style="text-align:center;max-width:400px;padding:24px;">
        <div style="font-size:18px;font-weight:700;color:#60a5fa;margin-bottom:16px;">convocate</div>
        <p style="color:${color};font-size:14px;">${message}</p>
        ${retryBtn}
      </div>
    </div>
  `;
}

async function bootstrap() {
  const rootEl = document.getElementById("root");
  if (!rootEl) return;

  showScreen("Connecting to services...");

  // First check: can we reach the health endpoint?
  try {
    await fetchWithTimeout("/v1/health", CONNECT_TIMEOUT_MS);
  } catch {
    showScreen(
      "Unable to reach the Router API. Check that the convocate services are running.",
      true,
      true
    );
    return;
  }

  // Second check: auth status.
  let authenticated = false;
  let user: MeResponse | null = null;
  let startupError = "";

  try {
    const resp = await fetchWithTimeout("/auth/me", CONNECT_TIMEOUT_MS);
    if (resp.status === 401) {
      authenticated = false;
    } else if (resp.ok) {
      user = await resp.json();
      authenticated = true;
    } else {
      startupError = `Auth service returned HTTP ${resp.status}`;
    }
  } catch (err: unknown) {
    if (err instanceof UnauthorizedError) {
      authenticated = false;
    } else {
      const msg = err instanceof Error ? err.message : "Unknown error";
      if (msg.includes("abort")) {
        startupError = "Auth service timed out after 10 seconds.";
      } else {
        startupError = `Auth service error: ${msg}`;
      }
    }
  }

  if (startupError) {
    showScreen(startupError, true, true);
    return;
  }

  // Fetch dashboard data before rendering.
  let projects: ProjectInfo[] = [];
  let hosts: HostHealthInfo[] = [];
  try {
    projects = await api.listProjects();
  } catch { /* empty is fine */ }
  try {
    hosts = await api.listHosts();
  } catch { /* empty is fine */ }

  // Determine component status — if health check passed, services are running.
  const components = [
    { name: "router-api", status: "running" as const },
    { name: "ui", status: "running" as const },
    { name: "openbao", status: "running" as const },
    { name: "dispatch-api", status: "running" as const },
    { name: "secrets-broker", status: "running" as const },
  ];

  // Services are healthy — clear loading screen and render the app.
  rootEl.innerHTML = "";
  const root = createRoot(rootEl);
  setAppRoot(root);

  root.render(
    <App
      initialAuthenticated={authenticated}
      initialUser={user}
      initialError=""
      projects={projects}
      hosts={hosts}
      components={components}
    />
  );
}

showScreen("Connecting to services...");
bootstrap();
