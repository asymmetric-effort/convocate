/**
 * RBAC — Post-Deployment Verification Tests
 *
 * Validates that RBAC middleware is wired up on all API routes
 * and that the UI correctly gates action buttons by role.
 */

import { test, expect, Page } from "@playwright/test";


const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

function authHeaders() {
  return { "Content-Type": "application/json", Authorization: "Bearer mock-token" };
}

async function login(page: Page): Promise<void> {
  await page.goto("/");
  await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });
  await page.locator('input[placeholder="Username"]').fill("admin");
  await page.locator('input[placeholder="Password"]').fill("test");
  await page.locator('input[placeholder="MFA Token"]').fill("123456");
  await page.locator('button:has-text("Sign In")').click();
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({ timeout: 15000 });
}

// ---------------------------------------------------------------------------
// API RBAC: verify all protected routes return 200 for admin
// ---------------------------------------------------------------------------

test.describe("RBAC API enforcement", () => {
  test("unauthenticated request returns 401", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, {
      headers: { "Content-Type": "application/json" },
    });
    expect(res.status).toBe(401);
  });

  test("authenticated admin can access node manager", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("authenticated admin can access agent manager", async () => {
    const res = await fetch(`${BASE}/api/v1/amgr/agent`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("authenticated admin can access project board", async () => {
    const res = await fetch(`${BASE}/api/v1/pb/board`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("authenticated admin can access IDE projects", async () => {
    const res = await fetch(`${BASE}/api/v1/ide/project`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("authenticated admin can access unified projects", async () => {
    const res = await fetch(`${BASE}/api/v1/projects`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("authenticated admin can access access control", async () => {
    const res = await fetch(`${BASE}/api/v1/ac/user`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("authenticated admin can access repo manager", async () => {
    const res = await fetch(`${BASE}/api/v1/repo/repo`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("authenticated admin can access support tool", async () => {
    const res = await fetch(`${BASE}/api/v1/sup/ticket`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });
});

// ---------------------------------------------------------------------------
// UI RBAC: verify action buttons are visible for admin user
// ---------------------------------------------------------------------------

test.describe("RBAC UI button gating", () => {
  test("admin sees Create Agent button", async ({ page }) => {
    await login(page);
    await page.locator('[data-dock-item-id="amgr"]').click();
    await expect(page.locator('[data-testid="agent-manager"]')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('button:has-text("Create Agent")')).toBeVisible({ timeout: 5000 });
  });

  test("admin sees New Card and New Project on Project Board", async ({ page }) => {
    await login(page);
    await page.locator('[data-dock-item-id="pb"]').click();
    await expect(page.locator('[data-testid="project-board"]')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('button:has-text("New Card")')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('button:has-text("New Project")')).toBeVisible();
  });

  test("admin sees all 7 dock items", async ({ page }) => {
    await login(page);
    // Admin should see all 7 applets
    await expect(page.locator("[data-dock-item-id]")).toHaveCount(7, { timeout: 5000 });
  });
});
