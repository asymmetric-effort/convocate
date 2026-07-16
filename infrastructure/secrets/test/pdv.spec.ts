import { test, expect } from "@playwright/test";

const OPENBAO_URL: string = process.env.OPENBAO_URL ?? "https://192.168.3.160:8200";
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
    metadata: Record<string, string>;
  };
}

interface KvReadResponse {
  data: {
    data: Record<string, string>;
    metadata: {
      version: number;
    };
  };
}

let pdvToken: string = "";

function headers(token: string): Record<string, string> {
  return {
    "X-Vault-Token": token,
    "Content-Type": "application/json",
  };
}

test.describe.serial("OpenBao PDV — full CRUD as pdv-test user", () => {
  test("health check — initialized and unsealed", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/health`);
    expect(resp.status).toBe(200);

    const body: HealthResponse = await resp.json();
    expect(body.initialized).toBe(true);
    expect(body.sealed).toBe(false);
  });

  test("login as pdv-test — POST /v1/auth/userpass/login/pdv-test", async () => {
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

  test("create KV secret — POST /v1/secret/data/pdv-test/test-key", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/test-key`, {
      method: "POST",
      headers: headers(pdvToken),
      body: JSON.stringify({ data: { value: "pdv-test-value" } }),
    });
    expect(resp.status).toBe(200);
  });

  test("read KV secret — GET /v1/secret/data/pdv-test/test-key", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/test-key`, {
      headers: headers(pdvToken),
    });
    expect(resp.status).toBe(200);

    const body: KvReadResponse = await resp.json();
    expect(body.data.data.value).toBe("pdv-test-value");
  });

  test("update KV secret — POST /v1/secret/data/pdv-test/test-key", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/test-key`, {
      method: "POST",
      headers: headers(pdvToken),
      body: JSON.stringify({ data: { value: "pdv-test-updated" } }),
    });
    expect(resp.status).toBe(200);
  });

  test("read updated secret — verify new value", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/test-key`, {
      headers: headers(pdvToken),
    });
    expect(resp.status).toBe(200);

    const body: KvReadResponse = await resp.json();
    expect(body.data.data.value).toBe("pdv-test-updated");
  });

  test("delete KV secret — DELETE data and metadata", async () => {
    // Delete data (soft delete)
    const dataResp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/test-key`, {
      method: "DELETE",
      headers: headers(pdvToken),
    });
    expect(dataResp.status).toBe(204);

    // Delete metadata (permanent cleanup)
    const metaResp = await fetch(`${OPENBAO_URL}/v1/secret/metadata/pdv-test/test-key`, {
      method: "DELETE",
      headers: headers(pdvToken),
    });
    expect(metaResp.status).toBe(204);
  });

  test("verify deleted — GET returns 404", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/pdv-test/test-key`, {
      headers: headers(pdvToken),
    });
    expect(resp.status).toBe(404);
  });
});
