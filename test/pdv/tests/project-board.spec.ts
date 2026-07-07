/**
 * Project Board Applet — Post-Deployment Verification Tests
 *
 * Validates that the Project Board loads, shows flat kanban columns,
 * and supports card CRUD operations. Containers have been removed —
 * each project has one agent-container managed by Agent Manager.
 */

import { test, expect, Page } from "@playwright/test";


const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

function authHeaders() {
  return { "Content-Type": "application/json", Authorization: "Bearer mock-token" };
}

async function login(page: Page): Promise<void> {
  await page.goto("/");
  await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });
  await page.locator('input[placeholder="Username"]').fill("admin");
  await page.locator('input[placeholder="Password"]').fill("test");
  await page.locator('input[placeholder="MFA Token"]').fill("123456");
  await page.locator('button:has-text("Sign In")').click();
  await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({ timeout: 15000 });
}

async function openProjectBoard(page: Page): Promise<void> {
  await page.locator('[data-dock-item-id="pb"]').click();
  await expect(page.locator('[role="dialog"][aria-label="Convocate Project Board"]')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('[data-testid="project-board"]')).toBeVisible({ timeout: 10000 });
}

test.describe("Project Board applet", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await openProjectBoard(page);
  });

  test("displays flat kanban columns for all statuses", async ({ page }) => {
    for (const status of ["TODO", "ACTIVE", "DONE", "FAIL", "NOTE"]) {
      await expect(
        page.locator(`[data-testid="project-board"] >> text=${status}`)
      ).toBeVisible({ timeout: 5000 });
    }
  });

  test("shows project selector and action buttons", async ({ page }) => {
    await expect(page.locator('[data-testid="project-board"] select')).toBeVisible();
    await expect(page.locator('[data-testid="project-board"] button:has-text("New Card")')).toBeVisible();
    await expect(page.locator('[data-testid="project-board"] button:has-text("New Project")')).toBeVisible();
  });

  test("has Status and Canvas view toggle", async ({ page }) => {
    await expect(page.locator('[data-testid="project-board"] button:has-text("Status")')).toBeVisible();
    await expect(page.locator('[data-testid="project-board"] button:has-text("Canvas")')).toBeVisible();
  });

  test("Status/Canvas toggle is leftmost toolbar item", async ({ page }) => {
    const statusBtn = page.locator('[data-testid="project-board"] button:has-text("Status")');
    const selectEl = page.locator('[data-testid="project-board"] select');
    await expect(statusBtn).toBeVisible({ timeout: 5000 });
    await expect(selectEl).toBeVisible();
    const statusBox = await statusBtn.boundingBox();
    const selectBox = await selectEl.boundingBox();
    expect(statusBox).toBeTruthy();
    expect(selectBox).toBeTruthy();
    // Status button should be to the LEFT of the project selector
    expect(statusBox!.x).toBeLessThan(selectBox!.x);
  });

  test("shows card count in footer", async ({ page }) => {
    await expect(page.locator('text=/\\d+ cards?/')).toBeVisible({ timeout: 5000 });
  });

  test("board content area is visible", async ({ page }) => {
    await page.waitForTimeout(2000);
    // The board should be loaded (either with cards or empty)
    await expect(page.locator('[data-testid="project-board"]')).toBeVisible();
  });
});

test.describe("Project Board card operations", () => {
  test("New Card dialog opens and validates", async ({ page }) => {
    await login(page);
    await openProjectBoard(page);
    await page.locator('[data-testid="project-board"] button:has-text("New Card")').click();
    await expect(page.locator('text=New Card').first()).toBeVisible({ timeout: 3000 });
    await page.locator('button:has-text("Add Card")').click();
    await expect(page.locator('text=Card title is required')).toBeVisible();
    await page.locator('button:has-text("Cancel")').click();
  });

  test("New Project dialog opens and validates", async ({ page }) => {
    await login(page);
    await openProjectBoard(page);
    await page.locator('[data-testid="project-board"] button:has-text("New Project")').click();
    await expect(page.locator('text=New Project').first()).toBeVisible({ timeout: 3000 });
    await page.locator('button:has-text("Create")').last().click();
    await expect(page.locator('text=/name is required/i')).toBeVisible();
    await page.locator('button:has-text("Cancel")').click();
  });
});

test.describe("Project Board API", () => {
  test("unified projects API returns results", async () => {
    const res = await fetch(`${BASE}/api/v1/projects?limit=10`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    const page = await res.json();
    expect(page).toHaveProperty("items");
  });

  test("get board returns cards and edges (no containers)", async () => {
    const listRes = await fetch(`${BASE}/api/v1/pb/board?limit=1`, { headers: authHeaders() });
    const boards = await listRes.json();
    if (boards.items && boards.items.length > 0) {
      const res = await fetch(`${BASE}/api/v1/pb/board/${boards.items[0].id}`, { headers: authHeaders() });
      expect(res.status).toBe(200);
      const board = await res.json();
      expect(board).toHaveProperty("cards");
      expect(board).toHaveProperty("edges");
      // Containers should NOT be present
      expect(board.containers).toBeUndefined();
    }
  });

  test("can create and delete a card", async () => {
    const listRes = await fetch(`${BASE}/api/v1/pb/board?limit=1`, { headers: authHeaders() });
    const boards = await listRes.json();
    if (boards.items && boards.items.length > 0) {
      const boardId = boards.items[0].id;
      const createRes = await fetch(`${BASE}/api/v1/pb/board/${boardId}/card`, {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ title: `pdv-test-${Date.now()}`, status: "todo" }),
      });
      expect(createRes.status).toBe(201);
      const card = await createRes.json();
      const delRes = await fetch(`${BASE}/api/v1/pb/board/${boardId}/card/${card.id}`, {
        method: "DELETE", headers: authHeaders(),
      });
      expect(delRes.status).toBe(204);
    }
  });
});
