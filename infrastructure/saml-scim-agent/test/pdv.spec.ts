import { test, expect } from '@playwright/test';

const AGENT_URL = process.env.SAML_SCIM_AGENT_URL || 'https://192.168.3.168:443';

test('health endpoint responds with healthy status', async ({ request }) => {
  const response = await request.get(`${AGENT_URL}/health`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expect(body.status).toBe('healthy');
});

test('SAML metadata endpoint returns valid XML', async ({ request }) => {
  const response = await request.get(`${AGENT_URL}/saml/metadata`);
  expect(response.ok()).toBeTruthy();
  const text = await response.text();
  expect(text).toContain('EntityDescriptor');
  expect(text).toContain('IDPSSODescriptor');
  expect(text).toContain('SingleSignOnService');
});

test('SCIM ServiceProviderConfig endpoint responds', async ({ request }) => {
  const response = await request.get(`${AGENT_URL}/scim/v2/ServiceProviderConfig`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expect(body.schemas).toContain('urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig');
});

test('SAML login with pdv-test user succeeds', async ({ request }) => {
  // Create a minimal SAMLRequest (base64-encoded AuthnRequest)
  const samlRequest = Buffer.from(
    '<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ' +
    'ID="_test_pdv" Version="2.0" IssueInstant="' + new Date().toISOString() + '" ' +
    'AssertionConsumerServiceURL="https://localhost/acs" ' +
    'Destination="' + AGENT_URL + '/saml/sso">' +
    '<saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">https://test-sp</saml:Issuer>' +
    '</samlp:AuthnRequest>'
  ).toString('base64');

  const response = await request.post(`${AGENT_URL}/saml/login`, {
    form: {
      SAMLRequest: samlRequest,
      RelayState: 'test',
      username: 'pdv-test',
      password: 'PdvTest-2026-Secure',
    },
  });
  expect(response.ok()).toBeTruthy();
  const html = await response.text();
  expect(html).toContain('SAMLResponse');
});

test('SAML login with admin user succeeds', async ({ request }) => {
  const adminPassword = process.env.GF_SECURITY_ADMIN_PASSWORD;
  test.skip(!adminPassword, 'GF_SECURITY_ADMIN_PASSWORD not set');

  const samlRequest = Buffer.from(
    '<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ' +
    'ID="_test_admin" Version="2.0" IssueInstant="' + new Date().toISOString() + '" ' +
    'AssertionConsumerServiceURL="https://localhost/acs" ' +
    'Destination="' + AGENT_URL + '/saml/sso">' +
    '<saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">https://test-sp</saml:Issuer>' +
    '</samlp:AuthnRequest>'
  ).toString('base64');

  const response = await request.post(`${AGENT_URL}/saml/login`, {
    form: {
      SAMLRequest: samlRequest,
      RelayState: 'test',
      username: 'admin',
      password: adminPassword,
    },
  });
  expect(response.ok()).toBeTruthy();
  const html = await response.text();
  expect(html).toContain('SAMLResponse');
});
