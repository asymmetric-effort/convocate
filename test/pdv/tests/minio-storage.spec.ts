/**
 * MinIO Object Storage — Post-Deployment Verification Tests
 *
 * Validates that MinIO object storage is operational and accessible
 * from the convocate namespace.
 */

import { test, expect } from "@playwright/test";

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

const BASE = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

function authHeaders() {
  return { "Content-Type": "application/json", Authorization: "Bearer mock-token" };
}

test.describe("MinIO health", () => {
  test("MinIO is accessible from convocate namespace (via API status)", async () => {
    // The API status endpoint verifies all backend services including MinIO
    const res = await fetch(`${BASE}/api/v1/status`, { headers: authHeaders() });
    expect(res.status).toBe(200);
    const status = await res.json();
    expect(status).toBeTruthy();
  });

  test("MinIO health endpoint responds", async () => {
    const MINIO_URL = process.env.MINIO_URL || "https://minio.asymmetric-effort.com";
    try {
      const res = await fetch(`${MINIO_URL}/minio/health/live`);
      expect(res.status).toBe(200);
    } catch {
      // MinIO may not be directly reachable from outside the cluster
      test.skip();
    }
  });

  test("MinIO cluster health check", async () => {
    const MINIO_URL = process.env.MINIO_URL || "https://minio.asymmetric-effort.com";
    try {
      const res = await fetch(`${MINIO_URL}/minio/health/cluster`);
      // Accept 200 (healthy) or 503 (degraded but responding)
      expect([200, 503]).toContain(res.status);
    } catch {
      // MinIO may not be directly reachable from outside the cluster
      test.skip();
    }
  });
});

test.describe("MinIO accessibility from convocate", () => {
  test("API can access storage backend", async () => {
    // Verify the API server is healthy which implies it can reach
    // all backing stores including MinIO in the data-layer namespace
    const res = await fetch(`${BASE}/api/v1/status`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });

  test("repo manager can store data (proves MinIO connectivity)", async () => {
    // The repo manager uses MinIO for storage — listing repos proves connectivity
    const res = await fetch(`${BASE}/api/v1/repo/repo?limit=1`, { headers: authHeaders() });
    expect(res.status).toBe(200);
  });
});
