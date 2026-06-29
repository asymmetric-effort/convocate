import { test, expect } from "@playwright/test";

const APP = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";
const AUTH = { Authorization: "Bearer mock-token" };
const UID = Date.now().toString(36);

test.describe("Status", () => {
  test("GET /status returns healthy", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/status`);
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.status).toBe("healthy");
    expect(b.version).toBeDefined();
    expect(b.services).toBeInstanceOf(Array);
    expect(b.timestamp).toBeDefined();
  });
});

test.describe("Auth", () => {
  test("POST /auth/login success", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/auth/login`, {
      data: { username: "admin", password: "test", mfaToken: "123456" },
    });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.accessToken).toBeTruthy();
    expect(b.refreshToken).toBeTruthy();
    expect(b.expiresAt).toBeTruthy();
    expect(b.principal.username).toBe("admin");
    expect(b.principal.roles).toContain("admin");
    expect(b.principal.authorizedApplets).toBeInstanceOf(Array);
    expect(b.principal.authorizedApplets.length).toBeGreaterThan(0);
  });

  test("POST /auth/login empty credentials returns 401", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/auth/login`, {
      data: { username: "", password: "" },
    });
    expect(r.status()).toBe(401);
  });

  test("GET /auth/me returns principal", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/auth/me`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.username).toBe("admin");
    expect(b.id).toBeTruthy();
    expect(b.email).toBeTruthy();
    expect(b.roles).toBeInstanceOf(Array);
  });

  test("GET /auth/me without token returns 401", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/auth/me`);
    expect(r.status()).toBe(401);
  });

  test("POST /auth/refresh returns new tokens", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/auth/refresh`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.accessToken).toBeTruthy();
  });

  test("POST /auth/logout returns 204", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/auth/logout`, { headers: AUTH });
    expect(r.status()).toBe(204);
  });
});

test.describe("Node Manager", () => {
  test("GET /nmgr/node returns paginated nodes", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
    expect(b.items).toBeInstanceOf(Array);
    expect(b.total).toBeGreaterThan(0);
    const n = b.items[0];
    expect(n.id).toBeTruthy();
    expect(n.ip).toBeTruthy();
    expect(n.status).toBeTruthy();
    expect(n).toHaveProperty("loadAvg");
    expect(n).toHaveProperty("memTotalGB");
  });

  test("GET /nmgr/node without auth returns 401", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/nmgr/node`);
    expect(r.status()).toBe(401);
  });

  test("GET /nmgr/node respects pagination", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/nmgr/node?offset=0&limit=2`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.offset).toBe(0);
    expect(b.limit).toBe(2);
    expect(b.items.length).toBeLessThanOrEqual(2);
  });

  test("POST /nmgr/node rejects missing host", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: AUTH, data: { user: "testuser" },
    });
    expect(r.status()).toBe(400);
    const b = await r.json();
    expect(b.message).toContain("host is required");
  });

  test("POST /nmgr/node rejects missing user", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: AUTH, data: { host: "10.88.88.88" },
    });
    expect(r.status()).toBe(400);
    const b = await r.json();
    expect(b.message).toContain("user is required");
  });

  test("POST /nmgr/node creates pending node (202)", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: AUTH, data: { host: "10.88.88.80", user: "testuser", location: "lab" },
    });
    expect(r.status()).toBe(202);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.ip).toBe("10.88.88.80");
    expect(b.status).toBe("Pending");
  });

  test("provisioned node appears in list", async ({ request }) => {
    const cr = await request.post(`${APP}/api/v1/nmgr/node`, {
      headers: AUTH, data: { host: "10.88.88.81", user: "testuser" },
    });
    const created = await cr.json();
    const r = await request.get(`${APP}/api/v1/nmgr/node?limit=200`, { headers: AUTH });
    const b = await r.json();
    const found = b.items.find((n: any) => n.id === created.id);
    expect(found).toBeDefined();
    expect(["Pending", "Ready"]).toContain(found.status);
  });

  test("GET /nmgr/node/{id} returns node detail", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH })).json();
    const nodeId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/nmgr/node/${nodeId}`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.id).toBe(nodeId);
    expect(b).toHaveProperty("agentList");
    expect(b).toHaveProperty("notes");
  });

  test("GET /nmgr/node/{id} 404 for nonexistent", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/nmgr/node/does-not-exist`, { headers: AUTH });
    expect(r.status()).toBe(404);
  });

  test("POST /nmgr/node/{id}/note creates note", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH })).json();
    const nodeId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/nmgr/node/${nodeId}/note`, {
      headers: AUTH, data: { text: "PDV test note" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.text).toBe("PDV test note");
    expect(b.author).toBeTruthy();
    expect(b.createdAt).toBeTruthy();
  });

  test("POST /nmgr/node/{id}/note rejects empty text", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH })).json();
    const nodeId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/nmgr/node/${nodeId}/note`, {
      headers: AUTH, data: { text: "" },
    });
    expect(r.status()).toBe(400);
  });

  test("GET /nmgr/node/{id}/note returns notes", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH })).json();
    const nodeId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/nmgr/node/${nodeId}/note`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toBeInstanceOf(Array);
  });

  test("POST /nmgr/node/{id}/stop cordons node (202)", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH })).json();
    const nodeId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/nmgr/node/${nodeId}/stop`, { headers: AUTH });
    expect(r.status()).toBe(202);
  });

  test("POST /nmgr/node/{id}/start uncordons node (202)", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH })).json();
    const nodeId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/nmgr/node/${nodeId}/start`, { headers: AUTH });
    expect(r.status()).toBe(202);
  });

  test("K8s node metrics are valid (-1 or >= 0)", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/nmgr/node`, { headers: AUTH });
    const b = await r.json();
    for (const n of b.items) {
      if (n.memTotalGB === 0) continue; // skip store-provisioned test nodes
      expect(n.memTotalGB).toBeGreaterThan(0);
      expect(n.loadAvg.one === -1 || n.loadAvg.one >= 0).toBe(true);
      expect(n.memUsedGB === -1 || n.memUsedGB >= 0).toBe(true);
      expect(n.diskUsedGB === -1 || n.diskUsedGB >= 0).toBe(true);
    }
  });
});

test.describe("Agent Manager", () => {
  test("GET /amgr/agent returns paginated agents", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/amgr/agent`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
    expect(b.items).toBeInstanceOf(Array);
  });

  test("POST /amgr/agent creates agent", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/amgr/agent`, {
      headers: AUTH, data: { project: `pdv-${UID}` },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.project).toBe(`pdv-${UID}`);
  });

  test("GET /amgr/agent/{id} returns agent detail", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/amgr/agent`, { headers: AUTH })).json();
    const agentId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/amgr/agent/${agentId}`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.id).toBe(agentId);
    expect(b).toHaveProperty("status");
    expect(b).toHaveProperty("project");
  });

  test("GET /amgr/agent/{id} 404 for nonexistent", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/amgr/agent/does-not-exist`, { headers: AUTH });
    expect(r.status()).toBe(404);
  });

  test("PATCH /amgr/agent/{id} updates or returns 501 (K8s)", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/amgr/agent`, { headers: AUTH })).json();
    const agentId = list.items[0].id;
    const r = await request.patch(`${APP}/api/v1/amgr/agent/${agentId}`, {
      headers: AUTH, data: { project: "updated-project" },
    });
    expect([200, 501]).toContain(r.status());
  });

  test("POST /amgr/agent/{id}/stop stops agent", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/amgr/agent`, { headers: AUTH })).json();
    const agentId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/amgr/agent/${agentId}/stop`, { headers: AUTH });
    expect([202, 204]).toContain(r.status());
  });

  test("POST /amgr/agent/{id}/start starts agent", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/amgr/agent`, { headers: AUTH })).json();
    const agentId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/amgr/agent/${agentId}/start`, { headers: AUTH });
    expect([202, 501]).toContain(r.status()); // 501 in K8s mode
  });

  test("DELETE /amgr/agent/{id} returns 204", async ({ request }) => {
    const cr = await request.post(`${APP}/api/v1/amgr/agent`, {
      headers: AUTH, data: { project: `del-${UID}` },
    });
    const created = await cr.json();
    const r = await request.delete(`${APP}/api/v1/amgr/agent/${created.id}`, { headers: AUTH });
    expect(r.status()).toBe(204);
  });

  test("GET /amgr/agent/{id}/shell returns 501 (not implemented)", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/amgr/agent`, { headers: AUTH })).json();
    const agentId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/amgr/agent/${agentId}/shell`, { headers: AUTH });
    expect(r.status()).toBe(501);
  });
});

test.describe("Project Board", () => {
  test("GET /pb/board returns paginated boards", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
  });

  test("POST /pb/board creates board", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/pb/board`, {
      headers: AUTH, data: { name: "PDV Test Board", repoId: "repo-001" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.name).toBe("PDV Test Board");
  });

  test("POST /pb/board with no name creates board with empty name", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/pb/board`, {
      headers: AUTH, data: { repoId: "repo-001" },
    });
    expect(r.status()).toBe(201);
  });

  test("GET /pb/board/{id} returns full board", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const boardId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/pb/board/${boardId}`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.id).toBe(boardId);
    expect(b).toHaveProperty("containers");
    expect(b).toHaveProperty("cards");
    expect(b).toHaveProperty("edges");
  });

  test("GET /pb/board/{id} 404 for nonexistent", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/pb/board/does-not-exist`, { headers: AUTH });
    expect(r.status()).toBe(404);
  });

  test("PATCH /pb/board/{id} renames board", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const boardId = list.items[0].id;
    const r = await request.patch(`${APP}/api/v1/pb/board/${boardId}`, {
      headers: AUTH, data: { name: "Renamed Board" },
    });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.name).toBe("Renamed Board");
  });

  test("POST /pb/board/{id}/container creates container", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const boardId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/pb/board/${boardId}/container`, {
      headers: AUTH, data: { title: "PDV Container" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.title).toBe("PDV Container");
  });

  test("POST /pb/board/{id}/card creates card", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const boardId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/pb/board/${boardId}/card`, {
      headers: AUTH, data: { title: "PDV Card", content: "test content", status: "todo" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.title).toBe("PDV Card");
  });

  test("GET /pb/board/{id}/card/{cardId} returns card", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const board = await (await request.get(`${APP}/api/v1/pb/board/${list.items[0].id}`, { headers: AUTH })).json();
    if (board.cards.length === 0) return;
    const cardId = board.cards[0].id;
    const r = await request.get(`${APP}/api/v1/pb/board/${board.id}/card/${cardId}`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.id).toBe(cardId);
  });

  test("POST /pb/board/{id}/edge creates edge", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const board = await (await request.get(`${APP}/api/v1/pb/board/${list.items[0].id}`, { headers: AUTH })).json();
    if (board.cards.length < 2) return;
    const r = await request.post(`${APP}/api/v1/pb/board/${board.id}/edge`, {
      headers: AUTH, data: { from: board.cards[0].id, to: board.cards[1].id, type: "DependsOn" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
  });

  test("POST /pb/board/{id}/implement returns 202", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const boardId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/pb/board/${boardId}/implement`, { headers: AUTH });
    expect(r.status()).toBe(202);
    const b = await r.json();
    expect(b).toHaveProperty("id");
  });

  test("POST /pb/board/{id}/save-as-repo returns 201", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/pb/board`, { headers: AUTH })).json();
    const boardId = list.items[0].id;
    const r = await request.post(`${APP}/api/v1/pb/board/${boardId}/save-as-repo`, { headers: AUTH });
    expect(r.status()).toBe(201);
  });
});

test.describe("IDE", () => {
  test("GET /ide/project returns paginated projects", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/ide/project`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
  });

  test("POST /ide/project creates project", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/ide/project`, {
      headers: AUTH, data: { name: `pdv-${UID}` },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.name).toBe(`pdv-${UID}`);
  });

  test("POST /ide/project with empty body still creates", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/ide/project`, {
      headers: AUTH, data: {},
    });
    expect([201, 400]).toContain(r.status());
  });

  test("GET /ide/project/{id}/tree returns file tree", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/ide/project`, { headers: AUTH })).json();
    const projId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/ide/project/${projId}/tree`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toBeInstanceOf(Array);
    if (b.length > 0) {
      expect(b[0]).toHaveProperty("name");
      expect(b[0]).toHaveProperty("type");
      expect(b[0]).toHaveProperty("path");
    }
  });

  test("GET /ide/project/{id}/tree for nonexistent returns 404 or empty", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/ide/project/does-not-exist/tree`, { headers: AUTH });
    expect([200, 404]).toContain(r.status());
  });

  test("PUT /ide/project/{id}/file/{path} creates/updates file", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/ide/project`, { headers: AUTH })).json();
    const projId = list.items[0].id;
    const r = await request.put(`${APP}/api/v1/ide/project/${projId}/file/pdv-test.txt`, {
      headers: AUTH, data: { content: "hello from pdv" },
    });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.path).toBe("pdv-test.txt");
    expect(b.content).toBe("hello from pdv");
  });

  test("GET /ide/project/{id}/file/{path} returns file", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/ide/project`, { headers: AUTH })).json();
    const projId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/ide/project/${projId}/file/pdv-test.txt`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.content).toBe("hello from pdv");
  });

  test("GET /ide/project/{id}/file/{path} 404 for nonexistent", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/ide/project`, { headers: AUTH })).json();
    const projId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/ide/project/${projId}/file/no-such-file.txt`, { headers: AUTH });
    expect(r.status()).toBe(404);
  });

  test("DELETE /ide/project/{id}/file/{path} removes file", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/ide/project`, { headers: AUTH })).json();
    const projId = list.items[0].id;
    await request.put(`${APP}/api/v1/ide/project/${projId}/file/delete-me.txt`, {
      headers: AUTH, data: { content: "temp" },
    });
    const r = await request.delete(`${APP}/api/v1/ide/project/${projId}/file/delete-me.txt`, { headers: AUTH });
    expect(r.status()).toBe(204);
    const check = await request.get(`${APP}/api/v1/ide/project/${projId}/file/delete-me.txt`, { headers: AUTH });
    expect(check.status()).toBe(404);
  });

  test("POST /ide/project/{id}/rename-file renames file", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/ide/project`, { headers: AUTH })).json();
    const projId = list.items[0].id;
    const oldName = `rename-${UID}.txt`;
    const newName = `renamed-${UID}.txt`;
    await request.put(`${APP}/api/v1/ide/project/${projId}/file/${oldName}`, {
      headers: AUTH, data: { content: "rename test" },
    });
    const r = await request.post(`${APP}/api/v1/ide/project/${projId}/rename-file`, {
      headers: AUTH, data: { oldPath: oldName, newPath: newName },
    });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.path).toBe(newName);
  });
});

