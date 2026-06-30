# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: agent-manager.spec.ts >> Agent Manager applet >> shows agent count in the footer
- Location: tests/agent-manager.spec.ts:72:7

# Error details

```
Error: expect(locator).toBeVisible() failed

Locator: locator('[data-testid="agent-manager"]')
Expected: visible
Timeout: 10000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" with timeout 10000ms
  - waiting for locator('[data-testid="agent-manager"]')

```

```yaml
- menubar "System panel":
  - button "Activities"
  - text: Agent Manager
  - timer "System clock": Tue, Jun 30 21:22:30
  - button "Mock Admin"
- toolbar "Application launcher":
  - button "Node Manager":
    - img "Node Manager"
  - button "Agent Manager" [pressed]:
    - img "Agent Manager"
    - tooltip "Agent Manager"
  - button "Convocate Project Board":
    - img "Convocate Project Board"
  - button "Code Monkey IDE":
    - img "Code Monkey IDE"
  - button "Access Control":
    - img "Access Control"
  - button "Repo Manager":
    - img "Repo Manager"
  - button "Support Tool":
    - img "Support Tool"
- main:
  - application "Desktop workspace":
    - dialog "Agent Manager":
      - toolbar "Agent Manager window controls":
        - text: Agent Manager
        - button "Minimize"
        - button "Maximize"
        - button "Close"
```

# Test source

