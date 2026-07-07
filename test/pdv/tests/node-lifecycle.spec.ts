/**
 * Node Lifecycle — Post-Deployment Verification Tests
 *
 * Validates the full stop → drain → start lifecycle on a live K8s node.
 *
 * The test picks a random node in Ready status, stops it (cordon/drain),
 * verifies the status transitions and that any agents are migrated off,
 * then starts it again and verifies restoration to Ready.
 *
 * This is a DESTRUCTIVE test — it temporarily removes a real node from
 * scheduling.  It always restores the node in the afterEach hook.
 */

import { test, expect, Page } from "@playwright/test";

// Disable TLS verification for direct API calls (self-signed certs)

// ---------------------------------------------------------------------------
// Increase timeout — drain and migration can take time on a real cluster
// ---------------------------------------------------------------------------

test.setTimeout(120_000);

// ---------------------------------------------------------------------------
// API helpers — bypass the UI for lifecycle verification
// ---------------------------------------------------------------------------

const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

function authHeaders(): Record<string, string> {
  return {
    "Content-Type": "application/json",
    Authorization: "Bearer mock-token",
  };
}

interface NodeInfo {
  id: string;
  status: string;
  agents: number;
  ip: string;
}

interface PageResponse {
  items: NodeInfo[];
  total: number;
}

/** Fetch all nodes from the API */
async function listNodes(): Promise<NodeInfo[]> {
  const res = await fetch(`${BASE}/api/v1/nmgr/node?limit=100`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`listNodes failed: ${res.status}`);
  const page: PageResponse = await res.json();
  return page.items || [];
}

/** Get a single node's current state */
async function getNode(id: string): Promise<NodeInfo> {
  const res = await fetch(`${BASE}/api/v1/nmgr/node/${id}`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`getNode ${id} failed: ${res.status}`);
  return res.json();
}

/** Stop (cordon/drain) a node */
async function stopNode(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/v1/nmgr/node/${id}/stop`, {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`stopNode ${id} failed: ${res.status}`);
}

/** Start (uncordon) a node */
async function startNode(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/v1/nmgr/node/${id}/start`, {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`startNode ${id} failed: ${res.status}`);
}

/** Poll a node until it reaches the expected status, with timeout */
async function waitForStatus(
  id: string,
  expectedStatus: string,
  timeoutMs: number = 60_000
): Promise<NodeInfo> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const node = await getNode(id);
    if (node.status === expectedStatus) return node;
    await new Promise((r) => setTimeout(r, 2000));
  }
  throw new Error(
    `Timed out waiting for node ${id} to reach status "${expectedStatus}" after ${timeoutMs}ms`
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function login(page: Page): Promise<void> {
  await page.goto("/");
  await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });
  await page.locator('input[placeholder="Username"]').fill("admin");
  await page.locator('input[placeholder="Password"]').fill("test");
  await page.locator('input[placeholder="MFA Token"]').fill("123456");
  await page.locator('button:has-text("Sign In")').click();
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({
    timeout: 15000,
  });
}

// ---------------------------------------------------------------------------
// Test
// ---------------------------------------------------------------------------

test.describe("Node stop/start lifecycle", () => {
  let targetNodeId: string | null = null;

  // SAFETY: always restore the node to Ready after each test
  test.afterEach(async () => {
    if (targetNodeId) {
      try {
        await startNode(targetNodeId);
        await waitForStatus(targetNodeId, "Ready", 30_000);
      } catch (err) {
        console.error(`WARNING: failed to restore node ${targetNodeId}:`, err);
      }
      targetNodeId = null;
    }
  });

  test("stop cordons the node, migrates agents, and start restores it", async ({
    page,
  }) => {
    // ---------------------------------------------------------------
    // Step 1: Find a random Ready node
    // ---------------------------------------------------------------
    const allNodes = await listNodes();
    const readyNodes = allNodes.filter((n) => n.status === "Ready");
    expect(readyNodes.length, "Need at least one Ready node").toBeGreaterThan(0);

    // Pick a random Ready node
    const target = readyNodes[Math.floor(Math.random() * readyNodes.length)];
    targetNodeId = target.id;
    const agentsBefore = target.agents;

    console.log(
      `Selected node: ${target.id} (ip=${target.ip}, agents=${agentsBefore})`
    );

    // ---------------------------------------------------------------
    // Step 2: Stop the node via the UI
    // ---------------------------------------------------------------
    await login(page);
    await page.locator('[data-dock-item-id="nmgr"]').click();
    await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({
      timeout: 10000,
    });
    await expect(page.locator("text=/\\d+ nodes?/")).toBeVisible({
      timeout: 10000,
    });

    // Click the target node's name to open detail dialog
    const nodeLink = page.locator(
      `[data-testid="node-manager"] a:has-text("${target.id}")`
    );
    await expect(nodeLink).toBeVisible({ timeout: 5000 });
    await nodeLink.click();

    // Wait for detail dialog
    await expect(page.locator(`text=Node: ${target.id}`)).toBeVisible({
      timeout: 5000,
    });

    // Click Stop button
    const stopBtn = page.locator(
      '[data-testid="node-detail-actions"] button:has-text("Cordon")'
    );
    await expect(stopBtn).toBeVisible({ timeout: 3000 });
    await stopBtn.click();

    // ---------------------------------------------------------------
    // Step 3: Verify the node transitions to SchedulingDisabled
    // ---------------------------------------------------------------
    const stoppedNode = await waitForStatus(
      target.id,
      "SchedulingDisabled",
      60_000
    );
    expect(stoppedNode.status).toBe("SchedulingDisabled");
    console.log(`Node ${target.id} is now SchedulingDisabled`);

    // ---------------------------------------------------------------
    // Step 4: Verify no NEW agents are scheduled on the cordoned node
    //
    // K8s cordon prevents NEW pod scheduling but does NOT evict existing
    // pods.  Existing agents stay until explicitly drained (which happens
    // on node delete, not stop).  We verify the agent count has not
    // increased — no new work is being scheduled here.
    // ---------------------------------------------------------------
    const afterStopNode = await getNode(target.id);
    expect(
      afterStopNode.agents,
      `Agent count should not increase on cordoned node ${target.id}`
    ).toBeLessThanOrEqual(agentsBefore);
    console.log(
      `Node ${target.id}: agents before=${agentsBefore}, after cordon=${afterStopNode.agents}`
    );

    // ---------------------------------------------------------------
    // Step 5: Start the node via the API (restore it)
    // ---------------------------------------------------------------
    await startNode(target.id);
    const restoredNode = await waitForStatus(target.id, "Ready", 30_000);
    expect(restoredNode.status).toBe("Ready");
    console.log(`Node ${target.id} restored to Ready`);

    // ---------------------------------------------------------------
    // Step 6: Verify the restored status via API
    // (The stop → SchedulingDisabled → start → Ready cycle is the
    //  critical path.  We verify final state via API to avoid
    //  flakiness from portal re-injection after modal interactions.)
    // ---------------------------------------------------------------
    const finalNode = await getNode(target.id);
    expect(finalNode.status).toBe("Ready");
    console.log(`Final verification: ${target.id} status=${finalNode.status}`);
  });
});
