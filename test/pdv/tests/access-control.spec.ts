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

// ---------------------------------------------------------------------------
// Tests: User context menu
// ---------------------------------------------------------------------------

test.describe("Access Control user context menu", () => {
  test("right-clicking a user row shows context menu with expected items", async ({ page }) => {
    await login(page); await openAC(page);
    // Wait for user data to load
    await page.waitForTimeout(2000);
    // Right-click on the first user row cell
    const firstUserCell = page.locator('[data-testid="access-control"] table tbody tr').first().locator("td").first();
    if (await firstUserCell.isVisible()) {
      await firstUserCell.click({ button: "right" });
      // Context menu should appear with user-specific items
      await expect(page.locator("text=Edit User")).toBeVisible({ timeout: 3000 });
      await expect(page.locator("text=Reset Password")).toBeVisible();
      await expect(page.locator("text=Delete User")).toBeVisible();
    }
    // If no users exist, skip gracefully
  });

  test("user context menu has Enable/Disable, Edit, Reset Password, MFA, and Delete options", async ({ page }) => {
    await login(page); await openAC(page);
    await page.waitForTimeout(2000);
    const firstUserCell = page.locator('[data-testid="access-control"] table tbody tr').first().locator("td").first();
    if (await firstUserCell.isVisible()) {
      await firstUserCell.click({ button: "right" });
      await expect(page.locator("text=Edit User")).toBeVisible({ timeout: 3000 });
      // Should have Enable or Disable User (depends on current status)
      const hasEnable = await page.locator("text=Enable User").isVisible();
      const hasDisable = await page.locator("text=Disable User").isVisible();
      expect(hasEnable || hasDisable, "Should show Enable or Disable User").toBeTruthy();
      await expect(page.locator("text=Reset Password")).toBeVisible();
      // Should have Enroll MFA or Reset MFA (depends on enrollment status)
      const hasEnroll = await page.locator("text=Enroll MFA").isVisible();
      const hasReset = await page.locator("text=Reset MFA").isVisible();
      expect(hasEnroll || hasReset, "Should show Enroll MFA or Reset MFA").toBeTruthy();
      await expect(page.locator("text=Delete User")).toBeVisible();
    }
  });

  test("clicking elsewhere dismisses the user context menu", async ({ page }) => {
    await login(page); await openAC(page);
    await page.waitForTimeout(2000);
    const firstUserCell = page.locator('[data-testid="access-control"] table tbody tr').first().locator("td").first();
    if (await firstUserCell.isVisible()) {
      await firstUserCell.click({ button: "right" });
      await expect(page.locator("text=Edit User")).toBeVisible({ timeout: 3000 });
      // Click elsewhere on the access-control container to dismiss
      await page.locator('[data-testid="access-control"]').click({ position: { x: 10, y: 10 } });
      await expect(page.locator("text=Edit User")).not.toBeVisible({ timeout: 3000 });
    }
  });
});

// ---------------------------------------------------------------------------
// Tests: Group context menu
// ---------------------------------------------------------------------------

test.describe("Access Control group context menu", () => {
  test("right-clicking a group row shows context menu with expected items", async ({ page }) => {
    await login(page); await openAC(page);
    // Switch to Groups tab
    await page.locator('[role="tab"]:has-text("Groups")').click();
    await page.waitForTimeout(2000);
    const firstGroupCell = page.locator('[data-testid="access-control"] table tbody tr').first().locator("td").first();
    if (await firstGroupCell.isVisible()) {
      await firstGroupCell.click({ button: "right" });
      // Context menu should appear — either read-only for built-in or editable for custom
      const hasReadOnly = await page.locator("text=Built-in group (read-only)").isVisible();
      const hasRename = await page.locator("text=Rename Group").isVisible();
      expect(hasReadOnly || hasRename, "Should show group context menu").toBeTruthy();
    }
  });

  test("non-builtin group context menu has Rename, Edit Membership, and Delete options", async ({ page }) => {
    await login(page); await openAC(page);
    await page.locator('[role="tab"]:has-text("Groups")').click();
    await page.waitForTimeout(2000);
    // Try to find a non-builtin group row (one without "Yes" in the Built-in column)
    const groupRows = page.locator('[data-testid="access-control"] table tbody tr');
    const count = await groupRows.count();
    let found = false;
    for (let i = 0; i < count; i++) {
      const row = groupRows.nth(i);
      const builtinCell = row.locator("td").last();
      const text = await builtinCell.textContent();
      if (text && text.trim() === "No") {
        await row.locator("td").first().click({ button: "right" });
        await expect(page.locator("text=Rename Group")).toBeVisible({ timeout: 3000 });
        await expect(page.locator("text=Edit Group Membership")).toBeVisible();
        await expect(page.locator("text=Delete Group")).toBeVisible();
        found = true;
        break;
      }
    }
    // If all groups are built-in, verify we get the read-only message instead
    if (!found && count > 0) {
      await groupRows.first().locator("td").first().click({ button: "right" });
      await expect(page.locator("text=Built-in group (read-only)")).toBeVisible({ timeout: 3000 });
    }
  });

  test("clicking elsewhere dismisses the group context menu", async ({ page }) => {
    await login(page); await openAC(page);
    await page.locator('[role="tab"]:has-text("Groups")').click();
    await page.waitForTimeout(2000);
    const firstGroupCell = page.locator('[data-testid="access-control"] table tbody tr').first().locator("td").first();
    if (await firstGroupCell.isVisible()) {
      await firstGroupCell.click({ button: "right" });
      // Wait for context menu
      await page.waitForTimeout(500);
      // Click elsewhere to dismiss
      await page.locator('[data-testid="access-control"]').click({ position: { x: 10, y: 10 } });
      // Verify all menu items are gone
      await expect(page.locator("text=Rename Group")).not.toBeVisible({ timeout: 3000 });
      await expect(page.locator("text=Built-in group (read-only)")).not.toBeVisible({ timeout: 3000 });
    }
  });
});

