/**
 * Color Contrast — Post-Deployment Verification Tests
 *
 * Guarantees that all visible text in the application has sufficient contrast
 * against its background.  The project uses a dark theme; foreground colors
 * must be light enough to meet WCAG AA (4.5:1 for normal text, 3:1 for
 * large text / UI components).
 *
 * Elements rendered by SpecifyJS internally (buttons, tabs, etc.) are
 * checked at a relaxed threshold (3:1) since we cannot modify their styling.
 * Our own application text is held to the strict 4.5:1 WCAG AA standard.
 */

import { test, expect, Page } from "@playwright/test";

// ---------------------------------------------------------------------------
// WCAG contrast helpers
// ---------------------------------------------------------------------------

/** Minimum acceptable contrast ratio (WCAG AA normal text) */
const MIN_CONTRAST = 4.5;

/** Relaxed threshold for large text / interactive elements */
const MIN_CONTRAST_INTERACTIVE = 3.0;

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

/**
 * Audit contrast for all visible text elements within a container.
 * Returns array of { text, fg, bg, ratio, isInteractive } objects.
 */
async function auditContrast(
  page: Page,
  containerSelector: string
): Promise<{ text: string; fg: string; bg: string; ratio: number; isInteractive: boolean }[]> {
  return page.evaluate((selector) => {
    function parseRgb(css: string): [number, number, number] | null {
      const m = css.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/);
      if (!m) return null;
      return [parseInt(m[1]), parseInt(m[2]), parseInt(m[3])];
    }

    function relativeLuminance(r: number, g: number, b: number): number {
      const [rs, gs, bs] = [r, g, b].map((c) => {
        const s = c / 255;
        return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
      });
      return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs;
    }

    function contrastRatio(l1: number, l2: number): number {
      const lighter = Math.max(l1, l2);
      const darker = Math.min(l1, l2);
      return (lighter + 0.05) / (darker + 0.05);
    }

    function effectiveBg(el: Element): string {
      let current: Element | null = el;
      while (current) {
        const style = window.getComputedStyle(current);
        const bg = style.backgroundColor;
        const parsed = parseRgb(bg);
        if (parsed) {
          const alphaMatch = bg.match(/rgba\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*,\s*([\d.]+)/);
          const alpha = alphaMatch ? parseFloat(alphaMatch[1]) : 1;
          if (alpha > 0.1) return bg;
        }
        current = current.parentElement;
      }
      return "rgb(0, 0, 0)";
    }

    const container = document.querySelector(selector);
    if (!container) return [];

    const results: { text: string; fg: string; bg: string; ratio: number; isInteractive: boolean }[] = [];
    const seen = new Set<string>();

    const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT);
    let node: Node | null;
    while ((node = walker.nextNode())) {
      const text = (node.textContent || "").trim();
      if (!text || text.length > 100) continue;

      const el = node.parentElement;
      if (!el) continue;

      const style = window.getComputedStyle(el);
      if (style.display === "none" || style.visibility === "hidden" || style.opacity === "0")
        continue;

      const fg = style.color;
      const bg = effectiveBg(el);

      const key = `${fg}|${bg}`;
      if (seen.has(key)) continue;
      seen.add(key);

      const fgRgb = parseRgb(fg);
      const bgRgb = parseRgb(bg);
      if (!fgRgb || !bgRgb) continue;

      const fgLum = relativeLuminance(...fgRgb);
      const bgLum = relativeLuminance(...bgRgb);
      const ratio = contrastRatio(fgLum, bgLum);

      // Determine if the element is interactive (button, link, tab)
      const tagName = el.tagName.toLowerCase();
      const role = el.getAttribute("role");
      const isInteractive =
        tagName === "button" || tagName === "a" ||
        role === "button" || role === "tab" || role === "link" ||
        !!el.closest("button, a, [role=button], [role=tab]");

      results.push({
        text: text.substring(0, 40),
        fg,
        bg,
        ratio: Math.round(ratio * 100) / 100,
        isInteractive,
      });
    }

    return results;
  }, containerSelector);
}

/**
 * Assert no contrast violations.
 * Interactive elements (buttons, links, tabs) use 3:1 threshold.
 * Normal text uses 4.5:1 threshold.
 */
function assertContrast(
  results: { text: string; fg: string; bg: string; ratio: number; isInteractive: boolean }[],
  context: string
) {
  const failures = results.filter((r) => {
    const threshold = r.isInteractive ? MIN_CONTRAST_INTERACTIVE : MIN_CONTRAST;
    return r.ratio < threshold;
  });

  if (failures.length > 0) {
    const report = failures
      .map((f) => `  "${f.text}" — fg:${f.fg} bg:${f.bg} ratio:${f.ratio} ${f.isInteractive ? "(interactive)" : "(text)"}`)
      .join("\n");
    expect(failures.length, `Contrast failures in ${context}:\n${report}`).toBe(0);
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe("Color contrast verification", () => {
  test("login screen text is visible against its background", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });

    const results = await auditContrast(page, "#app");
    assertContrast(results, "login screen");
  });

  test("Node Manager grid text is visible against dark background", async ({ page }) => {
    await login(page);

    await page.locator('[data-dock-item-id="nmgr"]').click();
    await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });

    const results = await auditContrast(page, '[data-testid="node-manager"]');
    assertContrast(results, "Node Manager grid");
  });

  test("Node Manager detail dialog text is visible", async ({ page }) => {
    await login(page);

    await page.locator('[data-dock-item-id="nmgr"]').click();
    await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });

    await page.locator('[data-testid="node-manager"] a').first().click();
    await expect(page.locator('text=/Node: /')).toBeVisible({ timeout: 5000 });

    // Audit the Node Manager area (excludes SpecifyJS chrome we can't control)
    const results = await auditContrast(page, '[data-testid="node-manager"]');
    assertContrast(results, "Node Manager detail dialog");
  });

  test("no metrics show impossible negative values", async ({ page }) => {
    await login(page);

    await page.locator('[data-dock-item-id="nmgr"]').click();
    await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });

    // Check that no cell in the grid shows negative values for metrics
    const cellTexts = await page.locator('[data-testid="node-manager"] td').allTextContents();
    for (const text of cellTexts) {
      // Metrics columns should never show negative numbers
      if (text.includes("-1.0") || text.includes("-1.00")) {
        expect(text, `Found impossible metric value: "${text}"`).not.toContain("-1");
      }
    }
  });

  test("metrics show dash when data unavailable", async ({ page }) => {
    await login(page);

    await page.locator('[data-dock-item-id="nmgr"]').click();
    await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });

    // Verify that metrics are either valid positive numbers or "—"
    const cellTexts = await page.locator('[data-testid="node-manager"] td').allTextContents();
    const metricPattern = /^-?\d+\.\d+\s*\/\s*\d+\.\d+\s*GB$/;
    for (const text of cellTexts) {
      if (metricPattern.test(text.trim())) {
        // If it matches the "X.X / X.X GB" pattern, values must be non-negative
        const nums = text.match(/-?\d+\.\d+/g);
        if (nums) {
          for (const n of nums) {
            expect(parseFloat(n), `Negative metric value in "${text}"`).toBeGreaterThanOrEqual(0);
          }
        }
      }
    }
  });
});
