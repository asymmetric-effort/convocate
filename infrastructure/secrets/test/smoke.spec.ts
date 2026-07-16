import { test, expect } from "@playwright/test";

const OPENBAO_URL: string = process.env.OPENBAO_URL ?? "http://192.168.3.161:8200";
const PDV_USER = "pdv-test";
const PDV_PASS = "PdvTest-2026-Secure";

interface HealthResponse {
  initialized: boolean;
  sealed: boolean;
  version: string;
}

interface AuthLoginResponse {
  auth: {
    client_token: string;
    policies: string[];
  };
}

let pdvToken: string = "";

test.describe.serial("OpenBao Smoke — secrets-b read-only verification", () => {
  test("health check — initialized and unsealed", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/health`);
    expect(resp.status).toBe(200);

    const body: HealthResponse = await resp.json();
    expect(body.initialized).toBe(true);
    expect(body.sealed).toBe(false);
  });

  test("login as pdv-test succeeds", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/auth/userpass/login/${PDV_USER}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: PDV_PASS }),
    });
    expect(resp.status).toBe(200);

    const body: AuthLoginResponse = await resp.json();
    expect(body.auth.client_token).toBeTruthy();
    expect(body.auth.policies).toContain("pdv-test-policy");
    pdvToken = body.auth.client_token;
  });

  test("token self-lookup succeeds", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/auth/token/lookup-self`, {
      headers: { "X-Vault-Token": pdvToken },
    });
    expect(resp.status).toBe(200);
  });

  test("pdv-test namespace is accessible", async () => {
    // Write a canary, read it, delete it — proves the namespace works
    // This is non-destructive to production data (pdv-test namespace only)
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/smoke-canary`, {
      method: "POST",
      headers: { "X-Vault-Token": pdvToken, "Content-Type": "application/json" },
      body: JSON.stringify({ data: { smoke: "ok" } }),
    });
    expect(resp.status).toBe(200);

    // Read it back
    const readResp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/smoke-canary`, {
      headers: { "X-Vault-Token": pdvToken },
    });
    expect(readResp.status).toBe(200);

    // Clean up
    await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/smoke-canary`, {
      method: "DELETE",
      headers: { "X-Vault-Token": pdvToken },
    });
  });
});
