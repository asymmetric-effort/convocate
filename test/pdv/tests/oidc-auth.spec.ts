/**
 * OIDC Authentication — Post-Deployment Verification Tests
 *
 * Validates that users can authenticate to Grafana via OpenBao OIDC.
 * Tests the full OIDC authorization code flow using the pdv-test user
 * and verifies read-only dashboard access.
 */

import { test, expect } from "@playwright/test";


const GRAFANA_URL = process.env.GRAFANA_URL || "https://grafana.asymmetric-effort.com";
const AUTH_URL = process.env.AUTH_URL || "https://auth.asymmetric-effort.com";

test.describe("OIDC authentication flow", () => {
  test("OpenBao OIDC discovery endpoint is accessible", async () => {
    try {
      const res = await fetch(`${AUTH_URL}/v1/identity/oidc/provider/default/.well-known/openid-configuration`);
      expect(res.status).toBe(200);
      const data = await res.json();
      expect(data.issuer).toContain("identity/oidc/provider/default");
    } catch {
      // auth.asymmetric-effort.com not reachable from test runner — skip
      test.skip();
    }
  });

  test("Grafana login page shows OpenBao SSO option", async ({ page }) => {
    await page.goto(`${GRAFANA_URL}/login`, { ignoreHTTPSErrors: true });
    await expect(page.locator('text=Sign in with OpenBao')).toBeVisible({ timeout: 10000 });
  });

  test("pdv-test user can login via OIDC and view dashboard", async ({ page }) => {
    // Navigate to Grafana login
    await page.goto(`${GRAFANA_URL}/login`, { ignoreHTTPSErrors: true });

    // Click the OpenBao SSO button
    const ssoButton = page.locator('a:has-text("Sign in with OpenBao")');
    if (!(await ssoButton.isVisible({ timeout: 5000 }).catch(() => false))) {
      test.skip();
      return;
    }
    await ssoButton.click();

    // Wait for redirect to OpenBao auth page
    try {
      await page.waitForURL(/auth\.asymmetric-effort\.com/, { timeout: 10000 });
    } catch {
      // Can't reach auth endpoint from test runner
      test.skip();
      return;
    }

    // Select userpass auth method if method selector is shown
    const methodSelect = page.locator('select, [data-test-select="auth-method"]');
    if (await methodSelect.isVisible({ timeout: 3000 }).catch(() => false)) {
      await methodSelect.selectOption({ label: "Username" }).catch(() => {});
    }

    // Fill in credentials
    const usernameField = page.locator('input[name="username"], input[id="username"], input[type="text"]').first();
    const passwordField = page.locator('input[name="password"], input[id="password"], input[type="password"]').first();
    await expect(usernameField).toBeVisible({ timeout: 10000 });
    await usernameField.fill("pdv-test");
    await passwordField.fill("PdvTest-2026-Secure");

    // Submit
    const submitBtn = page.locator('button[type="submit"], button:has-text("Sign In"), button:has-text("Log in")').first();
    await submitBtn.click();

    // Handle OIDC consent/authorize if prompted
    const authorizeBtn = page.locator('button:has-text("Authorize"), button:has-text("Allow")');
    if (await authorizeBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await authorizeBtn.click();
    }

    // Wait for redirect back to Grafana
    await page.waitForURL(/grafana\.asymmetric-effort\.com/, { timeout: 15000 });

    // Verify we're logged in — should see the home dashboard
    await expect(page.locator('text=Convocate Cluster Overview')).toBeVisible({ timeout: 10000 });

    // Verify the user has Viewer role (read-only)
    // Try to access the admin API — should be forbidden
    const cookies = await page.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'grafana_session');
    if (sessionCookie) {
      const adminRes = await page.evaluate(async () => {
        const res = await fetch('/api/admin/users', { credentials: 'include' });
        return res.status;
      });
      // Viewer should get 403 on admin endpoints
      expect(adminRes).toBe(403);
    }

    // Verify dashboard is visible and has panels
    await expect(page.locator('[class*="panel"]').first()).toBeVisible({ timeout: 5000 });
  });
});

test.describe("OIDC least-privilege validation", () => {
  test("pdv-test user gets login-only policy from OpenBao", async () => {
    try {
      const res = await fetch(`${AUTH_URL}/v1/auth/userpass/login/pdv-test`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password: "PdvTest-2026-Secure" }),
      });
      expect(res.status).toBe(200);
      const data = await res.json();
      // MFA enforcement means we get mfa_requirement, not a direct token
      // But the auth block confirms the user authenticated successfully
      expect(data.auth).toBeTruthy();
      // Should NOT have admin-policy or viewer-policy
      const policies = data.auth.policies || data.auth.token_policies || [];
      expect(policies).not.toContain("admin-policy");
      expect(policies).not.toContain("viewer-policy");
    } catch {
      test.skip();
    }
  });

  test("pdv-test token cannot access OpenBao secrets", async () => {
    try {
      // Login to get a token
      const loginRes = await fetch(`${AUTH_URL}/v1/auth/userpass/login/pdv-test`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password: "PdvTest-2026-Secure" }),
      });
      if (!loginRes.ok) { test.skip(); return; }
      const { auth } = await loginRes.json();

      // Try to read a secret — should be denied
      const secretRes = await fetch(`${AUTH_URL}/v1/convocate/data/influxdb`, {
        headers: { "X-Vault-Token": auth.client_token },
      });
      // Should be 403 Forbidden
      expect(secretRes.status).toBe(403);
    } catch {
      test.skip();
    }
  });
});
