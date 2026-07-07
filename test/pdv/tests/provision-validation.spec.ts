/**
 * Provision Node Input Validation — Post-Deployment Verification Tests
 *
 * Validates that the provision form and API properly validate and sanitize
 * all inputs.  Covers both happy path (valid inputs accepted) and sad path
 * (invalid inputs rejected with clear error messages).
 */

import { test, expect, Page } from "@playwright/test";

// Disable TLS for direct API calls

const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

// ---------------------------------------------------------------------------
// Cleanup: delete any test-* or cleanup-* nodes left over after tests
// ---------------------------------------------------------------------------

test.afterAll(async () => {
  const res = await fetch(`${BASE}/api/v1/nmgr/node?limit=200`, {
    headers: authHeaders(),
  });
  if (!res.ok) return;
  const page = await res.json();
  for (const node of page.items || []) {
    if (node.id.startsWith("test-") || node.id.startsWith("cleanup-")) {
      await fetch(`${BASE}/api/v1/nmgr/node/${node.id}`, {
        method: "DELETE",
        headers: authHeaders(),
      }).catch(() => {});
    }
  }
});

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
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({ timeout: 15000 });
}

async function openProvisionDialog(page: Page): Promise<void> {
  await page.locator('[data-dock-item-id="nmgr"]').click();
  await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
  await expect(page.locator("text=/\\d+ nodes?/")).toBeVisible({ timeout: 10000 });
  await page.locator('[data-testid="node-manager"] button:has-text("Provision Node")').click();
  await expect(page.locator('input[placeholder*="Node Name"]')).toBeVisible({ timeout: 5000 });
}

function authHeaders(): Record<string, string> {
  return {
    "Content-Type": "application/json",
    Authorization: "Bearer mock-token",
  };
}

// ---------------------------------------------------------------------------
// UI Validation — Sad Path
// ---------------------------------------------------------------------------

