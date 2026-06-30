/**
 * Project Board Applet — Post-Deployment Verification Tests
 *
 * Validates that the Project Board loads, shows kanban columns,
 * and supports card CRUD operations.
 */

import { test, expect, Page } from "@playwright/test";

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

function authHeaders(): Record<string, string> {
  return { "Content-Type": "application/json", Authorization: "Bearer mock-token" };
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

async function openProjectBoard(page: Page): Promise<void> {
  await page.locator('[data-dock-item-id="pb"]').click();
  await expect(
    page.locator('[role="dialog"][aria-label="Convocate Project Board"]')
  ).toBeVisible({ timeout: 5000 });
  await expect(
    page.locator('[data-testid="project-board"]')
  ).toBeVisible({ timeout: 10000 });
}

// ---------------------------------------------------------------------------
// Tests: Board loads and shows columns
// ---------------------------------------------------------------------------

test.describe("Project Board applet", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await openProjectBoard(page);
  });

  test("displays kanban columns for all statuses", async ({ page }) => {
    for (const status of ["TODO", "ACTIVE", "DONE", "FAIL", "NOTE"]) {
      await expect(
        page.locator(`[data-testid="project-board"] >> text=${status}`)
      ).toBeVisible({ timeout: 5000 });
    }
  });

  test("shows board selector and action buttons", async ({ page }) => {
    await expect(page.locator('[data-testid="project-board"] select')).toBeVisible();
    await expect(page.locator('[data-testid="project-board"] button:has-text("New Card")')).toBeVisible();
    await expect(page.locator('[data-testid="project-board"] button:has-text("New Board")')).toBeVisible();
    await expect(page.locator('[data-testid="project-board"] button:has-text("Refresh")')).toBeVisible();
  });

  test("shows card and edge count in footer", async ({ page }) => {
    await expect(page.locator('text=/\\d+ cards?/')).toBeVisible({ timeout: 5000 });
  });

  test("has Status and Canvas view toggle buttons", async ({ page }) => {
    await expect(page.locator('[data-testid="project-board"] button:has-text("Status")')).toBeVisible();
    await expect(page.locator('[data-testid="project-board"] button:has-text("Canvas")')).toBeVisible();
  });

  test("can switch to Canvas view", async ({ page }) => {
    // Click Canvas toggle
    await page.locator('[data-testid="project-board"] button:has-text("Canvas")').click();

    // Canvas view should show cards positioned on a dark background
    // and the footer should still show card count
    await expect(page.locator('text=/\\d+ cards?/')).toBeVisible({ timeout: 5000 });
  });

  test("can switch back to Status view", async ({ page }) => {
    // Switch to Canvas
    await page.locator('[data-testid="project-board"] button:has-text("Canvas")').click();
    await page.waitForTimeout(500);

    // Switch back to Status
    await page.locator('[data-testid="project-board"] button:has-text("Status")').click();

    // Should show kanban columns again
    await expect(page.locator('[data-testid="project-board"] >> text=TODO')).toBeVisible({ timeout: 3000 });
  });
});

// ---------------------------------------------------------------------------
// Tests: Card operations
// ---------------------------------------------------------------------------

test.describe("Project Board card operations", () => {
  test("New Card dialog opens and validates", async ({ page }) => {
    await login(page);
    await openProjectBoard(page);

    await page.locator('[data-testid="project-board"] button:has-text("New Card")').click();
    await expect(page.locator('text=New Card').first()).toBeVisible({ timeout: 3000 });

    // Submit empty
    await page.locator('button:has-text("Add Card")').click();
    await expect(page.locator('text=Card title is required')).toBeVisible();

    // Cancel
    await page.locator('button:has-text("Cancel")').click();
    await expect(page.locator('input[placeholder="Card title"]')).not.toBeVisible();
  });

  test("cards are rendered in the kanban columns", async ({ page }) => {
    await login(page);
    await openProjectBoard(page);

    // Wait for cards to load — the seed data has card-001 and card-002
    await page.waitForTimeout(2000);

    // Verify card IDs are visible in the board
    const cardText = await page.locator('[data-testid="project-board"]').textContent();
    expect(cardText).toContain("card-001");
    expect(cardText).toContain("card-002");
  });

  test("New Board dialog opens and validates", async ({ page }) => {
    await login(page);
    await openProjectBoard(page);

    await page.locator('[data-testid="project-board"] button:has-text("New Board")').click();
    await expect(page.locator('text=New Board').first()).toBeVisible({ timeout: 3000 });

    // Submit empty
    await page.locator('button:has-text("Create")').last().click();
    await expect(page.locator('text=Board name is required')).toBeVisible();

    // Cancel
    await page.locator('button:has-text("Cancel")').click();
  });
});

// ---------------------------------------------------------------------------
// Tests: API operations
// ---------------------------------------------------------------------------

test.describe("Project Board API", () => {
  test("list boards returns paginated results", async () => {
    const res = await fetch(`${BASE}/api/v1/pb/board?limit=10`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    const page = await res.json();
    expect(page).toHaveProperty("items");
    expect(page).toHaveProperty("total");
  });

  test("get board returns cards and edges", async () => {
    const listRes = await fetch(`${BASE}/api/v1/pb/board?limit=1`, { headers: authHeaders() });
    const boards = await listRes.json();
    if (boards.items && boards.items.length > 0) {
      const res = await fetch(`${BASE}/api/v1/pb/board/${boards.items[0].id}`, { headers: authHeaders() });
      expect(res.status).toBe(200);
      const board = await res.json();
      expect(board).toHaveProperty("cards");
      expect(board).toHaveProperty("edges");
      expect(board).toHaveProperty("containers");
    }
  });

  test("can create and delete a card", async () => {
    const listRes = await fetch(`${BASE}/api/v1/pb/board?limit=1`, { headers: authHeaders() });
    const boards = await listRes.json();
    if (boards.items && boards.items.length > 0) {
      const boardId = boards.items[0].id;

      // Create
      const createRes = await fetch(`${BASE}/api/v1/pb/board/${boardId}/card`, {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ title: `pdv-test-${Date.now()}`, status: "todo" }),
      });
      expect(createRes.status).toBe(201);
      const card = await createRes.json();
      expect(card).toHaveProperty("id");

      // Delete
      const delRes = await fetch(`${BASE}/api/v1/pb/board/${boardId}/card/${card.id}`, {
        method: "DELETE",
        headers: authHeaders(),
      });
      expect(delRes.status).toBe(204);
    }
  });
});
