/**
 * Node Manager Applet — Post-Deployment Verification Tests
 *
 * Validates that the Node Manager applet loads, displays node data from the
 * API, and supports CRUD + lifecycle operations (provision, detail view,
 * start/stop, delete, notes).
 *
 * Selector strategy: SpecifyJS components do not pass through data-testid
 * attributes, so tests use text/role-based selectors.  Wrapper divs with
 * data-testid are used where raw elements are available.
 */

import { test, expect, Page } from "@playwright/test";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Log in and navigate to the desktop */
async function login(page: Page): Promise<void> {
  await page.goto("/");
  await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });
  await page.locator('input[placeholder="Username"]').fill("admin");
  await page.locator('input[placeholder="Password"]').fill("test");
  await page.locator('input[placeholder="MFA Token"]').fill("123456");
  await page.locator('button:has-text("Sign In")').click();
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({ timeout: 15000 });
}

/** Open the Node Manager applet via the dock and wait for content injection */
async function openNodeManager(page: Page): Promise<void> {
  await page.locator('[data-dock-item-id="nmgr"]').click();
  // Wait for the window to appear
  await expect(
    page.locator('[role="dialog"][aria-label="Node Manager"]')
  ).toBeVisible({ timeout: 5000 });
  // Wait for the Node Manager component to be injected (AppletPortal)
  await expect(
    page.locator('[data-testid="node-manager"]')
  ).toBeVisible({ timeout: 10000 });
}

// ---------------------------------------------------------------------------
// Tests: Node Manager loads and displays data
// ---------------------------------------------------------------------------

test.describe("Node Manager applet", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await openNodeManager(page);
  });

  test("displays the toolbar with Nodes title and action buttons", async ({ page }) => {
    // The toolbar should show "Nodes" title
    await expect(page.locator('[data-testid="node-manager"] >> text=Nodes').first()).toBeVisible();

    // The Provision Node button should be visible (text-based selector)
    await expect(
      page.locator('[data-testid="node-manager"] button:has-text("Provision Node")')
    ).toBeVisible({ timeout: 5000 });

    // The Refresh button should be visible
    await expect(
      page.locator('[data-testid="node-manager"] button:has-text("Refresh")')
    ).toBeVisible();
  });

  test("shows node count in the footer", async ({ page }) => {
    // Wait for loading to complete — the footer shows "N node(s)"
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
  });

  test("refresh button reloads data without error", async ({ page }) => {
    // Wait for initial load
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });

    // Click refresh
    await page.locator('[data-testid="node-manager"] button:has-text("Refresh")').click();

    // Wait a bit for reload
    await page.waitForTimeout(1000);

    // Should still show node count (no error banner)
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
  });

  test("node list shows a table or grid", async ({ page }) => {
    // Wait for data to load
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });

    // A table should be present inside the node manager
    const table = page.locator('[data-testid="node-manager"] table');
    await expect(table).toBeVisible({ timeout: 5000 });
  });
});

// ---------------------------------------------------------------------------
// Tests: Provision dialog
// ---------------------------------------------------------------------------

