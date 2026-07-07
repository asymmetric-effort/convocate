/**
 * Repo Manager Applet — PDV Tests
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

async function openRepo(page: Page) {
  await page.locator('[data-dock-item-id="repo"]').click();
  await expect(page.locator('[role="dialog"][aria-label="Repo Manager"]')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('[data-testid="repo-manager"]')).toBeVisible({ timeout: 10000 });
}

test.describe("Repo Manager applet", () => {
  test("shows repo selector and New Repo button", async ({ page }) => {
    await login(page); await openRepo(page);
    await expect(page.locator('[data-testid="repo-manager"] select')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('button:has-text("New Repo")')).toBeVisible();
  });

  test("has Files and Pull Requests tabs", async ({ page }) => {
    await login(page); await openRepo(page);
    await expect(page.locator('[role="tab"]:has-text("Files")')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('[role="tab"]:has-text("Pull Requests")')).toBeVisible();
  });

  test("New Repo dialog validates name", async ({ page }) => {
    await login(page); await openRepo(page);
    await page.locator('button:has-text("New Repo")').click();
    await expect(page.locator('text=New Repository').first()).toBeVisible({ timeout: 3000 });
    await page.locator('button:has-text("Create")').last().click();
    await expect(page.locator('text=Repository name is required')).toBeVisible();
    await page.locator('button:has-text("Cancel")').click();
  });
});

test.describe("Repo Manager API", () => {
  test("list repos returns results", async () => {
    const res = await fetch(`${BASE}/api/v1/repo/repo?limit=10`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    expect((await res.json())).toHaveProperty("items");
  });
});
