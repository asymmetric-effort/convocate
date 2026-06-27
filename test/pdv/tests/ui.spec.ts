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

  test("dock click opens Node Manager with K8s data", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    // Click dock icon via mouse coordinates
    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width/2, box.y + box.height/2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("Node Manager");
    expect(text).toContain("nodes");
  });

  test("all 7 dock icons present after login", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icons = await page.locator("[class*='dock'] img, [class*='launcher'] img").count();
    expect(icons).toBeGreaterThanOrEqual(7);
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

  // -- Applet-specific tests --------------------------------------------------

  test("Node Manager shows DataGrid with node data", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Should show node count and Provision button
    const text = await page.textContent("body");
    expect(text).toContain("nodes");
    expect(text).toContain("Provision Node");
  });

  test("Node Manager Provision button opens dialog", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Click Provision Node button
    await page.locator("button").filter({ hasText: /Provision Node/i }).click();
    await page.waitForTimeout(500);

    // Should show dialog with form fields
    const text = await page.textContent("body");
    expect(text).toContain("Host");
    expect(text).toContain("SSH User");
  });

  test("Node Manager shows action buttons per node", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Should have Cordon/Delete action buttons
    const text = await page.textContent("body");
    expect(text?.includes("Cordon") || text?.includes("Uncordon")).toBeTruthy();
    expect(text).toContain("Delete");
  });

  test("Agent Manager shows Create Agent button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Agent Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("agent-containers");
    expect(text).toContain("Create Agent");
  });

  test("Access Control shows tabs and Create User button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Access Control']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("Users");
    expect(text).toContain("Groups");
    expect(text).toContain("Global Settings");
    expect(text).toContain("Create User");
  });

  test("Repo Manager shows repos and Create Repo button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Repo Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("repositories");
    expect(text).toContain("Create Repo");
  });

  test("Support Tool shows tickets tab and New Ticket button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Support Tool']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("Tickets");
    expect(text).toContain("Documentation");
    expect(text).toContain("New Ticket");
  });

  test("Project Board shows boards and New Board button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Convocate Project Board']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("boards");
    expect(text).toContain("New Board");
  });

  test("Code IDE opens and renders editor", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Code IDE']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Code IDE should render the framework IDE component
    const text = await page.textContent("body");
    expect(text).toContain("Code IDE");
  });
});
