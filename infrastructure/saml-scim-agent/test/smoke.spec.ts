import { test, expect } from '@playwright/test';

const AGENT_URL = process.env.SAML_SCIM_AGENT_URL || 'https://192.168.3.169:443';

test('health endpoint responds with healthy status', async ({ request }) => {
  const response = await request.get(`${AGENT_URL}/health`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expect(body.status).toBe('ok');
});

test('SAML metadata endpoint returns valid XML', async ({ request }) => {
  const response = await request.get(`${AGENT_URL}/saml/metadata`);
  expect(response.ok()).toBeTruthy();
  const text = await response.text();
  expect(text).toContain('EntityDescriptor');
});
