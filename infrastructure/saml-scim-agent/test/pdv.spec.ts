import { test, expect } from '@playwright/test';

const AGENT_URL = process.env.SAML_SCIM_AGENT_URL || 'https://192.168.3.168:443';

test('health endpoint responds', async ({ request }) => {
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
  expect(text).toContain('IDPSSODescriptor');
});

test('SCIM ServiceProviderConfig requires auth', async ({ request }) => {
  const response = await request.get(`${AGENT_URL}/scim/v2/ServiceProviderConfig`);
  expect(response.status()).toBe(401);
});

test('SAML login with pdv-test user succeeds', async ({ request }) => {
  const samlRequest = Buffer.from(
    '<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ' +
    'ID="_test_pdv" Version="2.0" IssueInstant="' + new Date().toISOString() + '" ' +
    'AssertionConsumerServiceURL="https://localhost/acs" ' +
    'Destination="' + AGENT_URL + '/saml/sso">' +
    '<saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">https://test-sp</saml:Issuer>' +
    '</samlp:AuthnRequest>'
  ).toString('base64');

  const response = await request.post(`${AGENT_URL}/saml/login`, {
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    data: `SAMLRequest=${encodeURIComponent(samlRequest)}&RelayState=test&username=pdv-test&password=PdvTest-2026-Secure`,
  });

  expect(response.status()).toBe(200);
  const html = await response.text();
  expect(html).toContain('SAMLResponse');
});

test('SAML login with bad password shows login form (no SAMLResponse)', async ({ request }) => {
  const samlRequest = Buffer.from(
    '<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ' +
    'ID="_test_bad" Version="2.0" IssueInstant="' + new Date().toISOString() + '" ' +
    'AssertionConsumerServiceURL="https://localhost/acs" ' +
    'Destination="' + AGENT_URL + '/saml/sso">' +
    '<saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">https://test-sp</saml:Issuer>' +
    '</samlp:AuthnRequest>'
  ).toString('base64');

  const response = await request.post(`${AGENT_URL}/saml/login`, {
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    data: `SAMLRequest=${encodeURIComponent(samlRequest)}&RelayState=test&username=pdv-test&password=wrong-password`,
  });

  const html = await response.text();
  // Bad password returns the login form again, NOT a SAMLResponse
  expect(html).not.toContain('SAMLResponse');
  expect(html).toContain('Sign In');
});
