# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: oidc-auth.spec.ts >> OIDC authentication flow >> Grafana login page shows OpenBao SSO option
- Location: tests/oidc-auth.spec.ts:28:7

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: page.goto: net::ERR_CONNECTION_RESET at http://192.168.3.159:38300/login
Call log:
  - navigating to "http://192.168.3.159:38300/login", waiting until "load"

```

# Test source

```ts
  1   | /**
  2   |  * OIDC Authentication — Post-Deployment Verification Tests
  3   |  *
  4   |  * Validates that users can authenticate to Grafana via OpenBao OIDC.
  5   |  * Tests the full OIDC authorization code flow using the pdv-test user
  6   |  * and verifies read-only dashboard access.
  7   |  */
  8   | 
  9   | import { test, expect } from "@playwright/test";
  10  | 
  11  | 
  12  | const GRAFANA_URL = process.env.GRAFANA_URL || "https://grafana.asymmetric-effort.com";
  13  | const AUTH_URL = process.env.AUTH_URL || "https://auth.asymmetric-effort.com";
  14  | 
  15  | test.describe("OIDC authentication flow", () => {
  16  |   test("OpenBao OIDC discovery endpoint is accessible", async () => {
  17  |     try {
  18  |       const res = await fetch(`${AUTH_URL}/v1/identity/oidc/provider/default/.well-known/openid-configuration`);
  19  |       expect(res.status).toBe(200);
  20  |       const data = await res.json();
  21  |       expect(data.issuer).toContain("identity/oidc/provider/default");
  22  |     } catch {
  23  |       // auth.asymmetric-effort.com not reachable from test runner — skip
  24  |       test.skip();
  25  |     }
  26  |   });
  27  | 
  28  |   test("Grafana login page shows OpenBao SSO option", async ({ page }) => {
> 29  |     await page.goto(`${GRAFANA_URL}/login`, { ignoreHTTPSErrors: true });
      |                ^ Error: page.goto: net::ERR_CONNECTION_RESET at http://192.168.3.159:38300/login
  30  |     await expect(page.locator('text=Sign in with OpenBao')).toBeVisible({ timeout: 10000 });
  31  |   });
  32  | 
  33  |   test("pdv-test user can login via OIDC and view dashboard", async ({ page }) => {
  34  |     // Navigate to Grafana login
  35  |     await page.goto(`${GRAFANA_URL}/login`, { ignoreHTTPSErrors: true });
  36  | 
  37  |     // Click the OpenBao SSO button
  38  |     const ssoButton = page.locator('a:has-text("Sign in with OpenBao")');
  39  |     if (!(await ssoButton.isVisible({ timeout: 5000 }).catch(() => false))) {
  40  |       test.skip();
  41  |       return;
  42  |     }
  43  |     await ssoButton.click();
  44  | 
  45  |     // Wait for redirect to OpenBao auth page
  46  |     try {
  47  |       await page.waitForURL(/auth\.asymmetric-effort\.com/, { timeout: 10000 });
  48  |     } catch {
  49  |       // Can't reach auth endpoint from test runner
  50  |       test.skip();
  51  |       return;
  52  |     }
  53  | 
  54  |     // Select userpass auth method if method selector is shown
  55  |     const methodSelect = page.locator('select, [data-test-select="auth-method"]');
  56  |     if (await methodSelect.isVisible({ timeout: 3000 }).catch(() => false)) {
  57  |       await methodSelect.selectOption({ label: "Username" }).catch(() => {});
  58  |     }
  59  | 
  60  |     // Fill in credentials
  61  |     const usernameField = page.locator('input[name="username"], input[id="username"], input[type="text"]').first();
  62  |     const passwordField = page.locator('input[name="password"], input[id="password"], input[type="password"]').first();
  63  |     await expect(usernameField).toBeVisible({ timeout: 10000 });
  64  |     await usernameField.fill("pdv-test");
  65  |     await passwordField.fill("PdvTest-2026-Secure");
  66  | 
  67  |     // Submit
  68  |     const submitBtn = page.locator('button[type="submit"], button:has-text("Sign In"), button:has-text("Log in")').first();
  69  |     await submitBtn.click();
  70  | 
  71  |     // Handle OIDC consent/authorize if prompted
  72  |     const authorizeBtn = page.locator('button:has-text("Authorize"), button:has-text("Allow")');
  73  |     if (await authorizeBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
  74  |       await authorizeBtn.click();
  75  |     }
  76  | 
  77  |     // Wait for redirect back to Grafana
  78  |     await page.waitForURL(/grafana\.asymmetric-effort\.com/, { timeout: 15000 });
  79  | 
  80  |     // Verify we're logged in — should see the home dashboard
  81  |     await expect(page.locator('text=Convocate Cluster Overview')).toBeVisible({ timeout: 10000 });
  82  | 
  83  |     // Verify the user has Viewer role (read-only)
  84  |     // Try to access the admin API — should be forbidden
  85  |     const cookies = await page.context().cookies();
  86  |     const sessionCookie = cookies.find(c => c.name === 'grafana_session');
  87  |     if (sessionCookie) {
  88  |       const adminRes = await page.evaluate(async () => {
  89  |         const res = await fetch('/api/admin/users', { credentials: 'include' });
  90  |         return res.status;
  91  |       });
  92  |       // Viewer should get 403 on admin endpoints
  93  |       expect(adminRes).toBe(403);
  94  |     }
  95  | 
  96  |     // Verify dashboard is visible and has panels
  97  |     await expect(page.locator('[class*="panel"]').first()).toBeVisible({ timeout: 5000 });
  98  |   });
  99  | });
  100 | 
  101 | test.describe("OIDC least-privilege validation", () => {
  102 |   test("pdv-test user gets login-only policy from OpenBao", async () => {
  103 |     try {
  104 |       const res = await fetch(`${AUTH_URL}/v1/auth/userpass/login/pdv-test`, {
  105 |         method: "POST",
  106 |         headers: { "Content-Type": "application/json" },
  107 |         body: JSON.stringify({ password: "PdvTest-2026-Secure" }),
  108 |       });
  109 |       expect(res.status).toBe(200);
  110 |       const data = await res.json();
  111 |       // MFA enforcement means we get mfa_requirement, not a direct token
  112 |       // But the auth block confirms the user authenticated successfully
  113 |       expect(data.auth).toBeTruthy();
  114 |       // Should NOT have admin-policy or viewer-policy
  115 |       const policies = data.auth.policies || data.auth.token_policies || [];
  116 |       expect(policies).not.toContain("admin-policy");
  117 |       expect(policies).not.toContain("viewer-policy");
  118 |     } catch {
  119 |       test.skip();
  120 |     }
  121 |   });
  122 | 
  123 |   test("pdv-test token cannot access OpenBao secrets", async () => {
  124 |     try {
  125 |       // Login to get a token
  126 |       const loginRes = await fetch(`${AUTH_URL}/v1/auth/userpass/login/pdv-test`, {
  127 |         method: "POST",
  128 |         headers: { "Content-Type": "application/json" },
  129 |         body: JSON.stringify({ password: "PdvTest-2026-Secure" }),
```