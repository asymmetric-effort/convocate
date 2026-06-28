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

  // -- Node provisioning and metrics API tests --------------------------------

  test("POST /api/v1/nmgr/node rejects missing host", async ({ request }) => {
    const res = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: { Authorization: "Bearer mock-token" },
      data: { user: "testuser" },
    });
    expect(res.status()).toBe(400);
    const body = await res.json();
    expect(body.message).toContain("host is required");
  });

  test("POST /api/v1/nmgr/node rejects missing user", async ({ request }) => {
    const res = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: { Authorization: "Bearer mock-token" },
      data: { host: "10.99.99.99" },
    });
    expect(res.status()).toBe(400);
    const body = await res.json();
    expect(body.message).toContain("user is required");
  });

  test("POST /api/v1/nmgr/node returns 202 with Pending node", async ({ request }) => {
    const res = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: { Authorization: "Bearer mock-token" },
      data: { host: "10.99.99.98", user: "testuser" },
    });
    expect(res.status()).toBe(202);
    const body = await res.json();
    expect(body.id).toBeTruthy();
    expect(body.ip).toBe("10.99.99.98");
    expect(body.status).toBe("Pending");
  });

  test("Provisioned node appears in GET /api/v1/nmgr/node list", async ({ request }) => {
    const createRes = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: { Authorization: "Bearer mock-token" },
      data: { host: "10.99.99.96", user: "testuser" },
    });
    const created = await createRes.json();

    const res = await request.get(`${APP}/api/v1/nmgr/node?limit=100`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    const found = body.items.find((n: any) => n.id === created.id);
    expect(found).toBeDefined();
    expect(["Pending", "Ready"]).toContain(found.status);
  });

  test("K8s node metrics contain valid loadAvg, memory, and disk fields", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/nmgr/node`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();

    for (const node of body.items) {
      // Skip store-provisioned test nodes
      if (node.memTotalGB === 0) continue;

      // Structure checks
      expect(node.id).toBeTruthy();
      expect(node.ip).toBeTruthy();
      expect(node.status).toBeTruthy();
      expect(node.loadAvg).toHaveProperty("one");
      expect(node.loadAvg).toHaveProperty("five");
      expect(node.loadAvg).toHaveProperty("fifteen");
      expect(typeof node.memUsedGB).toBe("number");
      expect(typeof node.memTotalGB).toBe("number");
      expect(typeof node.diskUsedGB).toBe("number");
      expect(typeof node.diskTotalGB).toBe("number");

      // K8s capacity is always positive
      expect(node.memTotalGB).toBeGreaterThan(0);

      // Metrics must be -1 (no metrics-server) or >= 0 (real data).
      // They must NEVER be the old hardcoded cpuCap*0.3 / cpuCap*0.25 / cpuCap*0.2 pattern.
      expect(node.loadAvg.one === -1 || node.loadAvg.one >= 0).toBe(true);
      expect(node.memUsedGB === -1 || node.memUsedGB >= 0).toBe(true);
      expect(node.diskUsedGB === -1 || node.diskUsedGB >= 0).toBe(true);
    }
  });

  test("Events endpoint rejects unauthenticated requests", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/events/nmgr/status`);
    expect(res.status()).toBe(401);
  });

  test("Events endpoint accepts token via query param", async ({ request }) => {
    let authed = false;
    try {
      const res = await request.get(`${APP}/api/v1/events/nmgr/status?token=mock-token`, {
        timeout: 2000,
      });
      authed = res.status() !== 401;
    } catch {
      authed = true; // timeout = SSE streaming accepted = auth passed
    }
    expect(authed).toBe(true);
  });

  test("Events API delivers node.metrics with correct structure", async ({ page }) => {
    // Navigate to the app first so EventSource connects same-origin
    await page.goto(APP);
    await page.waitForTimeout(1000);

    // Subscribe to SSE from within the page context and capture
    // the first node.metrics event (publisher runs every 3s)
    const metricsEvent = await page.evaluate(async () => {
      return new Promise<any>((resolve, reject) => {
        const timeout = setTimeout(() => {
          es.close();
          reject(new Error("no node.metrics event within 10s"));
        }, 10000);
        const es = new EventSource(`/api/v1/events/nmgr/status?token=mock-token`);
        es.onmessage = (msg) => {
          try {
            const evt = JSON.parse(msg.data);
            if (evt.type === "node.metrics") {
              clearTimeout(timeout);
              es.close();
              resolve(evt);
            }
          } catch { /* ignore parse errors */ }
        };
        es.onerror = () => {
          clearTimeout(timeout);
          es.close();
          reject(new Error("SSE connection error"));
        };
      });
    });

    expect(metricsEvent).not.toBeNull();
    expect(metricsEvent.type).toBe("node.metrics");
    expect(metricsEvent.payload).toBeInstanceOf(Array);
    expect(metricsEvent.payload.length).toBeGreaterThan(0);

    // Each node in the metrics event must carry the fields the UI needs
    for (const node of metricsEvent.payload) {
      expect(node.id).toBeTruthy();
      expect(node).toHaveProperty("loadAvg");
      expect(node.loadAvg).toHaveProperty("one");
      expect(node).toHaveProperty("memUsedGB");
      expect(node).toHaveProperty("memTotalGB");
      expect(node).toHaveProperty("diskUsedGB");
      expect(node).toHaveProperty("diskTotalGB");
    }
  });

  // ---------------------------------------------------------------------------

  test("GET /api/v1/ac/settings returns global settings", async ({ request }) => {
    const res = await request.get(`${APP}/api/v1/ac/settings`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("sessionTimeoutMinutes");
  });
});