test.describe("Provision form UI validation", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await openProvisionDialog(page);
  });

  test("rejects empty host", async ({ page }) => {
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=Host is required")).toBeVisible();
  });

  test("rejects empty SSH username", async ({ page }) => {
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.20");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=SSH Username is required")).toBeVisible();
  });

  test("rejects node name with uppercase letters", async ({ page }) => {
    await page.locator('input[placeholder*="Node Name"]').fill("MyNode");
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.20");
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/lowercase/")).toBeVisible();
  });

  test("rejects node name starting with hyphen", async ({ page }) => {
    await page.locator('input[placeholder*="Node Name"]').fill("-badname");
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.20");
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/hyphen/")).toBeVisible();
  });

  test("rejects node name ending with hyphen", async ({ page }) => {
    await page.locator('input[placeholder*="Node Name"]').fill("badname-");
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.20");
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/hyphen/")).toBeVisible();
  });

  test("rejects duplicate node name", async ({ page }) => {
    // convocate01 already exists
    await page.locator('input[placeholder*="Node Name"]').fill("convocate01");
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.99");
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/already exists/")).toBeVisible();
  });

  test("rejects invalid IP address (octet > 255)", async ({ page }) => {
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.999");
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/Invalid IP|octets/")).toBeVisible();
  });

  test("rejects invalid host format", async ({ page }) => {
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("not a valid host!");
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/valid IPv4|FQDN/")).toBeVisible();
  });

  test("rejects duplicate IP address", async ({ page }) => {
    // 192.168.56.11 is convocate01's IP
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.11");
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/already exists/")).toBeVisible();
  });

  test("rejects invalid SSH username", async ({ page }) => {
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill("192.168.56.20");
    await page.locator('input[placeholder="SSH Username"]').fill("bad user!");
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator("text=/valid Linux username/")).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// UI Validation — Happy Path
// ---------------------------------------------------------------------------

test.describe("Provision form UI validation happy path", () => {
  test("accepts valid inputs without client-side error", async ({ page }) => {
    await login(page);
    await openProvisionDialog(page);

    // Use timestamp-based values to avoid conflicts from previous test runs
    const ts = Date.now().toString(36);
    const uniqueName = `test-${ts}`;
    // Generate a random private IP in the 10.x.x.x range to avoid conflicts
    const octet3 = Math.floor(Math.random() * 254) + 1;
    const octet4 = Math.floor(Math.random() * 254) + 1;
    const uniqueIP = `10.200.${octet3}.${octet4}`;
    await page.locator('input[placeholder*="Node Name"]').fill(uniqueName);
    await page.locator('input[placeholder="Host (IP or FQDN)"]').fill(uniqueIP);
    await page.locator('input[placeholder="SSH Username"]').fill("convocate");
    await page.locator('input[placeholder="Password (optional)"]').fill("secret");
    await page.locator('input[placeholder="Location (optional)"]').fill("rack2");
    await page.locator('input[placeholder*="Tags"]').fill("test,worker");

    await page.locator('button:has-text("Provision")').last().click();

    // Should NOT show any client-side validation error
    // (may show API error since the host doesn't exist, but no UI validation error)
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Host is required")).not.toBeVisible();
    await expect(page.locator("text=SSH Username is required")).not.toBeVisible();
    await expect(page.locator("text=/lowercase/")).not.toBeVisible();
    await expect(page.locator("text=/already exists/")).not.toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Cleanup — deleting pending/error nodes that only exist in the store
// ---------------------------------------------------------------------------

test.describe("Provision cleanup", () => {
  test("can delete a pending/error node that does not exist in K8s", async ({ page }) => {
    // Create a node that will go to Error status (unreachable host)
    const name = `cleanup-${Date.now().toString(36)}`;
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ name, host: "10.254.254.254", user: "nobody" }),
    });
    expect(res.status).toBe(202);

    // Verify it exists in the API
    const getRes = await fetch(`${BASE}/api/v1/nmgr/node/${name}`, {
      headers: authHeaders(),
    });
    expect(getRes.status).toBe(200);

    // Delete it — should succeed (not 500)
    const delRes = await fetch(`${BASE}/api/v1/nmgr/node/${name}`, {
      method: "DELETE",
      headers: authHeaders(),
    });
    expect(delRes.status).toBe(202);

    // Verify it's gone
    const verifyRes = await fetch(`${BASE}/api/v1/nmgr/node/${name}`, {
      headers: authHeaders(),
    });
    expect(verifyRes.status).toBe(404);
  });
});

// ---------------------------------------------------------------------------
// API Validation — Sad Path (direct API calls)
// ---------------------------------------------------------------------------

test.describe("Provision API validation", () => {
  test("rejects missing host", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ user: "convocate" }),
    });
    expect(res.status).toBe(400);
    const body = await res.json();
    expect(body.message).toContain("host is required");
  });

  test("rejects missing user", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ host: "192.168.56.99" }),
    });
    expect(res.status).toBe(400);
    const body = await res.json();
    expect(body.message).toContain("user is required");
  });

  test("rejects node name with invalid characters", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ name: "Bad_Name!", host: "192.168.56.99", user: "convocate" }),
    });
    expect(res.status).toBe(400);
    const body = await res.json();
    expect(body.message).toContain("lowercase");
  });

  test("rejects node name starting with hyphen", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ name: "-badname", host: "192.168.56.99", user: "convocate" }),
    });
    expect(res.status).toBe(400);
    const body = await res.json();
    expect(body.message).toContain("hyphen");
  });

  test("rejects duplicate node name (existing K8s node)", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ name: "convocate01", host: "192.168.56.99", user: "convocate" }),
    });
    expect(res.status).toBe(409);
    const body = await res.json();
    expect(body.message).toContain("already exists");
  });

  test("rejects duplicate IP (existing K8s node)", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ host: "192.168.56.11", user: "convocate" }),
    });
    expect(res.status).toBe(409);
    const body = await res.json();
    expect(body.message).toContain("already exists");
  });

  test("rejects node name over 63 characters", async () => {
    const longName = "a".repeat(64);
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ name: longName, host: "192.168.56.99", user: "convocate" }),
    });
    expect(res.status).toBe(400);
    const body = await res.json();
    expect(body.message).toContain("63 characters");
  });
});
