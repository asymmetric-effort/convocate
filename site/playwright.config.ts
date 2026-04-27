import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: 1,
  reporter: 'list',
  use: {
    // Post-deployment tests target the live site.
    // Override with SITE_URL env var for local testing (e.g. vite preview).
    baseURL: process.env.SITE_URL ?? 'https://convocate.asymmetric-effort.com',
    headless: true,
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
