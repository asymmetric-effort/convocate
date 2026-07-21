import { test, expect } from '@playwright/test';

const LOGS_URL = process.env.VICTORIALOGS_URL || 'https://192.168.3.166:443';

test('VictoriaLogs health endpoint responds', async ({ request }) => {
  const response = await request.get(`${LOGS_URL}/health`);
  expect(response.ok()).toBeTruthy();
});

test('VictoriaLogs accepts log queries', async ({ request }) => {
  const response = await request.get(`${LOGS_URL}/select/logsql/stats_query?query=*&time=5m`);
  expect(response.status()).toBeLessThan(500);
});
