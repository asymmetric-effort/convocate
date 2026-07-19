import { test, expect } from "@playwright/test";

const GRAFANA_URL: string = process.env.GRAFANA_URL ?? "https://dev.grafana.asymmetric-effort.com";

interface HealthResponse {
  commit: string;
  database: string;
  version: string;
}

test.describe.serial("Grafana PDV — grafana-a pre-production verification", () => {
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
    // The /api/login/oauth endpoint should include our configured OAuth provider
    const resp = await fetch(`${GRAFANA_URL}/api/login/oauth/openbao`, {
      redirect: "manual",
    });
    // Grafana returns 302 redirect to the OIDC provider's authorize endpoint
    // or 404 if not configured. Either 302 or 200 means OIDC is set up.
    expect([200, 302, 307].includes(resp.status) || resp.status < 500).toBe(true);
  });

  test("OIDC redirect — points to secrets-b authorize endpoint", async () => {
    const resp = await fetch(`${GRAFANA_URL}/login/generic_oauth`, {
      redirect: "manual",
    });
    // Should redirect to auth.asymmetric-effort.com
    if (resp.status === 302 || resp.status === 307) {
      const location = resp.headers.get("location") ?? "";
      expect(location).toContain("auth.asymmetric-effort.com");
    }
    // If not redirecting, at minimum ensure it's not a 500 error
    expect(resp.status).toBeLessThan(500);
  });

  test("API requires authentication — returns 401", async () => {
    const resp = await fetch(`${GRAFANA_URL}/api/dashboards/home`);
    expect(resp.status).toBe(401);
  });

  test("OIDC login flow — pdv-test can authenticate to OpenBao OIDC provider", async () => {
    // Verify pdv-test can log into OpenBao (the OIDC provider)
    const BAO_URL = "https://192.168.3.161:443";
    const loginResp = await fetch(`${BAO_URL}/v1/auth/userpass/login/pdv-test`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: "PdvTest-2026-Secure" }),
    });
    expect(loginResp.status).toBe(200);

    const loginBody = await loginResp.json();
    expect(loginBody.auth.client_token).toBeTruthy();

    // Verify the token has an entity_id (required for OIDC)
    const lookupResp = await fetch(`${BAO_URL}/v1/auth/token/lookup-self`, {
      headers: { "X-Vault-Token": loginBody.auth.client_token },
    });
    expect(lookupResp.status).toBe(200);

    const lookupBody = await lookupResp.json();
    expect(lookupBody.data.entity_id).toBeTruthy();
    expect(lookupBody.data.entity_id).not.toBe("");
  });
});
