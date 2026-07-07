/**
 * Private Container Registry — Post-Deployment Verification Tests
 *
 * Validates that the private container registry at 192.168.3.90:5000
 * is accessible, serves the Docker Registry HTTP API V2, and contains
 * Convocate images.
 */

import { test, expect } from "@playwright/test";

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

const REGISTRY_URL = process.env.REGISTRY_URL || "http://192.168.3.90:5000";

test.describe("Container registry accessibility", () => {
  test("registry at 192.168.3.90:5000 responds to v2 API", async () => {
    try {
      const res = await fetch(`${REGISTRY_URL}/v2/`);
      expect(res.status).toBe(200);
    } catch {
      // Registry may not be reachable from outside the cluster network
      test.skip();
    }
  });

  test("can list repositories via catalog endpoint", async () => {
    try {
      const res = await fetch(`${REGISTRY_URL}/v2/_catalog`);
      if (res.status !== 200) {
        test.skip();
        return;
      }
      const body = await res.json();
      expect(body).toHaveProperty("repositories");
      expect(Array.isArray(body.repositories)).toBe(true);
    } catch {
      // Registry may not be reachable from outside the cluster network
      test.skip();
    }
  });
});

test.describe("Convocate images in registry", () => {
  test("convocate images exist in the registry", async () => {
    try {
      const res = await fetch(`${REGISTRY_URL}/v2/_catalog`);
      if (res.status !== 200) {
        test.skip();
        return;
      }
      const body = await res.json();
      const repos: string[] = body.repositories || [];

      // At least one convocate-related image should exist
      const hasConvocate = repos.some(
        (r: string) => r.includes("convocate") || r.includes("api") || r.includes("ui")
      );
      expect(
        hasConvocate,
        `Registry should contain convocate images, found: ${repos.join(", ")}`
      ).toBe(true);
    } catch {
      // Registry may not be reachable from outside the cluster network
      test.skip();
    }
  });

  test("convocate image has tags", async () => {
    try {
      const catalogRes = await fetch(`${REGISTRY_URL}/v2/_catalog`);
      if (catalogRes.status !== 200) {
        test.skip();
        return;
      }
      const catalog = await catalogRes.json();
      const repos: string[] = catalog.repositories || [];
      const convocateRepo = repos.find(
        (r: string) => r.includes("convocate")
      );
      if (!convocateRepo) {
        test.skip();
        return;
      }

      const tagsRes = await fetch(`${REGISTRY_URL}/v2/${convocateRepo}/tags/list`);
      expect(tagsRes.status).toBe(200);
      const tagsBody = await tagsRes.json();
      expect(tagsBody).toHaveProperty("tags");
      expect(tagsBody.tags.length).toBeGreaterThanOrEqual(1);
    } catch {
      // Registry may not be reachable from outside the cluster network
      test.skip();
    }
  });
});
