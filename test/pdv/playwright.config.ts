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
    { name: "ui-applet-windows", testMatch: /applet-windows\.spec\.ts/ },
    { name: "node-manager", testMatch: /node-manager\.spec\.ts/ },
    { name: "contrast", testMatch: /contrast\.spec\.ts/ },
    { name: "node-lifecycle", testMatch: /node-lifecycle\.spec\.ts/, retries: 0, timeout: 120000 },
    { name: "node-metrics", testMatch: /node-metrics\.spec\.ts/, timeout: 60000 },
    { name: "provision-validation", testMatch: /provision-validation\.spec\.ts/ },
    { name: "agent-manager", testMatch: /agent-manager\.spec\.ts/ },
    { name: "code-ide", testMatch: /code-ide\.spec\.ts/ },
    { name: "project-board", testMatch: /project-board\.spec\.ts/ },
  ],
});
