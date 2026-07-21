import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  timeout: 30000,
  retries: 1,
  use: {
    ignoreHTTPSErrors: true,
  },
});
