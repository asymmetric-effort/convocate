/**
 * Convocate Phase 1 — Applet Window PDV Tests
 *
 * Validates that all 7 applet dock icons are present after login, and that
 * clicking each icon opens a window which can be maximized, minimized
 * (via dock re-click to restore), and closed.
 *
 * These tests run against the deployed K8s environment.
 */

import { test, expect, Page } from "@playwright/test";

// ---------------------------------------------------------------------------
// Applet definitions — must match ui/src/app.ts APPLETS array
// ---------------------------------------------------------------------------

const APPLETS = [
  { id: "nmgr", label: "Node Manager" },
  { id: "amgr", label: "Agent Manager" },
  { id: "pb", label: "Convocate Project Board" },
  { id: "ide", label: "Code Monkey IDE" },
  { id: "ac", label: "Access Control" },
  { id: "repo", label: "Repo Manager" },
  { id: "sup", label: "Support Tool" },
];

// ---------------------------------------------------------------------------
// Helper: log in and reach the desktop
// ---------------------------------------------------------------------------

async function loginAndGetDesktop(page: Page): Promise<void> {
  await page.goto("/");

  // Wait for the login form to render
  await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });

  // Fill credentials — the API accepts admin/test/123456 in dev/test
  await page.locator('input[placeholder="Username"]').fill("admin");
  await page.locator('input[placeholder="Password"]').fill("test");
  await page.locator('input[placeholder="MFA Token"]').fill("123456");

  // Submit login
  await page.locator('button:has-text("Sign In")').click();

  // Wait for desktop to appear — the dock should be visible
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({
    timeout: 15000,
  });
}

// ---------------------------------------------------------------------------
// Test: all 7 dock icons are present after login
// ---------------------------------------------------------------------------

test("all 7 applet dock icons are present after login", async ({ page }) => {
  await loginAndGetDesktop(page);

  for (const applet of APPLETS) {
    const dockItem = page.locator(`[data-dock-item-id="${applet.id}"]`);
    await expect(dockItem).toBeVisible();
  }
});

// ---------------------------------------------------------------------------
// Per-applet window lifecycle tests: open, maximize, minimize, close
// ---------------------------------------------------------------------------

for (const applet of APPLETS) {
  test.describe(`${applet.label} applet window`, () => {
    test.beforeEach(async ({ page }) => {
      await loginAndGetDesktop(page);
    });

    test(`opens when dock icon is clicked`, async ({ page }) => {
      // Click the dock icon to open the applet window
      await page.locator(`[data-dock-item-id="${applet.id}"]`).click();

      // A dialog window with the applet label should appear
      const window = page.locator(`[role="dialog"][aria-label="${applet.label}"]`);
      await expect(window).toBeVisible({ timeout: 5000 });

      // Window should be in normal (non-maximized) state
      await expect(window).toHaveClass(/draggable-window--normal/);
    });

    test(`maximizes and restores`, async ({ page }) => {
      // Open the applet
      await page.locator(`[data-dock-item-id="${applet.id}"]`).click();
      const window = page.locator(`[role="dialog"][aria-label="${applet.label}"]`);
      await expect(window).toBeVisible({ timeout: 5000 });

      // Click maximize button
      const maximizeBtn = window.locator('button[aria-label="Maximize"]');
      await maximizeBtn.click();

      // Window should now be maximized
      await expect(window).toHaveClass(/draggable-window--maximized/);

      // Click restore button (same button, label changes to "Restore")
      const restoreBtn = window.locator('button[aria-label="Restore"]');
      await restoreBtn.click();

      // Window should return to normal state
      await expect(window).toHaveClass(/draggable-window--normal/);
    });

    test(`minimizes and restores via dock`, async ({ page }) => {
      // Open the applet
      await page.locator(`[data-dock-item-id="${applet.id}"]`).click();
      const window = page.locator(`[role="dialog"][aria-label="${applet.label}"]`);
      await expect(window).toBeVisible({ timeout: 5000 });

      // Click minimize button — window should disappear from view
      const minimizeBtn = window.locator('button[aria-label="Minimize"]');
      await minimizeBtn.click();
      await expect(window).not.toBeVisible();

      // Click dock icon again — window should reappear
      await page.locator(`[data-dock-item-id="${applet.id}"]`).click();
      await expect(window).toBeVisible({ timeout: 5000 });
    });

    test(`closes when close button is clicked`, async ({ page }) => {
      // Open the applet
      await page.locator(`[data-dock-item-id="${applet.id}"]`).click();
      const window = page.locator(`[role="dialog"][aria-label="${applet.label}"]`);
      await expect(window).toBeVisible({ timeout: 5000 });

      // Click close button — window should be removed from DOM
      const closeBtn = window.locator('button[aria-label="Close"]');
      await closeBtn.click();
      await expect(window).not.toBeVisible();
    });

    test(`can reopen after closing`, async ({ page }) => {
      // Open the applet
      await page.locator(`[data-dock-item-id="${applet.id}"]`).click();
      const window = page.locator(`[role="dialog"][aria-label="${applet.label}"]`);
      await expect(window).toBeVisible({ timeout: 5000 });

      // Close it
      await window.locator('button[aria-label="Close"]').click();
      await expect(window).not.toBeVisible();

      // Reopen via dock icon
      await page.locator(`[data-dock-item-id="${applet.id}"]`).click();

      // A new window should appear (may have a different instance id)
      const reopened = page.locator(`[role="dialog"][aria-label="${applet.label}"]`);
      await expect(reopened).toBeVisible({ timeout: 5000 });
    });
  });
}

// ---------------------------------------------------------------------------
// Sad-path: verify no window opens for a non-existent applet id
// ---------------------------------------------------------------------------

test("clicking a non-dock area does not open any window", async ({ page }) => {
  await loginAndGetDesktop(page);

  // Count existing dialog windows
  const before = await page.locator('[role="dialog"]').count();

  // Click the desktop background (not a dock icon)
  await page.locator(".unity-desktop__workspace").click({ position: { x: 400, y: 300 } });

  // No new windows should have appeared
  const after = await page.locator('[role="dialog"]').count();
  expect(after).toBe(before);
});
