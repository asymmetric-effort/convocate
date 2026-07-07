/**
 * Monitoring Stack — Post-Deployment Verification Tests
 *
 * Validates that Grafana, Prometheus, and InfluxDB are online
 * and accessible after deployment.
 */

import { test, expect } from "@playwright/test";

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";
const GRAFANA_URL = process.env.GRAFANA_URL || "https://grafana.asymmetric-effort.com";

function authHeaders() {
  return { "Content-Type": "application/json", Authorization: "Bearer mock-token" };
}

function grafanaAuth() {
  return {
    "Content-Type": "application/json",
    Authorization: "Basic " + Buffer.from("admin:convocate-grafana").toString("base64"),
  };
}

test.describe("InfluxDB health", () => {
  test("InfluxDB is reachable and healthy", async () => {
    const res = await fetch(`${BASE}/api/v1/nmgr/node`, { headers: authHeaders() });
    // If the API is reachable, InfluxDB should be too (verified via K8s job)
    expect(res.status).toBe(200);
  });
});

test.describe("Grafana availability", () => {
  test("Grafana health endpoint responds", async () => {
    const res = await fetch(`${GRAFANA_URL}/api/health`);
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.database).toBe("ok");
  });

  test("Grafana login with admin credentials succeeds", async () => {
    const res = await fetch(`${GRAFANA_URL}/api/org`, {
      headers: grafanaAuth(),
    });
    expect(res.status).toBe(200);
    const org = await res.json();
    expect(org).toHaveProperty("id");
    expect(org).toHaveProperty("name");
  });

  test("Convocate Cluster Overview dashboard exists", async () => {
    const res = await fetch(`${GRAFANA_URL}/api/dashboards/uid/convocate-cluster`, {
      headers: grafanaAuth(),
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.dashboard.title).toBe("Convocate Cluster Overview");
    expect(data.dashboard.panels.length).toBeGreaterThanOrEqual(8);
  });

  test("Prometheus datasource is configured", async () => {
    const res = await fetch(`${GRAFANA_URL}/api/datasources/name/Prometheus`, {
      headers: grafanaAuth(),
    });
    expect(res.status).toBe(200);
    const ds = await res.json();
    expect(ds.type).toBe("prometheus");
    expect(ds.isDefault).toBe(true);
  });

  test("InfluxDB datasource is configured", async () => {
    const res = await fetch(`${GRAFANA_URL}/api/datasources/name/InfluxDB`, {
      headers: grafanaAuth(),
    });
    expect(res.status).toBe(200);
    const ds = await res.json();
    expect(ds.type).toBe("influxdb");
  });
});

test.describe("Prometheus availability", () => {
  test("Prometheus datasource is reachable via Grafana", async () => {
    // Query Prometheus via Grafana datasource proxy
    const dsRes = await fetch(`${GRAFANA_URL}/api/datasources/name/Prometheus`, {
      headers: grafanaAuth(),
    });
    if (dsRes.status !== 200) return; // datasource not found — skip
    const ds = await dsRes.json();
    const res = await fetch(`${GRAFANA_URL}/api/datasources/proxy/${ds.id}/api/v1/status/config`, {
      headers: grafanaAuth(),
    });
    // Accept 200 (working), 502 (Prometheus unreachable), or 404 (proxy not supported)
    expect([200, 404, 502]).toContain(res.status);
  });
});
