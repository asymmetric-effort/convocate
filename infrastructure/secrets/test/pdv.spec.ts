import { test, expect } from "@playwright/test";

const OPENBAO_URL: string = process.env.OPENBAO_URL ?? "http://192.168.3.160:8200";
const ROOT_TOKEN: string = process.env.OPENBAO_ROOT_TOKEN ?? "";

interface HealthResponse {
  initialized: boolean;
  sealed: boolean;
  standby: boolean;
  performance_standby: boolean;
  replication_performance_mode: string;
  replication_dr_mode: string;
  server_time_utc: number;
  version: string;
}

interface InitResponse {
  keys: string[];
  keys_base64: string[];
  root_token: string;
}

interface UnsealResponse {
  sealed: boolean;
  t: number;
  n: number;
  progress: number;
}

interface AuthLoginResponse {
  auth: {
    client_token: string;
    accessor: string;
    policies: string[];
    token_policies: string[];
    metadata: Record<string, string>;
    lease_duration: number;
    renewable: boolean;
  };
}

interface KvReadResponse {
  data: {
    data: Record<string, string>;
    metadata: {
      created_time: string;
      deletion_time: string;
      destroyed: boolean;
      version: number;
    };
  };
}

interface PolicyReadResponse {
  data: {
    name?: string;
    policy: string;
    rules?: string;
  };
}

let rootToken: string = ROOT_TOKEN;
let unsealKeys: string[] = [];

function headers(token: string): Record<string, string> {
  return {
    "X-Vault-Token": token,
    "Content-Type": "application/json",
  };
}

