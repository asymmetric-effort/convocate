import { defineConfig } from "@playwright/test";

const APP_URL = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";

export default defineConfig({
  testDir: "./tests",
  timeout: 30000,
  retries: 1,
  use: {
    baseURL: APP_URL,
    headless: true,
    ignoreHTTPSErrors: true,
  },
  projects: [
    { name: "api", testMatch: /api\.spec\.ts/ },
    { name: "ui", testMatch: /ui\.spec\.ts/ },
    { name: "node-lifecycle", testMatch: /node-lifecycle\.spec\.ts/, retries: 0, timeout: 600000 },
  ],
});
