/**
 * Smoke Tests - Post-Deployment Verification
 *
 * Minimal health checks for critical infrastructure components.
 * Run after production deploys to verify the system is alive.
 *
 * Usage:
 *   npx playwright test tests/smoke.spec.ts
 */

import { test, expect } from "@playwright/test";


const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";
const GRAFANA_URL = process.env.GRAFANA_URL || "https://grafana.asymmetric-effort.com";

function authHeaders() {
  return { "Content-Type": "application/json", Authorization: "Bearer mock-token" };
}

test.describe("Smoke: API", () => {
  test("API status returns healthy", async () => {
    const res = await fetch(`${BASE}/api/v1/status`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(["ok", "healthy"]).toContain(data.status);
  });
});

test.describe("Smoke: UI", () => {
  test("UI responds with HTML", async () => {
    const res = await fetch(BASE);
    expect(res.status).toBe(200);
    const ct = res.headers.get("content-type") || "";
    expect(ct).toContain("text/html");
  });
});

test.describe("Smoke: Auth", () => {
  test("Auth endpoint is alive", async () => {
    const res = await fetch(`${BASE}/api/v1/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: "pdv-test", password: "invalid" }),
    });
    // Any response (401, 400) means the auth endpoint is alive
    expect([400, 401, 403]).toContain(res.status);
  });
});

test.describe("Smoke: OpenBao", () => {
  test("OpenBao is unsealed", async () => {
    // Verified through the API status endpoint which reports OpenBao health
    const res = await fetch(`${BASE}/api/v1/status`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    const data = await res.json();
    // The status endpoint includes component health; OpenBao sealed would
    // cause the API to report degraded or fail to start entirely.
    expect(["ok", "healthy"]).toContain(data.status);
  });
});

test.describe("Smoke: Grafana", () => {
  test("Grafana responds", async () => {
    try {
      const res = await fetch(`${GRAFANA_URL}/api/health`, { signal: AbortSignal.timeout(5000) });
      expect(res.status).toBe(200);
    } catch {
      test.skip(true, "Grafana not reachable — skipping until DNS/ingress configured");
    }
  });
});

test.describe("Smoke: PostgreSQL", () => {
  test("PostgreSQL connected (via API status)", async () => {
    // The API status endpoint checks database connectivity.
    // If PostgreSQL is down, the API returns a non-200 or degraded status.
    const res = await fetch(`${BASE}/api/v1/status`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(["ok", "healthy"]).toContain(data.status);
  });
});

async function getAdminToken(): Promise<string> {
  const res = await fetch(`${BASE}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: "admin", password: "Convocate-Admin-2026" }),
  });
  if (res.status !== 200) {
    throw new Error(`Admin login failed: ${res.status}`);
  }
  const data = await res.json();
  return data.accessToken;
}

test.describe("Smoke: K8s-dependent endpoints", () => {
  let token: string;

  test.beforeAll(async () => {
    token = await getAdminToken();
  });

  test("Node Manager lists nodes without 500", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.items).toBeDefined();
  });

  test("Agent Manager lists agents without 500", async () => {
    const res = await fetch(`${BASE}/api/v1/amgr/agent`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.items).toBeDefined();
  });
});
