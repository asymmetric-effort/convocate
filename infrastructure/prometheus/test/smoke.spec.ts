import { test, expect } from "@playwright/test";

const PROMETHEUS_URL: string = process.env.PROMETHEUS_URL ?? "https://prometheus.asymmetric-effort.com";

test.describe.serial("Prometheus Smoke — prometheus-b production verification", () => {
  test("health check — /-/healthy returns 200", async () => {
    const resp = await fetch(`${PROMETHEUS_URL}/-/healthy`);
    expect(resp.status).toBe(200);

    const body = await resp.text();
    expect(body).toContain("Healthy");
  });

  test("ready check — /-/ready returns 200", async () => {
    const resp = await fetch(`${PROMETHEUS_URL}/-/ready`);
    expect(resp.status).toBe(200);

    const body = await resp.text();
    expect(body).toContain("Ready");
  });
});
