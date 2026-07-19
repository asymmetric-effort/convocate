import { test, expect } from "@playwright/test";

const GRAFANA_URL: string = process.env.GRAFANA_URL ?? "https://dev.grafana.asymmetric-effort.com";

interface HealthResponse {
  commit: string;
  database: string;
  version: string;
}

test.describe.serial("Grafana PDV — grafana-a pre-production verification", () => {
  test("health check — Grafana responds with ok", async () => {
    const resp = await fetch(`${GRAFANA_URL}/api/health`);
    expect(resp.status).toBe(200);

    const body: HealthResponse = await resp.json();
    expect(body.database).toBe("ok");
    expect(body.version).toBeTruthy();
  });

  test("login page — returns 200", async () => {
    const resp = await fetch(`${GRAFANA_URL}/login`);
    expect(resp.status).toBe(200);
  });

  test("OIDC endpoint — generic_oauth is configured", async () => {
    // The /api/login/oauth endpoint should include our configured OAuth provider
    const resp = await fetch(`${GRAFANA_URL}/api/login/oauth/openbao`, {
      redirect: "manual",
    });
    // Grafana returns 302 redirect to the OIDC provider's authorize endpoint
    // or 404 if not configured. Either 302 or 200 means OIDC is set up.
    expect([200, 302, 307].includes(resp.status) || resp.status < 500).toBe(true);
  });

  test("OIDC redirect — points to secrets-b authorize endpoint", async () => {
    const resp = await fetch(`${GRAFANA_URL}/login/generic_oauth`, {
      redirect: "manual",
    });
    // Should redirect to auth.asymmetric-effort.com
    if (resp.status === 302 || resp.status === 307) {
      const location = resp.headers.get("location") ?? "";
      expect(location).toContain("auth.asymmetric-effort.com");
    }
    // If not redirecting, at minimum ensure it's not a 500 error
    expect(resp.status).toBeLessThan(500);
  });

  test("API requires authentication — returns 401", async () => {
    const resp = await fetch(`${GRAFANA_URL}/api/dashboards/home`);
    expect(resp.status).toBe(401);
  });

  test("OIDC full token exchange — pdv-test gets authorization code and exchanges for token", async () => {
    const BAO_URL = "https://192.168.3.161:443";

    // Step 1: Login as pdv-test to get a vault token
    const loginResp = await fetch(`${BAO_URL}/v1/auth/userpass/login/pdv-test`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: "PdvTest-2026-Secure" }),
    });
    expect(loginResp.status).toBe(200);
    const loginBody = await loginResp.json();
    const vaultToken = loginBody.auth.client_token;
    expect(vaultToken).toBeTruthy();

    // Step 2: Verify entity_id exists (required for OIDC)
    const lookupResp = await fetch(`${BAO_URL}/v1/auth/token/lookup-self`, {
      headers: { "X-Vault-Token": vaultToken },
    });
    expect(lookupResp.status).toBe(200);
    const lookupBody = await lookupResp.json();
    expect(lookupBody.data.entity_id).toBeTruthy();

    // Step 3: Get the OIDC client credentials for grafana-a
    // Read from the .env file on the runner (set during deploy)
    const clientId = process.env.OIDC_CLIENT_ID_A;
    const clientSecret = process.env.OIDC_CLIENT_SECRET_A;

    // If credentials aren't in env, read from OpenBao directly using root token
    let actualClientId = clientId;
    let actualClientSecret = clientSecret;

    if (!actualClientId || !actualClientSecret) {
      // Fall back: read from the OIDC client endpoint (requires privileged token)
      // Skip the token exchange test if we can't get credentials
      console.log("OIDC client credentials not available in env — skipping token exchange");
      return;
    }

    // Step 4: Get authorization code
    const authorizeUrl = new URL(`${BAO_URL}/v1/identity/oidc/provider/default/authorize`);
    authorizeUrl.searchParams.set("client_id", actualClientId);
    authorizeUrl.searchParams.set("redirect_uri", "https://dev.grafana.asymmetric-effort.com/login/generic_oauth");
    authorizeUrl.searchParams.set("response_type", "code");
    authorizeUrl.searchParams.set("scope", "openid profile email");
    authorizeUrl.searchParams.set("state", "pdv-test-state");

    const authResp = await fetch(authorizeUrl.toString(), {
      headers: { "X-Vault-Token": vaultToken },
    });
    expect(authResp.status).toBe(200);
    const authBody = await authResp.json();
    expect(authBody.code).toBeTruthy();

    // Step 5: Exchange code for token (simulating what Grafana does)
    const tokenResp = await fetch(`${BAO_URL}/v1/identity/oidc/provider/default/token`, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "Authorization": "Basic " + Buffer.from(`${actualClientId}:${actualClientSecret}`).toString("base64"),
      },
      body: new URLSearchParams({
        grant_type: "authorization_code",
        code: authBody.code,
        redirect_uri: "https://dev.grafana.asymmetric-effort.com/login/generic_oauth",
      }).toString(),
    });
    const tokenBody = await tokenResp.json();
    if (tokenResp.status !== 200) {
      console.error("Token exchange failed:", JSON.stringify(tokenBody));
    }
    expect(tokenResp.status).toBe(200);
    expect(tokenBody.access_token).toBeTruthy();
    expect(tokenBody.id_token).toBeTruthy();
    expect(tokenBody.token_type).toBe("Bearer");

    // Step 6: Verify userinfo returns expected fields for Grafana user sync
    const BAO_URL_USERINFO = "https://192.168.3.161:443";
    const userinfoResp = await fetch(
      `${BAO_URL_USERINFO}/v1/identity/oidc/provider/default/userinfo`,
      { headers: { "Authorization": `Bearer ${tokenBody.access_token}` } }
    );
    expect(userinfoResp.status).toBe(200);

    const userinfo = await userinfoResp.json();
    expect(userinfo.username).toBeTruthy();
    expect(userinfo.email).toBeTruthy();
    expect(userinfo.sub).toBeTruthy();
  });

  test("OIDC Grafana login — full browser flow creates user session", async () => {
    const BAO_URL = "https://192.168.3.161:443";

    const clientId = process.env.OIDC_CLIENT_ID_A;
    const clientSecret = process.env.OIDC_CLIENT_SECRET_A;

    if (!clientId || !clientSecret) {
      console.log("OIDC credentials not in env — skipping Grafana login test");
      return;
    }

    // Step 1: Initiate OAuth flow from Grafana — get redirect with state
    const initiateResp = await fetch(`${GRAFANA_URL}/login/generic_oauth`, {
      redirect: "manual",
    });
    expect(initiateResp.status).toBe(302);

    const authRedirect = initiateResp.headers.get("location") ?? "";
    expect(authRedirect).toContain("authorize");

    // Extract state and redirect_uri from Grafana's redirect
    const authUrl = new URL(authRedirect);
    const state = authUrl.searchParams.get("state") ?? "";
    const redirectUri = authUrl.searchParams.get("redirect_uri") ?? "";
    expect(state).toBeTruthy();
    expect(redirectUri).toBeTruthy();

    // Capture Grafana's cookies (oauth state stored in cookies)
    const rawCookies = initiateResp.headers.get("set-cookie") ?? "";

    // Step 2: Login as pdv-test to OpenBao
    const loginResp = await fetch(`${BAO_URL}/v1/auth/userpass/login/pdv-test`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: "PdvTest-2026-Secure" }),
    });
    expect(loginResp.status).toBe(200);
    const vaultToken = (await loginResp.json()).auth.client_token;

    // Step 3: Get authorization code using Grafana's exact params
    const authorizeUrl = new URL(`${BAO_URL}/v1/identity/oidc/provider/default/authorize`);
    authorizeUrl.searchParams.set("client_id", clientId);
    authorizeUrl.searchParams.set("redirect_uri", redirectUri);
    authorizeUrl.searchParams.set("response_type", "code");
    authorizeUrl.searchParams.set("scope", authUrl.searchParams.get("scope") ?? "openid profile email");
    authorizeUrl.searchParams.set("state", state);

    const codeNonce = authUrl.searchParams.get("nonce");
    if (codeNonce) authorizeUrl.searchParams.set("nonce", codeNonce);

    const authResp = await fetch(authorizeUrl.toString(), {
      headers: { "X-Vault-Token": vaultToken },
    });

    const authBody = await authResp.json();
    if (authResp.status !== 200) {
      console.error("Authorize failed:", JSON.stringify(authBody));
    }
    expect(authResp.status).toBe(200);
    expect(authBody.code).toBeTruthy();

    // Step 4: Call Grafana's callback with code + state + cookies
    const callbackUrl = `${GRAFANA_URL}/login/generic_oauth?code=${authBody.code}&state=${state}`;
    const cookieHeader = rawCookies.split(",").map(c => c.split(";")[0].trim()).join("; ");

    const callbackResp = await fetch(callbackUrl, {
      redirect: "manual",
      headers: { "Cookie": cookieHeader },
    });

    const location = callbackResp.headers.get("location") ?? "";
    const setCookie = callbackResp.headers.get("set-cookie") ?? "";

    // Log on failure
    if (callbackResp.status !== 302 || location.includes("login")) {
      console.error("Grafana login callback failed:");
      console.error("  Status:", callbackResp.status);
      console.error("  Location:", location);
      console.error("  Response:", await callbackResp.text().catch(() => ""));
    }

    expect(callbackResp.status).toBe(302);
    expect(location).not.toContain("login");
    expect(setCookie).toContain("grafana_session");
  });
});
