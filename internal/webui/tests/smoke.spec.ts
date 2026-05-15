import { test, expect } from "@playwright/test";

test.describe("Web UI Smoke Tests", () => {
  test("dashboard loads", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("text=convocate")).toBeVisible();
    await expect(page.locator("text=Dashboard")).toBeVisible();
  });

  test("navigation works", async ({ page }) => {
    await page.goto("/");

    await page.click("text=Create Project");
    await expect(page.locator("h1:has-text('Create Project')")).toBeVisible();

    await page.click("text=Cluster Auth");
    await expect(page.locator("h1:has-text('Cluster Authentication')")).toBeVisible();

    await page.click("text=Ad-hoc Submit");
    await expect(page.locator("h1:has-text('Ad-hoc Job Submission')")).toBeVisible();

    await page.click("text=Dashboard");
    await expect(page.locator("h1:has-text('Dashboard')")).toBeVisible();
  });

  test("create project form has required fields", async ({ page }) => {
    await page.goto("/");
    await page.click("text=Create Project");

    await expect(page.locator("text=Repository")).toBeVisible();
    await expect(page.locator("text=SSH Private Key")).toBeVisible();
    await expect(page.locator("text=GitHub PAT")).toBeVisible();
    await expect(page.locator("button:has-text('Create Project')")).toBeVisible();
  });

  test("cluster auth form has mode selector", async ({ page }) => {
    await page.goto("/");
    await page.click("text=Cluster Auth");

    await expect(page.locator("text=Authentication Mode")).toBeVisible();
    await expect(page.locator("select")).toBeVisible();
    await expect(page.locator("button:has-text('Update Authentication')")).toBeVisible();
  });

  test("ad-hoc submit form has project selector and prompt", async ({ page }) => {
    await page.goto("/");
    await page.click("text=Ad-hoc Submit");

    await expect(page.locator("text=Project")).toBeVisible();
    await expect(page.locator("text=Prompt")).toBeVisible();
    await expect(page.locator("button:has-text('Submit')")).toBeVisible();
  });
});
