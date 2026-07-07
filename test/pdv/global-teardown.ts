/**
 * Global teardown — runs once after ALL Playwright tests complete.
 * Cleans up any test agents and projects left behind by failed or retried tests.
 */

// lgtm[js/disabling-certificate-validation] — PDV tests run against K8s services with self-signed TLS certificates
process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

function authHeaders() {
  return { "Content-Type": "application/json", Authorization: "Bearer mock-token" };
}

export default async function globalTeardown() {
  console.log("[global-teardown] Running cleanup...");
  // Clean up test agents
  try {
    const res = await fetch(`${BASE}/api/v1/amgr/agent?limit=200`, { headers: authHeaders() });
    if (res.ok) {
      const page = await res.json();
      const testAgents = (page.items || []).filter((a: any) =>
        a.project?.startsWith("pdv-") || a.project?.startsWith("pdvsel")
      );
      for (const agent of testAgents) {
        await fetch(`${BASE}/api/v1/amgr/agent/${agent.id}`, {
          method: "DELETE", headers: authHeaders(),
        }).catch(() => {});
      }
      if (testAgents.length > 0) {
        await new Promise((r) => setTimeout(r, 3000));
      }
    }
  } catch { /* ignore */ }

  // Clean up test projects
  console.log("[global-teardown] Cleaning up test projects...");
  try {
    const res = await fetch(`${BASE}/api/v1/projects?limit=200`, { headers: authHeaders() });
    console.log(`[global-teardown] Projects fetch status: ${res.status}`);
    if (res.ok) {
      const page = await res.json();
      const testProjects = (page.items || []).filter((p: any) =>
        p.name?.startsWith("pdv-") || p.name?.startsWith("pdvsel")
      );
      console.log(`[global-teardown] Found ${testProjects.length} test projects to clean up`);
      for (const proj of testProjects) {
        const delRes = await fetch(`${BASE}/api/v1/projects/${proj.id}`, {
          method: "DELETE", headers: authHeaders(),
        });
        console.log(`[global-teardown] Delete project ${proj.name}: ${delRes.status}`);
      }
    }
  } catch (e: any) { console.log(`[global-teardown] Error: ${e.message}`); }
}