```ts
  1   | /**
  2   |  * Agent Manager Applet — Post-Deployment Verification Tests
  3   |  *
  4   |  * Validates that the Agent Manager applet loads, displays agent data
  5   |  * from the API, and supports create/stop/delete operations.
  6   |  */
  7   | 
  8   | import { test, expect, Page } from "@playwright/test";
  9   | 
  10  | // Disable TLS for direct API calls
  11  | process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";
  12  | 
  13  | const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";
  14  | 
  15  | function authHeaders(): Record<string, string> {
  16  |   return {
  17  |     "Content-Type": "application/json",
  18  |     Authorization: "Bearer mock-token",
  19  |   };
  20  | }
  21  | 
  22  | // ---------------------------------------------------------------------------
  23  | // Helpers
  24  | // ---------------------------------------------------------------------------
  25  | 
  26  | async function login(page: Page): Promise<void> {
  27  |   await page.goto("/");
  28  |   await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });
  29  |   await page.locator('input[placeholder="Username"]').fill("admin");
  30  |   await page.locator('input[placeholder="Password"]').fill("test");
  31  |   await page.locator('input[placeholder="MFA Token"]').fill("123456");
  32  |   await page.locator('button:has-text("Sign In")').click();
  33  |   await expect(page.locator("[data-dock-item-id]").first()).toBeVisible({ timeout: 15000 });
  34  | }
  35  | 
  36  | async function openAgentManager(page: Page): Promise<void> {
  37  |   await page.locator('[data-dock-item-id="amgr"]').click();
  38  |   await expect(
  39  |     page.locator('[role="dialog"][aria-label="Agent Manager"]')
  40  |   ).toBeVisible({ timeout: 5000 });
  41  |   await expect(
  42  |     page.locator('[data-testid="agent-manager"]')
> 43  |   ).toBeVisible({ timeout: 10000 });
      |     ^ Error: expect(locator).toBeVisible() failed
  44  | }
  45  | 
  46  | // ---------------------------------------------------------------------------
  47  | // Tests: Agent Manager loads
  48  | // ---------------------------------------------------------------------------
  49  | 
  50  | test.describe("Agent Manager applet", () => {
  51  |   test.beforeEach(async ({ page }) => {
  52  |     await login(page);
  53  |     await openAgentManager(page);
  54  |   });
  55  | 
  56  |   test("displays the toolbar with Agents title and action buttons", async ({ page }) => {
  57  |     await expect(
  58  |       page.locator('[data-testid="agent-manager"] >> text=Agents').first()
  59  |     ).toBeVisible();
  60  | 
  61  |     // Create Agent button
  62  |     await expect(
  63  |       page.locator('[data-testid="agent-manager"] button:has-text("Create Agent")')
  64  |     ).toBeVisible();
  65  | 
  66  |     // Refresh button
  67  |     await expect(
  68  |       page.locator('[data-testid="agent-manager"] button:has-text("Refresh")')
  69  |     ).toBeVisible();
  70  |   });
  71  | 
  72  |   test("shows agent count in the footer", async ({ page }) => {
  73  |     await expect(
  74  |       page.locator('text=/\\d+ agents?/')
  75  |     ).toBeVisible({ timeout: 10000 });
  76  |   });
  77  | 
  78  |   test("shows agent count or empty state after loading", async ({ page }) => {
  79  |     // Wait for loading to complete
  80  |     await page.waitForTimeout(3000);
  81  |     // Should show either the empty message or the agent count footer
  82  |     const hasCount = await page.locator('text=/\\d+ agents?/').isVisible();
  83  |     const hasEmpty = await page.locator('text=No agents running').isVisible();
  84  |     expect(hasCount || hasEmpty, "Should show agent count or empty state").toBeTruthy();
  85  |   });
  86  | });
  87  | 
  88  | // ---------------------------------------------------------------------------
  89  | // Tests: Create Agent dialog
  90  | // ---------------------------------------------------------------------------
  91  | 
  92  | test.describe("Agent Manager create dialog", () => {
  93  |   test.beforeEach(async ({ page }) => {
  94  |     await login(page);
  95  |     await openAgentManager(page);
  96  |   });
  97  | 
  98  |   test("opens when Create Agent is clicked", async ({ page }) => {
  99  |     await page.locator('[data-testid="agent-manager"] button:has-text("Create Agent")').click();
  100 |     await expect(page.locator('text=Create Agent').first()).toBeVisible({ timeout: 5000 });
  101 |     await expect(page.locator('input[placeholder="Project name"]')).toBeVisible();
  102 |     await expect(page.locator('input[placeholder*="Node ID"]')).toBeVisible();
  103 |   });
  104 | 
  105 |   test("validates required fields", async ({ page }) => {
  106 |     await page.locator('[data-testid="agent-manager"] button:has-text("Create Agent")').click();
  107 |     await expect(page.locator('input[placeholder="Project name"]')).toBeVisible({ timeout: 5000 });
  108 | 
  109 |     // Submit without filling fields
  110 |     await page.locator('button:has-text("Create")').last().click();
  111 |     await expect(page.locator('text=Project name is required')).toBeVisible();
  112 |   });
  113 | 
  114 |   test("shows capabilities and network policy fields", async ({ page }) => {
  115 |     await page.locator('[data-testid="agent-manager"] button:has-text("Create Agent")').click();
  116 |     await expect(page.locator('input[placeholder="Project name"]')).toBeVisible({ timeout: 5000 });
  117 | 
  118 |     // Network policy field should be visible
  119 |     await expect(page.locator('input[placeholder*="Additional egress"]')).toBeVisible();
  120 |     // Default network info text
  121 |     await expect(page.locator('text=Default: Anthropic API')).toBeVisible();
  122 |   });
  123 | 
  124 |   test("can be cancelled", async ({ page }) => {
  125 |     await page.locator('[data-testid="agent-manager"] button:has-text("Create Agent")').click();
  126 |     await expect(page.locator('input[placeholder="Project name"]')).toBeVisible({ timeout: 5000 });
  127 | 
  128 |     await page.locator('button:has-text("Cancel")').click();
  129 |     await expect(page.locator('input[placeholder="Project name"]')).not.toBeVisible();
  130 |   });
  131 | });
  132 | 
  133 | // ---------------------------------------------------------------------------
  134 | // Tests: Agent lifecycle via API
  135 | // ---------------------------------------------------------------------------
  136 | 
  137 | test.describe("Agent Manager API lifecycle", () => {
  138 |   let testAgentId: string | null = null;
  139 | 
  140 |   test.afterEach(async () => {
  141 |     // Cleanup: delete the test agent if it was created
  142 |     if (testAgentId) {
  143 |       await fetch(`${BASE}/api/v1/amgr/agent/${testAgentId}`, {
```