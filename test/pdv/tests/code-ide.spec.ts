/**
 * Code IDE Applet — Post-Deployment Verification Tests
 *
 * Validates that the Code IDE applet loads, shows projects, file explorer,
 * editor tabs, and supports file CRUD operations.
 */

import { test, expect, Page } from "@playwright/test";

// Disable TLS for direct API calls
process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

function authHeaders(): Record<string, string> {
  return {
    "Content-Type": "application/json",
    Authorization: "Bearer mock-token",
  };
}

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
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({ timeout: 15000 });
}

async function openCodeIDE(page: Page): Promise<void> {
  await page.locator('[data-dock-item-id="ide"]').click();
  await expect(
    page.locator('[role="dialog"][aria-label="Code IDE"]')
  ).toBeVisible({ timeout: 5000 });
  await expect(
    page.locator('[data-testid="code-ide"]')
  ).toBeVisible({ timeout: 10000 });
}

// ---------------------------------------------------------------------------
// Tests: IDE loads and shows basic UI
// ---------------------------------------------------------------------------

test.describe("Code IDE applet", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await openCodeIDE(page);
  });

  test("displays the menu bar with project selector", async ({ page }) => {
    // Project selector (dropdown) should be visible
    await expect(
      page.locator('[data-testid="code-ide"] select')
    ).toBeVisible({ timeout: 5000 });
  });

  test("shows the file explorer sidebar", async ({ page }) => {
    // Explorer heading
    await expect(
      page.locator('[data-testid="code-ide"] >> text=Explorer')
    ).toBeVisible({ timeout: 5000 });
  });

  test("shows the status bar", async ({ page }) => {
    // Status bar with project name
    const statusBar = page.locator('[data-testid="code-ide"]').locator('div').filter({
      has: page.locator('span'),
    }).last();
    await expect(statusBar).toBeVisible();
  });

  test("has New Project and New File buttons", async ({ page }) => {
    await expect(
      page.locator('[data-testid="code-ide"] button:has-text("New Project")')
    ).toBeVisible();
    await expect(
      page.locator('[data-testid="code-ide"] button:has-text("New File")')
    ).toBeVisible();
  });

  test("shows empty editor message when no file is open", async ({ page }) => {
    await expect(
      page.locator('text=Select a file to edit')
    ).toBeVisible({ timeout: 5000 });
  });
});

// ---------------------------------------------------------------------------
// Tests: File operations
// ---------------------------------------------------------------------------

test.describe("Code IDE file operations", () => {
  test("clicking a file in explorer opens it in a tab", async ({ page }) => {
    await login(page);
    await openCodeIDE(page);

    // Wait for file tree to load — click the first file
    const fileItem = page.locator('[data-testid="code-ide"] >> text=SPECIFICATION.md');
    if (await fileItem.isVisible({ timeout: 5000 })) {
      await fileItem.click();

      // A tab should appear
      await expect(
        page.locator('[data-testid="code-ide"]').locator('text=SPECIFICATION.md').first()
      ).toBeVisible({ timeout: 3000 });

      // The empty editor message should be gone
      await expect(page.locator('text=Select a file to edit')).not.toBeVisible();
    }
  });

  test("New File dialog opens and validates", async ({ page }) => {
    await login(page);
    await openCodeIDE(page);

    await page.locator('[data-testid="code-ide"] button:has-text("New File")').click();
    await expect(page.locator('text=New File').first()).toBeVisible({ timeout: 3000 });
    await expect(page.locator('input[placeholder*="File path"]')).toBeVisible();

    // Submit empty — should show error
    await page.locator('button:has-text("Create")').last().click();
    await expect(page.locator('text=File path is required')).toBeVisible();

    // Cancel
    await page.locator('button:has-text("Cancel")').click();
    await expect(page.locator('input[placeholder*="File path"]')).not.toBeVisible();
  });

  test("New Project dialog opens and validates", async ({ page }) => {
    await login(page);
    await openCodeIDE(page);

    await page.locator('[data-testid="code-ide"] button:has-text("New Project")').click();
    await expect(page.locator('text=New Project').first()).toBeVisible({ timeout: 3000 });

    // Submit empty
    await page.locator('button:has-text("Create")').last().click();
    await expect(page.locator('text=Project name is required')).toBeVisible();

    // Cancel
    await page.locator('button:has-text("Cancel")').click();
  });
});

// ---------------------------------------------------------------------------
// Tests: API operations
// ---------------------------------------------------------------------------

test.describe("Code IDE API", () => {
  test("list projects returns paginated results", async () => {
    const res = await fetch(`${BASE}/api/v1/ide/project?limit=10`, {
      headers: authHeaders(),
    });
    expect(res.status).toBe(200);
    const page = await res.json();
    expect(page).toHaveProperty("items");
    expect(page).toHaveProperty("total");
  });

  test("file tree returns files for a project", async () => {
    // Get first project
    const projRes = await fetch(`${BASE}/api/v1/ide/project?limit=1`, {
      headers: authHeaders(),
    });
    const projects = await projRes.json();
    if (projects.items && projects.items.length > 0) {
      const projectId = projects.items[0].id;
      const treeRes = await fetch(`${BASE}/api/v1/ide/project/${projectId}/tree`, {
        headers: authHeaders(),
      });
      expect(treeRes.status).toBe(200);
      const tree = await treeRes.json();
      expect(Array.isArray(tree)).toBeTruthy();
    }
  });

  test("can create and read a file", async () => {
    const projRes = await fetch(`${BASE}/api/v1/ide/project?limit=1`, {
      headers: authHeaders(),
    });
    const projects = await projRes.json();
    if (projects.items && projects.items.length > 0) {
      const projectId = projects.items[0].id;
      const testPath = `test-${Date.now()}.txt`;

      // Create
      const createRes = await fetch(
        `${BASE}/api/v1/ide/project/${projectId}/file/${encodeURIComponent(testPath)}`,
        {
          method: "PUT",
          headers: authHeaders(),
          body: JSON.stringify({ content: "hello world" }),
        }
      );
      expect(createRes.status).toBe(200);

      // Read
      const readRes = await fetch(
        `${BASE}/api/v1/ide/project/${projectId}/file/${encodeURIComponent(testPath)}`,
        { headers: authHeaders() }
      );
      expect(readRes.status).toBe(200);
      const file = await readRes.json();
      expect(file.content).toBe("hello world");

      // Cleanup
      await fetch(
        `${BASE}/api/v1/ide/project/${projectId}/file/${encodeURIComponent(testPath)}`,
        { method: "DELETE", headers: authHeaders() }
      );
    }
  });
});
