import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: ["*.spec.ts"],
  timeout: 60_000,
  retries: 0,
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: process.env.OPENBAO_URL ?? "http://192.168.3.160:443",
  },
});
