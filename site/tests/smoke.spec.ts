import { test, expect } from '@playwright/test';

// Post-deployment smoke tests for the convocate documentation site.
// These run against the live GitHub Pages deployment to verify the
// site renders correctly after each deploy.

test.describe('site smoke tests', () => {
  test('home page loads and renders the app shell', async ({ page }) => {
    await page.goto('/');

    // The #root element should be populated (specifyjs rendered).
    const root = page.locator('#root');
    await expect(root).not.toBeEmpty();

    // Header brand text should be visible.
    await expect(page.locator('.brand-text')).toHaveText('Convocate');
  });

  test('no console errors on load', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') errors.push(msg.text());
    });

    await page.goto('/');
    // Wait for the app to fully render.
    await expect(page.locator('.brand-text')).toBeVisible();

    expect(errors).toEqual([]);
  });

  test('sidebar navigation renders sections', async ({ page }) => {
    await page.goto('/');

    const sidebar = page.locator('.sidebar');
    await expect(sidebar).toBeVisible();

    // At least one section heading should exist.
    const headings = sidebar.locator('.sidebar-heading');
    await expect(headings.first()).toBeVisible();
    expect(await headings.count()).toBeGreaterThan(0);
  });

  test('navigating to getting-started renders content', async ({ page }) => {
    await page.goto('/');

    // Click the Getting started link in the top nav.
    await page.locator('.top-nav-link', { hasText: 'Getting started' }).click();

    // The markdown body should render.
    const content = page.locator('.markdown-body');
    await expect(content).toBeVisible();
    await expect(content).not.toBeEmpty();
  });

  test('theme toggle switches data-theme attribute', async ({ page }) => {
    await page.goto('/');

    const html = page.locator('html');
    const toggle = page.locator('.theme-toggle');

    // Get initial theme.
    const initial = await html.getAttribute('data-theme');

    // Toggle.
    await toggle.click();
    const after = await html.getAttribute('data-theme');
    expect(after).not.toEqual(initial);

    // Toggle back.
    await toggle.click();
    const restored = await html.getAttribute('data-theme');
    expect(restored).toEqual(initial);
  });

  test('sidebar link navigation updates content', async ({ page }) => {
    await page.goto('/');

    // Click the first sidebar link that is not already active.
    const inactiveLink = page.locator('.sidebar-link:not(.sidebar-link-active)').first();
    const linkText = await inactiveLink.textContent();
    await inactiveLink.click();

    // Content area should update.
    const content = page.locator('.markdown-body');
    await expect(content).toBeVisible();
    await expect(content).not.toBeEmpty();

    // The clicked link should now be active.
    await expect(
      page.locator('.sidebar-link-active', { hasText: linkText ?? '' }),
    ).toBeVisible();
  });

  test('404 page renders for unknown routes', async ({ page }) => {
    await page.goto('/#/this-route-does-not-exist-42');

    await expect(page.locator('.markdown-body')).toContainText('Page not found');
  });
});
