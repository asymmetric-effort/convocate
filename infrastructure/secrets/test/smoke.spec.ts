import { test, expect } from "@playwright/test";

const OPENBAO_URL: string = process.env.OPENBAO_URL ?? "http://192.168.3.161:8200";
const ROOT_TOKEN: string = process.env.OPENBAO_ROOT_TOKEN ?? "";

interface HealthResponse {
  initialized: boolean;
  sealed: boolean;
  standby: boolean;
  version: string;
}

interface AuthMethodsResponse {
  [key: string]: {
    type: string;
    description: string;
    accessor: string;
  };
}

interface PoliciesListResponse {
  data: {
    policies: string[];
    keys: string[];
  };
}

interface TokenLookupResponse {
  data: {
    accessor: string;
    creation_time: number;
    display_name: string;
    id: string;
    policies: string[];
    ttl: number;
  };
}

function headers(token: string): Record<string, string> {
  return {
    "X-Vault-Token": token,
    "Content-Type": "application/json",
  };
}

test.describe("OpenBao Smoke — secrets-b read-only verification", () => {
  test("health check — GET /v1/sys/health", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/health`);
    expect(resp.status).toBe(200);

    const body: HealthResponse = await resp.json();
    expect(body.initialized).toBe(true);
    expect(body.sealed).toBe(false);
  });

  test("list auth methods — GET /v1/sys/auth", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/auth`, {
      headers: headers(ROOT_TOKEN),
    });
    expect(resp.status).toBe(200);

    const body: AuthMethodsResponse = await resp.json();
    const authTypes: string[] = Object.values(body)
      .filter((v): v is { type: string; description: string; accessor: string } =>
        typeof v === "object" && v !== null && "type" in v
      )
      .map((v) => v.type);
    expect(authTypes).toContain("userpass");
  });

  test("list policies — GET /v1/sys/policies/acl", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/policies/acl`, {
      method: "GET",
      headers: headers(ROOT_TOKEN),
    });
    expect(resp.status).toBe(200);

    const body: PoliciesListResponse = await resp.json();
    const policies: string[] = body.data?.policies ?? body.data?.keys ?? [];
    expect(policies).toContain("admin-policy");
  });

  test("token self lookup — GET /v1/auth/token/lookup-self", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/auth/token/lookup-self`, {
      headers: headers(ROOT_TOKEN),
    });
    expect(resp.status).toBe(200);

    const body: TokenLookupResponse = await resp.json();
    expect(body.data).toBeTruthy();
    expect(body.data.policies).toContain("root");
  });
});
