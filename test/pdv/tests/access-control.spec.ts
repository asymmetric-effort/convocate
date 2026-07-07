/**
 * Access Control Applet — PDV Tests
 */
import { test, expect, Page } from "@playwright/test";
const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";
function authHeaders() { return { "Content-Type": "application/json", Authorization: "Bearer mock-token" }; }

async function login(page: Page) {
  await page.goto("/");
  await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });
  await page.locator('input[placeholder="Username"]').fill("admin");
  await page.locator('input[placeholder="Password"]').fill("test");
  await page.locator('input[placeholder="MFA Token"]').fill("123456");
  await page.locator('button:has-text("Sign In")').click();
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({ timeout: 15000 });
}

async function openAC(page: Page) {
  await page.locator('[data-dock-item-id="ac"]').click();
  await expect(page.locator('[role="dialog"][aria-label="Access Control"]')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('[data-testid="access-control"]')).toBeVisible({ timeout: 10000 });
}

test.describe("Access Control applet", () => {
  test("shows Users, Groups, and Settings tabs", async ({ page }) => {
    await login(page); await openAC(page);
    await expect(page.locator('[role="tab"]:has-text("Users")')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('[role="tab"]:has-text("Groups")')).toBeVisible();
    await expect(page.locator('[role="tab"]:has-text("Settings")')).toBeVisible();
  });

  test("Users tab has Add User button", async ({ page }) => {
    await login(page); await openAC(page);
    await expect(page.locator('button:has-text("Add User")')).toBeVisible({ timeout: 5000 });
  });

  test("Groups tab has Add Group button", async ({ page }) => {
    await login(page); await openAC(page);
    await page.locator('[role="tab"]:has-text("Groups")').click();
    await expect(page.locator('button:has-text("Add Group")')).toBeVisible({ timeout: 5000 });
  });

  test("Settings tab shows MFA toggle", async ({ page }) => {
    await login(page); await openAC(page);
    await page.locator('[role="tab"]:has-text("Settings")').click();
    await expect(page.locator('text=Require MFA')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('button:has-text("Save Settings")')).toBeVisible();
  });
});

test.describe("Access Control API", () => {
  test("list users returns results", async () => {
    const res = await fetch(`${BASE}/api/v1/ac/user?limit=10`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    expect((await res.json())).toHaveProperty("items");
  });
  test("list groups returns results", async () => {
    const res = await fetch(`${BASE}/api/v1/ac/group?limit=10`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    expect((await res.json())).toHaveProperty("items");
  });
  test("get settings returns settings object", async () => {
    const res = await fetch(`${BASE}/api/v1/ac/settings`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    const s = await res.json();
    expect(s).toHaveProperty("requireMfa");
    expect(s).toHaveProperty("sessionTimeoutMinutes");
  });
});
