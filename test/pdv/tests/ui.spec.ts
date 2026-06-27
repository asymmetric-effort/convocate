import { test, expect } from "@playwright/test";

const APP = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

test.describe("UI Post-Deployment Verification", () => {
  test("login page renders with form fields", async ({ page }) => {
    await page.goto(APP);
    // Wait for SpecifyJS to render the login form
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = await page.locator("input").count();
    expect(inputs).toBeGreaterThanOrEqual(3); // username, password, MFA
  });

  test("login with valid credentials shows desktop", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });

    // Fill login form
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");

    // Click sign in button
    await page.locator("button").filter({ hasText: /sign in/i }).click();

    // Wait for desktop to render (dock should appear)
    await page.waitForSelector("[class*='dock'], [class*='unity-desktop']", { timeout: 10000 });
  });

  test("dock shows applet icons after login", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });

    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();

    await page.waitForSelector("[class*='dock'], [class*='unity-desktop']", { timeout: 10000 });

    // Should have dock items (applet icons)
    const dockItems = page.locator("[class*='dock'] img, [class*='launcher'] img");
    await expect(dockItems.first()).toBeVisible({ timeout: 5000 });
  });

  test("clicking Node Manager dock icon opens window", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });

    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();

    await page.waitForSelector("[class*='dock'], [class*='unity-desktop']", { timeout: 10000 });

    // Click first dock icon (Node Manager)
    const firstIcon = page.locator("[class*='dock'] img, [class*='launcher'] img").first();
    await firstIcon.click();

    // A window should appear with Node Manager content
    await page.waitForSelector("[class*='window'], [class*='titlebar']", { timeout: 5000 });
  });

  test("healthz endpoint returns ok", async ({ request }) => {
    const res = await request.get(`${APP}/healthz`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.status).toBe("ok");
  });

  test("SPA fallback serves app for unknown paths", async ({ page }) => {
    await page.goto(`${APP}/some/unknown/path`);
    // Should still load the app (SPA fallback)
    await expect(page.locator("#app")).toBeAttached();
  });
});
