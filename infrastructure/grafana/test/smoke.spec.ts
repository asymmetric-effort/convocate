import { test, expect } from "@playwright/test";

const GRAFANA_URL: string = process.env.GRAFANA_URL ?? "https://grafana.asymmetric-effort.com";

interface HealthResponse {
  commit: string;
  database: string;
  version: string;
}

test.describe.serial("Grafana Smoke — grafana-b production verification", () => {
  test("health check — Grafana responds with ok", async () => {
    const resp = await fetch(`${GRAFANA_URL}/api/health`);
    expect(resp.status).toBe(200);

    const body: HealthResponse = await resp.json();
    expect(body.database).toBe("ok");
    expect(body.version).toBeTruthy();
  });

  test("login page — returns 200", async () => {
    const resp = await fetch(`${GRAFANA_URL}/login`);
    expect(resp.status).toBe(200);
  });

  test("OIDC endpoint — generic_oauth is configured", async () => {
    const resp = await fetch(`${GRAFANA_URL}/login/generic_oauth`, {
      redirect: "manual",
    });
    // Should redirect to auth.asymmetric-effort.com for OIDC login
    if (resp.status === 302 || resp.status === 307) {
      const location = resp.headers.get("location") ?? "";
      expect(location).toContain("auth.asymmetric-effort.com");
    }
    // At minimum ensure it's not a 500 error
    expect(resp.status).toBeLessThan(500);
  });

  test("API requires authentication — returns 401", async () => {
    const resp = await fetch(`${GRAFANA_URL}/api/dashboards/home`);
    expect(resp.status).toBe(401);
  });
});