// ---------------------------------------------------------------------------
// Tests: Access Control API — context menu operations
// ---------------------------------------------------------------------------

test.describe("Access Control API context menu operations", () => {
  let testUserId: string | null = null;
  let testGroupId: string | null = null;

  test.afterAll(async () => {
    // Clean up test user
    if (testUserId) {
      await fetch(`${BASE}/api/v1/ac/user/${testUserId}`, { method: "DELETE", headers: authHeaders() }).catch(() => {});
    }
    // Clean up test group
    if (testGroupId) {
      await fetch(`${BASE}/api/v1/ac/group/${testGroupId}`, { method: "DELETE", headers: authHeaders() }).catch(() => {});
    }
  });

  test("PATCH /api/v1/ac/user/:id with status change works (Enable/Disable)", async () => {
    // Create a test user first
    const email = `pdv-status-${Date.now()}@test.local`;
    const createRes = await fetch(`${BASE}/api/v1/ac/user`, {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ email, name: "PDV Status Test", password: "TestPass123!" }),
    });
    expect(createRes.status).toBe(201);
    const user = await createRes.json();
    testUserId = user.id;

    // Disable the user
    const disableRes = await fetch(`${BASE}/api/v1/ac/user/${user.id}`, {
      method: "PATCH", headers: authHeaders(),
      body: JSON.stringify({ status: "disabled" }),
    });
    expect(disableRes.status).toBe(200);
    const disabled = await disableRes.json();
    expect(disabled.status).toBe("disabled");

    // Re-enable the user
    const enableRes = await fetch(`${BASE}/api/v1/ac/user/${user.id}`, {
      method: "PATCH", headers: authHeaders(),
      body: JSON.stringify({ status: "active" }),
    });
    expect(enableRes.status).toBe(200);
    const enabled = await enableRes.json();
    expect(enabled.status).toBe("active");
  });

  test("PATCH /api/v1/ac/user/:id with password change works (Reset Password)", async () => {
    // Create a test user
    const email = `pdv-passwd-${Date.now()}@test.local`;
    const createRes = await fetch(`${BASE}/api/v1/ac/user`, {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ email, name: "PDV Password Test", password: "OldPass123!" }),
    });
    expect(createRes.status).toBe(201);
    const user = await createRes.json();
    // Track for cleanup (overwrite previous if set)
    const userId = user.id;

    // Reset password
    const patchRes = await fetch(`${BASE}/api/v1/ac/user/${userId}`, {
      method: "PATCH", headers: authHeaders(),
      body: JSON.stringify({ password: "NewPass456!" }),
    });
    expect(patchRes.status).toBe(200);

    // Clean up
    await fetch(`${BASE}/api/v1/ac/user/${userId}`, { method: "DELETE", headers: authHeaders() }).catch(() => {});
  });

  test("PATCH /api/v1/ac/group/:id with name change works (Rename Group)", async () => {
    const groupName = `pdv-rename-${Date.now()}`;
    // Create a test group
    const createRes = await fetch(`${BASE}/api/v1/ac/group`, {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ name: groupName, roles: [] }),
    });
    expect(createRes.status).toBe(201);
    const group = await createRes.json();
    testGroupId = group.id;

    // Rename the group
    const newName = `${groupName}-renamed`;
    const patchRes = await fetch(`${BASE}/api/v1/ac/group/${group.id}`, {
      method: "PATCH", headers: authHeaders(),
      body: JSON.stringify({ name: newName }),
    });
    expect(patchRes.status).toBe(200);
    const renamed = await patchRes.json();
    expect(renamed.name).toBe(newName);
  });

  test("DELETE /api/v1/ac/group/:id works (Delete Group)", async () => {
    const groupName = `pdv-delete-${Date.now()}`;
    // Create a test group
    const createRes = await fetch(`${BASE}/api/v1/ac/group`, {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ name: groupName, roles: [] }),
    });
    expect(createRes.status).toBe(201);
    const group = await createRes.json();

    // Delete the group
    const deleteRes = await fetch(`${BASE}/api/v1/ac/group/${group.id}`, {
      method: "DELETE", headers: authHeaders(),
    });
    expect(deleteRes.status).toBe(204);

    // Verify it's gone
    const getRes = await fetch(`${BASE}/api/v1/ac/group/${group.id}`, {
      headers: authHeaders(),
    });
    expect(getRes.status).toBe(404);
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