test.describe.serial("OpenBao PDV — secrets-a full CRUD", () => {
  test("health check — GET /v1/sys/health", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/health`, {
      method: "GET",
    });
    // Health endpoint returns 200 (initialized+unsealed), 429 (standby),
    // 501 (not initialized), or 503 (sealed)
    expect([200, 429, 501, 503]).toContain(resp.status);

    const body: HealthResponse = await resp.json();

    if (resp.status === 200) {
      expect(body.initialized).toBe(true);
      expect(body.sealed).toBe(false);
    }
  });

  test("init if needed — POST /v1/sys/init", async () => {
    const healthResp = await fetch(`${OPENBAO_URL}/v1/sys/health`);
    const health: HealthResponse = await healthResp.json();

    if (health.initialized) {
      test.skip();
      return;
    }

    const resp = await fetch(`${OPENBAO_URL}/v1/sys/init`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        secret_shares: 1,
        secret_threshold: 1,
      }),
    });
    expect(resp.status).toBe(200);

    const body: InitResponse = await resp.json();
    expect(body.keys).toHaveLength(1);
    expect(body.root_token).toBeTruthy();

    unsealKeys = body.keys;
    rootToken = body.root_token;
  });

  test("unseal if needed — POST /v1/sys/unseal", async () => {
    const healthResp = await fetch(`${OPENBAO_URL}/v1/sys/health`);

    if (healthResp.status === 200) {
      test.skip();
      return;
    }

    if (unsealKeys.length === 0) {
      throw new Error("No unseal keys available — was init skipped without a root token?");
    }

    const resp = await fetch(`${OPENBAO_URL}/v1/sys/unseal`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: unsealKeys[0] }),
    });
    expect(resp.status).toBe(200);

    const body: UnsealResponse = await resp.json();
    expect(body.sealed).toBe(false);
  });

  test("enable userpass auth if needed", async () => {
    const listResp = await fetch(`${OPENBAO_URL}/v1/sys/auth`, {
      headers: headers(rootToken),
    });
    const authMethods = await listResp.json();

    if (authMethods["userpass/"]) {
      test.skip();
      return;
    }

    const resp = await fetch(`${OPENBAO_URL}/v1/sys/auth/userpass`, {
      method: "POST",
      headers: headers(rootToken),
      body: JSON.stringify({ type: "userpass" }),
    });
    expect(resp.status).toBe(204);
  });

  test("enable kv secrets engine if needed", async () => {
    const listResp = await fetch(`${OPENBAO_URL}/v1/sys/mounts`, {
      headers: headers(rootToken),
    });
    const mounts = await listResp.json();

    if (mounts["secret/"]) {
      test.skip();
      return;
    }

    const resp = await fetch(`${OPENBAO_URL}/v1/sys/mounts/secret`, {
      method: "POST",
      headers: headers(rootToken),
      body: JSON.stringify({ type: "kv", options: { version: "2" } }),
    });
    expect(resp.status).toBe(204);
  });

  test("create userpass user — POST /v1/auth/userpass/users/test-user", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/auth/userpass/users/test-user`, {
      method: "POST",
      headers: headers(rootToken),
      body: JSON.stringify({
        password: "test-password-pdv",
        policies: "default",
      }),
    });
    expect(resp.status).toBe(204);
  });

  test("login as test user — POST /v1/auth/userpass/login/test-user", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/auth/userpass/login/test-user`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: "test-password-pdv" }),
    });
    expect(resp.status).toBe(200);

    const body: AuthLoginResponse = await resp.json();
    expect(body.auth.client_token).toBeTruthy();
    expect(body.auth.policies).toContain("default");
  });

  test("create KV secret — POST /v1/secret/data/test-key", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/test-key`, {
      method: "POST",
      headers: headers(rootToken),
      body: JSON.stringify({
        data: { value: "pdv-test-value" },
      }),
    });
    expect(resp.status).toBe(200);
  });

  test("read KV secret — GET /v1/secret/data/test-key", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/test-key`, {
      headers: headers(rootToken),
    });
    expect(resp.status).toBe(200);

    const body: KvReadResponse = await resp.json();
    expect(body.data.data.value).toBe("pdv-test-value");
  });

  test("update KV secret — POST /v1/secret/data/test-key with new value", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/test-key`, {
      method: "POST",
      headers: headers(rootToken),
      body: JSON.stringify({
        data: { value: "pdv-test-value-updated" },
      }),
    });
    expect(resp.status).toBe(200);
  });

  test("read updated secret — GET /v1/secret/data/test-key", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/test-key`, {
      headers: headers(rootToken),
    });
    expect(resp.status).toBe(200);

    const body: KvReadResponse = await resp.json();
    expect(body.data.data.value).toBe("pdv-test-value-updated");
  });

  test("delete KV secret — DELETE /v1/secret/data/test-key", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/test-key`, {
      method: "DELETE",
      headers: headers(rootToken),
    });
    expect(resp.status).toBe(204);
  });

  test("verify deleted — GET /v1/secret/data/test-key returns 404", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/secret/data/test-key`, {
      headers: headers(rootToken),
    });
    // After soft-delete, the data field is null but metadata remains (status 200)
    // or 404 if metadata is also gone
    expect([200, 404]).toContain(resp.status);

    if (resp.status === 200) {
      const body: KvReadResponse = await resp.json();
      expect(body.data.data).toBeNull();
    }
  });

  test("delete test user — DELETE /v1/auth/userpass/users/test-user", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/auth/userpass/users/test-user`, {
      method: "DELETE",
      headers: headers(rootToken),
    });
    expect(resp.status).toBe(204);
  });

  test("create policy — PUT /v1/sys/policies/acl/test-policy", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/policies/acl/test-policy`, {
      method: "PUT",
      headers: headers(rootToken),
      body: JSON.stringify({
        policy: 'path "secret/data/*" { capabilities = ["read", "list"] }',
      }),
    });
    expect(resp.status).toBe(204);
  });

  test("read policy — GET /v1/sys/policies/acl/test-policy", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/policies/acl/test-policy`, {
      headers: headers(rootToken),
    });
    expect(resp.status).toBe(200);

    const body: PolicyReadResponse = await resp.json();
    const policyText = body.data.policy || body.data.rules || "";
    expect(policyText).toContain("secret/data/*");
  });

  test("delete policy — DELETE /v1/sys/policies/acl/test-policy", async () => {
    const resp = await fetch(`${OPENBAO_URL}/v1/sys/policies/acl/test-policy`, {
      method: "DELETE",
      headers: headers(rootToken),
    });
    expect(resp.status).toBe(204);
  });
});
