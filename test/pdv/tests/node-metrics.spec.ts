/**
 * Node Metrics Live Updates — Post-Deployment Verification Tests
 *
 * Validates that the Node Manager receives real-time metrics from the
 * DaemonSet via the events API and displays them in the grid without
 * requiring a manual refresh.
 */

import { test, expect, Page } from "@playwright/test";

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

async function openNodeManager(page: Page): Promise<void> {
  await page.locator('[data-dock-item-id="nmgr"]').click();
  await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({
    timeout: 10000,
  });
  await expect(page.locator("text=/\\d+ nodes?/")).toBeVisible({
    timeout: 10000,
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe("Node metrics live updates", () => {
  test("grid shows real load average values (not dashes)", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    // Wait for data to include load values in "X.XX / X.XX / X.XX" format
    const loadCell = page.locator('[data-testid="node-manager"] table td').filter({
      hasText: /^\d+\.\d{2} \/ \d+\.\d{2} \/ \d+\.\d{2}$/,
    }).first();
    await expect(loadCell).toBeVisible({ timeout: 15000 });
  });

  test("grid shows real memory values (not dashes)", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    // Memory column shows "X.X / Y.Y GB" format
    const memCell = page.locator('[data-testid="node-manager"] table td').filter({
      hasText: /\d+\.\d+ \/ \d+\.\d+ GB/,
    }).first();
    await expect(memCell).toBeVisible({ timeout: 15000 });
  });

  test("grid shows real disk values (not dashes)", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    // Disk column shows "X.X / 57.1 GB" format
    const diskCell = page.locator('[data-testid="node-manager"] table td').filter({
      hasText: /\d+\.\d+ \/ 57/,
    }).first();
    await expect(diskCell).toBeVisible({ timeout: 15000 });
  });

  test("connection indicator is visible", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    const indicator = page.locator('[data-testid="ws-indicator"]');
    await expect(indicator).toBeVisible({ timeout: 5000 });

    // Wait for the events stream to connect
    await page.waitForTimeout(5000);

    // Indicator should be green (connected) — rgb(34, 197, 94)
    const bgColor = await indicator.evaluate(
      (el) => window.getComputedStyle(el).backgroundColor
    );
    expect(
      bgColor.includes("34, 197, 94"),
      `Expected green indicator but got ${bgColor}`
    ).toBeTruthy();
  });

  test("metrics update automatically within 10 seconds", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    // Wait for initial data to render
    await page.waitForTimeout(3000);

    // Capture a snapshot of all cell text
    const getCells = () =>
      page.locator('[data-testid="node-manager"] table tbody td').allTextContents();

    const initial = await getCells();

    // Wait 8 seconds (at least two 3-second metric cycles)
    await page.waitForTimeout(8000);

    const updated = await getCells();

    // At least one numeric cell should have changed (load averages fluctuate)
    let changed = false;
    for (let i = 0; i < Math.min(initial.length, updated.length); i++) {
      if (initial[i] !== updated[i] && /\d/.test(initial[i])) {
        changed = true;
        break;
      }
    }
    expect(changed, "Metrics should auto-update without manual refresh").toBeTruthy();
  });

  test("grid shows 1m/5m/15m load averages", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    // Wait for data to load with load values in "X.XX / X.XX / X.XX" format
    const loadCell = page.locator('[data-testid="node-manager"] table td').filter({
      hasText: /^\d+\.\d{2} \/ \d+\.\d{2} \/ \d+\.\d{2}$/,
    }).first();
    await expect(loadCell).toBeVisible({ timeout: 15000 });

    // Verify the three values are present
    const text = await loadCell.textContent();
    const parts = text!.split(" / ");
    expect(parts.length, "Load average should have 3 values (1m/5m/15m)").toBe(3);

    // All three should be non-negative numbers
    for (const p of parts) {
      const val = parseFloat(p);
      expect(val, `Load average value "${p}" should be >= 0`).toBeGreaterThanOrEqual(0);
    }
  });

  test("no metric columns show impossible negative values", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    await page.waitForTimeout(5000);

    const cells = await page
      .locator('[data-testid="node-manager"] td')
      .allTextContents();
    for (const text of cells) {
      expect(text, `Found impossible negative metric: "${text}"`).not.toMatch(
        /^-\d/
      );
    }
  });
});
