import { test, expect } from "@playwright/test";

const API = process.env.API_URL || "http://convocate-api.convocate.svc:8443";

test.describe("API Post-Deployment Verification", () => {
  test("GET /api/v1/status returns healthy", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/status`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.status).toBe("healthy");
    expect(body.version).toBeDefined();
  });

  test("POST /api/v1/auth/login returns session", async ({ request }) => {
    const res = await request.post(`${API}/api/v1/auth/login`, {
      data: { username: "admin", password: "test", mfaToken: "123456" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.accessToken).toBeDefined();
    expect(body.principal.username).toBe("admin");
  });

  test("GET /api/v1/auth/me returns principal", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/auth/me`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.username).toBe("admin");
  });

  test("unauthenticated request returns 401", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/nmgr/node`);
    expect(res.status()).toBe(401);
  });

  test("GET /api/v1/nmgr/node returns paginated nodes", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/nmgr/node`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
    expect(body.items).toBeInstanceOf(Array);
  });

  test("GET /api/v1/amgr/agent returns paginated agents", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/amgr/agent`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
  });

  test("GET /api/v1/pb/board returns paginated boards", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/pb/board`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
  });

  test("GET /api/v1/ac/user returns paginated users", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/ac/user`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
  });

  test("GET /api/v1/repo/repo returns paginated repos", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/repo/repo`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
  });

  test("GET /api/v1/sup/ticket returns paginated tickets", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/sup/ticket`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
  });

  test("GET /api/v1/ide/project returns paginated projects", async ({ request }) => {
    const res = await request.get(`${API}/api/v1/ide/project`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
  });
});
