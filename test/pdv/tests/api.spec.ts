import { test, expect } from "@playwright/test";

const APP = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

test.describe("API Post-Deployment Verification", () => {
  test("GET /api/v1/status returns healthy", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/status`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.status).toBe("healthy");
    expect(body.version).toBeDefined();
    expect(body.services).toBeInstanceOf(Array);
  });

  test("POST /api/v1/auth/login returns session with JWT", async ({ request }) => {
    const res = await request.post(`${APP}/api/v1/auth/login`, {
      data: { username: "admin", password: "test", mfaToken: "123456" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.accessToken).toBeDefined();
    expect(body.principal.username).toBe("admin");
    expect(body.principal.roles).toContain("admin");
    expect(body.principal.authorizedApplets).toBeInstanceOf(Array);
    expect(body.principal.authorizedApplets.length).toBeGreaterThan(0);
  });

  test("GET /api/v1/auth/me returns principal", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/auth/me`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.username).toBe("admin");
  });

  test("unauthenticated request returns 401", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/nmgr/node`);
    expect(res.status()).toBe(401);
  });

  test("GET /api/v1/nmgr/node returns K8s nodes", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/nmgr/node`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.total).toBeGreaterThan(0);
    expect(body.items).toBeInstanceOf(Array);
    expect(body.items[0].id).toBeDefined();
    expect(body.items[0].ip).toBeDefined();
    expect(body.items[0].status).toBeDefined();
  });

  test("GET /api/v1/amgr/agent returns paginated agents", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/amgr/agent`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("total");
    expect(body).toHaveProperty("items");
  });

  test("GET /api/v1/pb/board returns paginated boards", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/pb/board`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("total");
  });

  test("GET /api/v1/ac/user returns paginated users", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/ac/user`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("total");
  });

  test("GET /api/v1/repo/repo returns paginated repos", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/repo/repo`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("total");
  });

  test("GET /api/v1/ide/project returns paginated projects", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/ide/project`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("total");
  });

  test("GET /api/v1/sup/ticket returns paginated tickets", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/sup/ticket`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("total");
  });

  test("GET /api/v1/ac/settings returns global settings", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/ac/settings`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("sessionTimeoutMinutes");
  });
});
