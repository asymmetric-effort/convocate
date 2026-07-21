import { test, expect } from '@playwright/test';

const LOGS_URL = process.env.VICTORIALOGS_URL || 'https://192.168.3.167:443';

test('VictoriaLogs health endpoint responds', async ({ request }) => {
  const response = await request.get(`${LOGS_URL}/health`);
  expect(response.ok()).toBeTruthy();
});
