import { test, expect } from "@playwright/test";

const APP = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

test.describe("UI Post-Deployment Verification", () => {
  test("login page renders with form fields", async ({ page }) => {
    await page.goto(APP);
    // Wait for SpecifyJS to render the login form
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = await page.locator("input").count();
    expect(inputs).toBeGreaterThanOrEqual(3); // username, password, MFA
  });

  test("login with valid credentials shows desktop", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });

    // Fill login form
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");

    // Click sign in button
    await page.locator("button").filter({ hasText: /sign in/i }).click();

    // Wait for desktop to render (dock icons should appear)
    await page.waitForSelector("img[alt='Node Manager']", { timeout: 10000 });
  });

  test("dock shows applet icons after login", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });

    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();

    await page.waitForSelector("img[alt='Node Manager']", { timeout: 10000 });

    // Should have dock items (applet icons)
    const dockItems = page.locator("img[alt='Node Manager'], img[alt='Agent Manager'], img[alt='Code IDE'], img[alt='Access Control'], img[alt='Repo Manager'], img[alt='Support Tool'], img[alt='Convocate Project Board']");
    await expect(dockItems.first()).toBeVisible({ timeout: 5000 });
  });

  test("dock click opens Node Manager with K8s data", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    // Click dock icon via mouse coordinates
    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width/2, box.y + box.height/2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("Node Manager");
    expect(text).toContain("nodes");
  });

  test("all 7 dock icons present after login", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icons = await page.locator("img[alt='Node Manager'], img[alt='Agent Manager'], img[alt='Code IDE'], img[alt='Access Control'], img[alt='Repo Manager'], img[alt='Support Tool'], img[alt='Convocate Project Board']").count();
    expect(icons).toBeGreaterThanOrEqual(7);
  });

  test("healthz endpoint returns ok", async ({ request }) => {
    const res = await request.get(`${APP}/healthz`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.status).toBe("ok");
  });

  test("SPA fallback serves app for unknown paths", async ({ page }) => {
    await page.goto(`${APP}/some/unknown/path`);
    // Should still load the app (SPA fallback)
    await expect(page.locator("#app")).toBeAttached();
  });

  // -- Applet-specific tests --------------------------------------------------

  test("Node Manager shows DataGrid with node data", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Should show node count and Provision button
    const text = await page.textContent("body");
    expect(text).toContain("nodes");
    expect(text).toContain("Provision Node");
  });

  test("Node Manager DataGrid rows have readable text on dark background", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Get all table body cells and check text color has sufficient lightness
    const cells = page.locator("table tbody td");
    const cellCount = await cells.count();
    expect(cellCount).toBeGreaterThan(0);

    // Check at least 3 cells across different rows for light text color
    for (let i = 0; i < Math.min(cellCount, 6); i++) {
      const color = await cells.nth(i).evaluate((el) => {
        return window.getComputedStyle(el).color;
      });
      // Parse rgb(r, g, b) and verify lightness > 150 (readable on dark bg)
      const match = color.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
      expect(match).not.toBeNull();
      if (match) {
        const [, r, g, b] = match.map(Number);
        const lightness = (r + g + b) / 3;
        expect(lightness).toBeGreaterThan(150);
      }
    }
  });

  test("Node Manager Provision button opens dialog", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Click Provision Node button
    await page.locator("button").filter({ hasText: /Provision Node/i }).click();
    await page.waitForTimeout(500);

    // Should show dialog with form fields
    const text = await page.textContent("body");
    expect(text).toContain("Host");
    expect(text).toContain("SSH User");
  });

  test("Node Manager shows action buttons per node", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Should have Cordon/Delete action buttons
    const text = await page.textContent("body");
    expect(text?.includes("Cordon") || text?.includes("Uncordon")).toBeTruthy();
    expect(text).toContain("Delete");
  });

  test("Agent Manager shows Create Agent button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Agent Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("agent-containers");
    // Menubar should have Create, Configure, Start, Stop, Delete buttons
    expect(text).toContain("Create");
    expect(text).toContain("Configure");
    expect(text).toContain("Start");
    expect(text).toContain("Stop");
    expect(text).toContain("Delete");
  });

  test("Access Control shows tabs and Create User button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Access Control']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("Users");
    expect(text).toContain("Groups");
    expect(text).toContain("Global Settings");
    expect(text).toContain("Create User");
  });

  test("Repo Manager shows repos and Create Repo button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Repo Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("repositories");
    expect(text).toContain("Create Repo");
  });

  test("Support Tool shows tickets tab and New Ticket button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Support Tool']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("Tickets");
    expect(text).toContain("Documentation");
    expect(text).toContain("New Ticket");
  });

  test("Project Board shows boards and New Board button", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Convocate Project Board']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    const text = await page.textContent("body");
    expect(text).toContain("boards");
    expect(text).toContain("New Board");
  });

  // -- Node Manager: column correctness, provisioning, and metrics ------------

  test("Node Manager table has correct column headers", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Collect all column headers from the node table
    const headers = page.locator("table thead th");
    const headerCount = await headers.count();
    const headerTexts: string[] = [];
    for (let i = 0; i < headerCount; i++) {
      headerTexts.push((await headers.nth(i).textContent() || "").trim());
    }

    // Required columns — must appear in exact form
    expect(headerTexts).toContain("Name");
    expect(headerTexts).toContain("IP");
    expect(headerTexts).toContain("Status");
    expect(headerTexts).toContain("Load Avg");
    expect(headerTexts).toContain("Memory");
    expect(headerTexts).toContain("Disk");
    expect(headerTexts).toContain("Actions");
  });

  test("Node Manager node rows display valid metric values or 'no data'", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    // Wait for initial load + one SSE metric update cycle (3s)
    await page.waitForTimeout(5000);

    const rows = page.locator("table tbody tr");
    const rowCount = await rows.count();
    expect(rowCount).toBeGreaterThan(0);

    for (let r = 0; r < rowCount; r++) {
      const cells = rows.nth(r).locator("td");
      const cellCount = await cells.count();

      for (let c = 0; c < cellCount; c++) {
        const text = (await cells.nth(c).textContent() || "").trim();

        // No cell should ever display raw -1 values from missing metrics
        expect(text).not.toMatch(/-1\.?\d*/);

        // No cell should display "undefined" or "NaN"
        expect(text).not.toContain("undefined");
        expect(text).not.toContain("NaN");
      }
    }
  });

  test("Node Manager load average column shows valid format", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(5000);

    // Find the Load Avg column index from headers
    const headers = page.locator("table thead th");
    const headerCount = await headers.count();
    let loadCol = -1;
    for (let i = 0; i < headerCount; i++) {
      const h = (await headers.nth(i).textContent() || "").trim();
      if (h === "Load Avg") { loadCol = i; break; }
    }
    expect(loadCol).toBeGreaterThan(-1);

    // Each row's load avg cell must be either "no data" or "X.XX / X.XX / X.XX"
    const rows = page.locator("table tbody tr");
    const rowCount = await rows.count();
    for (let r = 0; r < rowCount; r++) {
      const cell = rows.nth(r).locator("td").nth(loadCol);
      const text = (await cell.textContent() || "").trim();
      const valid = text === "no data" || /^\d+\.\d+ \/ \d+\.\d+ \/ \d+\.\d+$/.test(text);
      expect(valid).toBe(true);
    }
  });

  test("Node Manager memory column shows valid format", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(5000);

    // Find the Memory column index
    const headers = page.locator("table thead th");
    const headerCount = await headers.count();
    let memCol = -1;
    for (let i = 0; i < headerCount; i++) {
      const h = (await headers.nth(i).textContent() || "").trim();
      if (h === "Memory") { memCol = i; break; }
    }
    expect(memCol).toBeGreaterThan(-1);

    // Each row's memory cell must be "no data" or "X.X / XX GB"
    const rows = page.locator("table tbody tr");
    const rowCount = await rows.count();
    for (let r = 0; r < rowCount; r++) {
      const cell = rows.nth(r).locator("td").nth(memCol);
      const text = (await cell.textContent() || "").trim();
      const valid = text === "no data" || /^\d+\.\d+ \/ \d+ GB$/.test(text);
      expect(valid).toBe(true);
    }
  });

  test("Node Manager disk column shows valid format", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(5000);

    // Find the Disk column index
    const headers = page.locator("table thead th");
    const headerCount = await headers.count();
    let diskCol = -1;
    for (let i = 0; i < headerCount; i++) {
      const h = (await headers.nth(i).textContent() || "").trim();
      if (h === "Disk") { diskCol = i; break; }
    }
    expect(diskCol).toBeGreaterThan(-1);

    // Each row's disk cell must be "no data" or "X.X / XX GB"
    const rows = page.locator("table tbody tr");
    const rowCount = await rows.count();
    for (let r = 0; r < rowCount; r++) {
      const cell = rows.nth(r).locator("td").nth(diskCol);
      const text = (await cell.textContent() || "").trim();
      const valid = text === "no data" || /^\d+\.\d+ \/ \d+ GB$/.test(text);
      expect(valid).toBe(true);
    }
  });

  test("Provision dialog validates required fields", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Open provision dialog
    await page.locator("button").filter({ hasText: /Provision Node/i }).click();
    await page.waitForTimeout(500);

    // Submit with empty fields — should show validation error
    await page.locator("button").filter({ hasText: /^Provision$/i }).click();
    await page.waitForTimeout(500);
    const text = await page.textContent("body");
    expect(text).toContain("Host is required");
  });

  test("Provision form submission creates a pending node in list", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Node Manager']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Count nodes before provisioning
    const countBefore = await page.locator("table tbody tr").count();

    // Open provision dialog and fill form
    await page.locator("button").filter({ hasText: /Provision Node/i }).click();
    await page.waitForTimeout(1000);

    // Fill Host and SSH User in the modal form inputs
    const allInputs = page.locator("input");
    const inputCount = await allInputs.count();
    await allInputs.nth(inputCount - 4).fill("10.99.99.99");
    await allInputs.nth(inputCount - 3).fill("testuser");

    await page.locator("button").filter({ hasText: /^Provision$/i }).click();
    await page.waitForTimeout(3000);

    // Node list should have one more row with Pending status
    const countAfter = await page.locator("table tbody tr").count();
    expect(countAfter).toBeGreaterThan(countBefore);
    const bodyText = await page.textContent("body");
    expect(bodyText).toContain("Pending");
  });

  // -- End Node Manager tests -------------------------------------------------

  test("Code IDE opens and renders editor", async ({ page }) => {
    await page.goto(APP);
    await page.waitForSelector("input", { timeout: 10000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
    await page.waitForTimeout(2000);

    const icon = page.locator("img[alt='Code IDE']");
    const box = await icon.boundingBox();
    if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(2000);

    // Code IDE should render the framework IDE component
    const text = await page.textContent("body");
    expect(text).toContain("Code IDE");
  });
});