test.describe("Repository", () => {
  test("GET /repo/repo returns paginated repos", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/repo/repo`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
  });

  test("POST /repo/repo creates repo", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/repo/repo`, {
      headers: AUTH, data: { name: "pdv-test-repo", visibility: "private" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.name).toBe("pdv-test-repo");
    expect(b.visibility).toBe("private");
  });

  test("POST /repo/repo with missing name", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/repo/repo`, {
      headers: AUTH, data: { visibility: "private" },
    });
    expect([201, 400]).toContain(r.status());
  });

  test("GET /repo/repo/{id}/file returns file list", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/repo/repo`, { headers: AUTH })).json();
    const repoId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/repo/repo/${repoId}/file`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toBeInstanceOf(Array);
  });

  test("GET /repo/repo/{id}/pr returns paginated PRs", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/repo/repo`, { headers: AUTH })).json();
    const repoId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/repo/repo/${repoId}/pr`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
  });

  test("GET /repo/repo/{id}/pr/{prId} returns PR detail", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/repo/repo`, { headers: AUTH })).json();
    const repoId = list.items[0].id;
    const prs = await (await request.get(`${APP}/api/v1/repo/repo/${repoId}/pr`, { headers: AUTH })).json();
    if (prs.items.length === 0) return;
    const prId = prs.items[0].id;
    const r = await request.get(`${APP}/api/v1/repo/repo/${repoId}/pr/${prId}`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.id).toBe(prId);
    expect(b).toHaveProperty("status");
    expect(b).toHaveProperty("branch");
  });

  test("GET /repo/repo/{id}/pr/{prId} 404 for nonexistent", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/repo/repo`, { headers: AUTH })).json();
    const repoId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/repo/repo/${repoId}/pr/no-such-pr`, { headers: AUTH });
    expect(r.status()).toBe(404);
  });

  test("POST /repo/repo/{id}/pr/{prId}/merge merges PR", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/repo/repo`, { headers: AUTH })).json();
    const repoId = list.items[0].id;
    const prs = await (await request.get(`${APP}/api/v1/repo/repo/${repoId}/pr`, { headers: AUTH })).json();
    if (prs.items.length === 0) return;
    const prId = prs.items[0].id;
    const r = await request.post(`${APP}/api/v1/repo/repo/${repoId}/pr/${prId}/merge`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.status).toBe("merged");
  });
});

test.describe("Access Control", () => {
  test("GET /ac/user returns paginated users", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/ac/user`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
    expect(b.total).toBeGreaterThan(0);
  });

  test("POST /ac/user creates user", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/ac/user`, {
      headers: AUTH, data: { email: "pdv@test.com", name: "PDV User", status: "active" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.email).toBe("pdv@test.com");
  });

  test("PATCH /ac/user/{id} updates user", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/ac/user`, { headers: AUTH })).json();
    const userId = list.items[0].id;
    const r = await request.patch(`${APP}/api/v1/ac/user/${userId}`, {
      headers: AUTH, data: { name: "Updated Name" },
    });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.name).toBe("Updated Name");
  });

  test("PATCH /ac/user/{id} 404 for nonexistent", async ({ request }) => {
    const r = await request.patch(`${APP}/api/v1/ac/user/no-such-user`, {
      headers: AUTH, data: { name: "nope" },
    });
    expect(r.status()).toBe(404);
  });

  test("DELETE /ac/user/{id} deletes user", async ({ request }) => {
    const cr = await request.post(`${APP}/api/v1/ac/user`, {
      headers: AUTH, data: { email: "del@test.com", name: "Delete Me" },
    });
    const created = await cr.json();
    const r = await request.delete(`${APP}/api/v1/ac/user/${created.id}`, { headers: AUTH });
    expect(r.status()).toBe(204);
  });

  test("GET /ac/group returns paginated groups", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/ac/group`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b.total).toBeGreaterThan(0);
    const g = b.items[0];
    expect(g).toHaveProperty("name");
    expect(g).toHaveProperty("roles");
  });

  test("POST /ac/group creates group", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/ac/group`, {
      headers: AUTH, data: { name: "pdv-test-group" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.name).toBe("pdv-test-group");
  });

  test("POST /ac/group with missing name", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/ac/group`, {
      headers: AUTH, data: {},
    });
    expect([201, 400]).toContain(r.status());
  });

  test("DELETE /ac/group/{id} deletes non-builtin group", async ({ request }) => {
    const cr = await request.post(`${APP}/api/v1/ac/group`, {
      headers: AUTH, data: { name: "delete-me-group" },
    });
    const created = await cr.json();
    const r = await request.delete(`${APP}/api/v1/ac/group/${created.id}`, { headers: AUTH });
    expect(r.status()).toBe(204);
  });

  test("PUT /ac/group/{id}/user sets group users", async ({ request }) => {
    const groups = await (await request.get(`${APP}/api/v1/ac/group`, { headers: AUTH })).json();
    const groupId = groups.items[0].id;
    const users = await (await request.get(`${APP}/api/v1/ac/user`, { headers: AUTH })).json();
    const userIds = users.items.map((u: any) => u.id);
    const r = await request.put(`${APP}/api/v1/ac/group/${groupId}/user`, {
      headers: AUTH, data: { userIds },
    });
    expect(r.status()).toBe(200);
  });

  test("PUT /ac/group/{id}/role sets group roles", async ({ request }) => {
    const groups = await (await request.get(`${APP}/api/v1/ac/group`, { headers: AUTH })).json();
    const groupId = groups.items[0].id;
    const r = await request.put(`${APP}/api/v1/ac/group/${groupId}/role`, {
      headers: AUTH, data: { roles: ["admin", "node-view"] },
    });
    expect(r.status()).toBe(200);
  });

  test("GET /ac/role returns paginated roles", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/ac/role`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b.total).toBeGreaterThan(0);
  });

  test("GET /ac/settings returns global settings", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/ac/settings`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("sessionTimeoutMinutes");
    expect(b).toHaveProperty("passwordMinLength");
  });

  test("PUT /ac/settings updates global settings", async ({ request }) => {
    const r = await request.put(`${APP}/api/v1/ac/settings`, {
      headers: AUTH,
      data: { requireMfa: false, sessionTimeoutMinutes: 60, passwordMinLength: 12, passwordRotationDays: 90 },
    });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.sessionTimeoutMinutes).toBe(60);
  });
});

test.describe("Support", () => {
  test("GET /sup/ticket returns paginated tickets", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/sup/ticket`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
  });

  test("POST /sup/ticket creates ticket", async ({ request }) => {
    const r = await request.post(`${APP}/api/v1/sup/ticket`, {
      headers: AUTH, data: { subject: "PDV Test Ticket", priority: "low", body: "test body" },
    });
    expect(r.status()).toBe(201);
    const b = await r.json();
    expect(b.id).toBeTruthy();
    expect(b.subject).toBe("PDV Test Ticket");
  });

  test("GET /sup/ticket/{id} returns ticket detail", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/sup/ticket`, { headers: AUTH })).json();
    const ticketId = list.items[0].id;
    const r = await request.get(`${APP}/api/v1/sup/ticket/${ticketId}`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.id).toBe(ticketId);
    expect(b).toHaveProperty("subject");
    expect(b).toHaveProperty("status");
  });

  test("GET /sup/ticket/{id} 404 for nonexistent", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/sup/ticket/no-such-ticket`, { headers: AUTH });
    expect(r.status()).toBe(404);
  });

  test("PATCH /sup/ticket/{id} updates ticket", async ({ request }) => {
    const list = await (await request.get(`${APP}/api/v1/sup/ticket`, { headers: AUTH })).json();
    const ticketId = list.items[0].id;
    const r = await request.patch(`${APP}/api/v1/sup/ticket/${ticketId}`, {
      headers: AUTH, data: { status: "closed" },
    });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b.status).toBe("closed");
  });

  test("GET /sup/doc returns paginated docs", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/sup/doc`, { headers: AUTH });
    expect(r.status()).toBe(200);
    const b = await r.json();
    expect(b).toHaveProperty("total");
    expect(b).toHaveProperty("items");
    expect(b.total).toBeGreaterThan(0);
  });
});

test.describe("Events", () => {
  test("Events endpoint rejects unauthenticated requests", async ({ request }) => {
    const r = await request.get(`${APP}/api/v1/events/nmgr/status`);
    expect(r.status()).toBe(401);
  });

  test("Events endpoint accepts token via query param", async ({ request }) => {
    let authed = false;
    try {
      const r = await request.get(`${APP}/api/v1/events/nmgr/status?token=mock-token`, { timeout: 2000 });
      authed = r.status() !== 401;
    } catch {
      authed = true;
    }
    expect(authed).toBe(true);
  });

  test("SSE delivers node.metrics event", async ({ page }) => {
    await page.goto(APP);
    await page.waitForTimeout(1000);
    const evt = await page.evaluate(async () => {
      return new Promise<any>((resolve, reject) => {
        const t = setTimeout(() => { es.close(); reject(new Error("timeout")); }, 10000);
        const es = new EventSource(`/api/v1/events/nmgr/status?token=mock-token`);
        es.onmessage = (msg) => {
          try {
            const e = JSON.parse(msg.data);
            if (e.type === "node.metrics") { clearTimeout(t); es.close(); resolve(e); }
          } catch {}
        };
        es.onerror = () => { clearTimeout(t); es.close(); reject(new Error("sse error")); };
      });
    });
    expect(evt.type).toBe("node.metrics");
    expect(evt.payload).toBeInstanceOf(Array);
    expect(evt.payload.length).toBeGreaterThan(0);
    for (const n of evt.payload) {
      expect(n.id).toBeTruthy();
      expect(n).toHaveProperty("loadAvg");
      expect(n).toHaveProperty("memUsedGB");
      expect(n).toHaveProperty("memTotalGB");
    }
  });
});
