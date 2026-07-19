import { test, expect } from "@playwright/test";

const PROMETHEUS_URL: string = process.env.PROMETHEUS_URL ?? "https://dev.prometheus.asymmetric-effort.com";

test.describe.serial("Prometheus PDV — prometheus-a pre-production verification", () => {
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

  test("targets endpoint — returns configured scrape targets", async () => {
    const resp = await fetch(`${PROMETHEUS_URL}/api/v1/targets`);
    expect(resp.status).toBe(200);

    const body = await resp.json();
    expect(body.status).toBe("success");
    expect(body.data).toBeTruthy();

    // Verify activeTargets exist
    const activeTargets = body.data.activeTargets;
    expect(Array.isArray(activeTargets)).toBe(true);
    expect(activeTargets.length).toBeGreaterThan(0);

    // Verify expected job names are present
    const jobNames = activeTargets.map((t: { labels: { job: string } }) => t.labels.job);
    expect(jobNames).toContain("node-exporter");
    expect(jobNames).toContain("prometheus-a");
  });

  test("config endpoint — returns loaded configuration", async () => {
    const resp = await fetch(`${PROMETHEUS_URL}/api/v1/status/config`);
    expect(resp.status).toBe(200);

    const body = await resp.json();
    expect(body.status).toBe("success");
    expect(body.data.yaml).toBeTruthy();
  });

  test("self-scrape — prometheus can query its own metrics", async () => {
    const resp = await fetch(`${PROMETHEUS_URL}/api/v1/query?query=up`);
    expect(resp.status).toBe(200);

    const body = await resp.json();
    expect(body.status).toBe("success");
    expect(body.data.result).toBeTruthy();
    expect(body.data.result.length).toBeGreaterThan(0);
  });
});
