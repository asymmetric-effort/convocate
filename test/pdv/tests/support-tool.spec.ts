/**
 * Support Tool Applet — PDV Tests
 */
import { test, expect, Page } from "@playwright/test";
process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";
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

async function openSupport(page: Page) {
  await page.locator('[data-dock-item-id="sup"]').click();
  await expect(page.locator('[role="dialog"][aria-label="Support Tool"]')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('[data-testid="support-tool"]')).toBeVisible({ timeout: 10000 });
}

test.describe("Support Tool applet", () => {
  test("shows Tickets and Documentation tabs", async ({ page }) => {
    await login(page); await openSupport(page);
    await expect(page.locator('[role="tab"]:has-text("Tickets")')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('[role="tab"]:has-text("Documentation")')).toBeVisible();
  });

  test("Tickets tab has New Ticket button", async ({ page }) => {
    await login(page); await openSupport(page);
    await expect(page.locator('button:has-text("New Ticket")')).toBeVisible({ timeout: 5000 });
  });

  test("New Ticket dialog validates subject", async ({ page }) => {
    await login(page); await openSupport(page);
    await page.locator('button:has-text("New Ticket")').click();
    await expect(page.locator('text=New Ticket').first()).toBeVisible({ timeout: 3000 });
    await page.locator('button:has-text("Submit")').click();
    await expect(page.locator('text=Subject is required')).toBeVisible();
    await page.locator('button:has-text("Cancel")').click();
  });
});

test.describe("Support Tool API", () => {
  test("list tickets returns results", async () => {
    const res = await fetch(`${BASE}/api/v1/sup/ticket?limit=10`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    expect((await res.json())).toHaveProperty("items");
  });
  test("list docs returns results", async () => {
    const res = await fetch(`${BASE}/api/v1/sup/doc?limit=10`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    expect((await res.json())).toHaveProperty("items");
  });
  test("can create a ticket", async () => {
    const res = await fetch(`${BASE}/api/v1/sup/ticket`, {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ subject: `PDV test ${Date.now()}`, priority: "low", body: "test" }),
    });
    expect(res.status).toBe(201);
    const ticket = await res.json();
    expect(ticket).toHaveProperty("id");
    expect(ticket.status).toBe("open");
  });
});
