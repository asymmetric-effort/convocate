import { test, expect } from "@playwright/test";

const UI = process.env.UI_URL || "http://convocate-ui.convocate.svc:8080";

test.describe("UI Post-Deployment Verification", () => {
  test("UI healthz endpoint returns ok", async ({ request }) => {
    const res = await request.get(`${UI}/healthz`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.status).toBe("ok");
  });

  test("UI serves index.html", async ({ request }) => {
    const res = await request.get(`${UI}/`);
    expect(res.status()).toBe(200);
    const text = await res.text();
    expect(text).toContain("Convocate");
    expect(text).toContain("login");
  });

  test("login form is visible", async ({ page }) => {
    await page.goto(`${UI}/`);
    await expect(page.locator("#login")).toBeVisible();
    await expect(page.locator("#username")).toBeVisible();
    await expect(page.locator("#password")).toBeVisible();
    await expect(page.locator("#mfa")).toBeVisible();
  });

  test("login with empty credentials shows error", async ({ page }) => {
    await page.goto(`${UI}/`);
    await page.click("button");
    // Empty credentials should trigger an error (connection or 401)
    const error = page.locator("#error");
    await expect(error).toBeVisible({ timeout: 5000 });
  });
});
