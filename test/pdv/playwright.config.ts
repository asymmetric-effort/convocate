import { defineConfig } from "@playwright/test";

const API_URL = process.env.API_URL || "http://convocate-api.convocate.svc:8443";
const UI_URL = process.env.UI_URL || "http://convocate-ui.convocate.svc:8080";

export default defineConfig({
  testDir: "./tests",
  timeout: 30000,
  retries: 1,
  use: {
    baseURL: UI_URL,
    headless: true,
  },
  projects: [
    {
      name: "api",
      testMatch: /api\.spec\.ts/,
    },
    {
      name: "ui",
      testMatch: /ui\.spec\.ts/,
    },
  ],
});
