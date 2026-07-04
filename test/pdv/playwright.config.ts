import { defineConfig } from "@playwright/test";

const APP_URL = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

export default defineConfig({
  globalTeardown: "./global-teardown.ts",
  testDir: "./tests",
  timeout: 30000,
  retries: 1,
  reporter: [["list"], ["./influxdb-reporter.ts"]],
  use: {
    baseURL: APP_URL,
    headless: true,
    ignoreHTTPSErrors: true,
  },
  projects: [
    { name: "ui-applet-windows", testMatch: /applet-windows\.spec\.ts/ },
    { name: "node-manager", testMatch: /node-manager\.spec\.ts/ },
    { name: "contrast", testMatch: /contrast\.spec\.ts/ },
    { name: "node-lifecycle", testMatch: /node-lifecycle\.spec\.ts/, retries: 0, timeout: 120000 },
    { name: "node-metrics", testMatch: /node-metrics\.spec\.ts/, timeout: 60000 },
    { name: "provision-validation", testMatch: /provision-validation\.spec\.ts/ },
    { name: "agent-manager", testMatch: /agent-manager\.spec\.ts/ },
    { name: "code-ide", testMatch: /code-ide\.spec\.ts/ },
    { name: "project-board", testMatch: /project-board\.spec\.ts/ },
    { name: "access-control", testMatch: /access-control\.spec\.ts/ },
    { name: "repo-manager", testMatch: /repo-manager\.spec\.ts/ },
    { name: "support-tool", testMatch: /support-tool\.spec\.ts/ },
    { name: "rbac", testMatch: /rbac\.spec\.ts/ },
    { name: "monitoring", testMatch: /monitoring\.spec\.ts/ },
    { name: "k8s-infrastructure", testMatch: /k8s-infrastructure\.spec\.ts/ },
    { name: "network-boundaries", testMatch: /network-boundaries\.spec\.ts/ },
    { name: "oidc-auth", testMatch: /oidc-auth\.spec\.ts/ },
    { name: "agent-container", testMatch: /agent-container\.spec\.ts/, retries: 0, timeout: 120000 },
    { name: "events-api", testMatch: /events-api\.spec\.ts/ },
    { name: "jaeger-tracing", testMatch: /jaeger-tracing\.spec\.ts/ },
    { name: "log-shipping", testMatch: /log-shipping\.spec\.ts/ },
    { name: "minio-storage", testMatch: /minio-storage\.spec\.ts/ },
    { name: "container-registry", testMatch: /container-registry\.spec\.ts/ },
  ],
});