test.describe("Node Manager provision dialog", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await openNodeManager(page);
  });

  test("opens when Provision Node is clicked", async ({ page }) => {
    await page.locator('[data-testid="node-manager"] button:has-text("Provision Node")').click();

    // Modal should appear with title "Provision Node"
    await expect(page.locator('text=Provision Node').first()).toBeVisible({ timeout: 5000 });

    // Fields should be present (Name is optional, Host and User are required)
    await expect(page.locator('input[placeholder*="Node Name"]')).toBeVisible();
    await expect(page.locator('input[placeholder="Host (IP or FQDN)"]')).toBeVisible();
    await expect(page.locator('input[placeholder="SSH Username"]')).toBeVisible();
  });

  test("validates required fields on submit", async ({ page }) => {
    await page.locator('[data-testid="node-manager"] button:has-text("Provision Node")').click();
    await expect(page.locator('text=Provision Node').first()).toBeVisible({ timeout: 5000 });

    // Submit without filling required fields
    await page.locator('button:has-text("Provision")').last().click();
    await expect(page.locator('text=Host is required')).toBeVisible();
  });

  test("can be cancelled", async ({ page }) => {
    await page.locator('[data-testid="node-manager"] button:has-text("Provision Node")').click();
    await expect(page.locator('input[placeholder="Host (IP or FQDN)"]')).toBeVisible({ timeout: 5000 });

    // Click cancel
    await page.locator('button:has-text("Cancel")').click();

    // Dialog should close — the host input should no longer be visible
    await expect(page.locator('input[placeholder="Host (IP or FQDN)"]')).not.toBeVisible();
  });

  test("rejects empty host with user filled", async ({ page }) => {
    await page.locator('[data-testid="node-manager"] button:has-text("Provision Node")').click();
    await expect(page.locator('input[placeholder="SSH Username"]')).toBeVisible({ timeout: 5000 });

    // Fill only user
    await page.locator('input[placeholder="SSH Username"]').fill("ubuntu");
    await page.locator('button:has-text("Provision")').last().click();

    // Validation error
    await expect(page.locator('text=Host is required')).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Tests: Node detail dialog
// ---------------------------------------------------------------------------

test.describe("Node Manager detail dialog", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await openNodeManager(page);
  });

  test("clicking a node name opens the detail dialog", async ({ page }) => {
    // Wait for data to load
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });

    // Click the first node link (styled as <a> in the grid)
    const firstNodeLink = page.locator('[data-testid="node-manager"] a').first();
    await expect(firstNodeLink).toBeVisible({ timeout: 5000 });
    const nodeId = await firstNodeLink.textContent();
    await firstNodeLink.click();

    // Detail dialog should open with node ID in title
    await expect(page.locator(`text=Node: ${nodeId}`)).toBeVisible({ timeout: 5000 });
  });

  test("detail dialog shows Overview, Notes, Agents tabs", async ({ page }) => {
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
    await page.locator('[data-testid="node-manager"] a').first().click();

    // Wait for detail to load
    await expect(page.locator('text=/Node: /')).toBeVisible({ timeout: 5000 });

    // All three tabs
    await expect(page.locator('[role="tab"]:has-text("Overview")').first()).toBeVisible();
    await expect(page.locator('[role="tab"]:has-text("Notes")')).toBeVisible();
    await expect(page.locator('[role="tab"]:has-text("Agents")')).toBeVisible();
  });

  test("overview tab shows resource information", async ({ page }) => {
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
    await page.locator('[data-testid="node-manager"] a').first().click();
    await expect(page.locator('text=/Node: /')).toBeVisible({ timeout: 5000 });

    // Overview should show key fields (use exact text match to avoid grid header conflict)
    await expect(page.getByText('STATUS', { exact: true })).toBeVisible();
    await expect(page.getByText('IP ADDRESS', { exact: true })).toBeVisible();
    await expect(page.getByText('MEMORY', { exact: true })).toBeVisible();
    await expect(page.getByText('DISK', { exact: true })).toBeVisible();
  });

  test("detail dialog has Start or Stop button and Delete button", async ({ page }) => {
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
    await page.locator('[data-testid="node-manager"] a').first().click();
    await expect(page.locator('text=/Node: /')).toBeVisible({ timeout: 5000 });

    // Should have action buttons wrapper
    await expect(page.locator('[data-testid="node-detail-actions"]')).toBeVisible();

    // Should have either Start or Stop, and always Delete
    const hasStart = await page.locator('[data-testid="node-detail-actions"] button:has-text("Start")').isVisible();
    const hasStop = await page.locator('[data-testid="node-detail-actions"] button:has-text("Stop")').isVisible();
    expect(hasStart || hasStop).toBeTruthy();

    await expect(
      page.locator('[data-testid="node-detail-actions"] button:has-text("Delete")')
    ).toBeVisible();
  });

  test("delete button shows confirmation dialog", async ({ page }) => {
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
    await page.locator('[data-testid="node-manager"] a').first().click();
    await expect(page.locator('[data-testid="node-detail-actions"]')).toBeVisible({ timeout: 5000 });

    // Click Delete
    await page.locator('[data-testid="node-detail-actions"] button:has-text("Delete")').click();

    // Confirmation should appear
    await expect(page.locator('text=Confirm Delete')).toBeVisible({ timeout: 3000 });
    await expect(page.locator('text=Are you sure')).toBeVisible();

    // Cancel should dismiss
    await page.locator('button:has-text("Cancel")').last().click();
    await expect(page.locator('text=Confirm Delete')).not.toBeVisible();
  });

  test("detail dialog can be closed", async ({ page }) => {
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
    await page.locator('[data-testid="node-manager"] a').first().click();

    const detailTitle = page.locator('text=/Node: /');
    await expect(detailTitle).toBeVisible({ timeout: 5000 });

    // Close via escape or close button
    await page.keyboard.press("Escape");
    await expect(detailTitle).not.toBeVisible({ timeout: 3000 });
  });
});

// ---------------------------------------------------------------------------
// Tests: Notes tab
// ---------------------------------------------------------------------------

test.describe("Node Manager notes tab", () => {
  test("notes tab shows add-note form", async ({ page }) => {
    await login(page);
    await openNodeManager(page);

    // Open first node detail
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
    await page.locator('[data-testid="node-manager"] a').first().click();
    await expect(page.locator('text=/Node: /')).toBeVisible({ timeout: 5000 });

    // Switch to Notes tab
    await page.locator('[role="tab"]:has-text("Notes")').click();

    // Should show note input
    await expect(page.locator('input[placeholder="Add a note..."]')).toBeVisible({ timeout: 3000 });
    await expect(page.locator('button:has-text("Add")')).toBeVisible();
  });
});
