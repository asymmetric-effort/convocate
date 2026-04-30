import { test, expect } from '@playwright/test';

// Post-deployment verification tests for robots.txt and sitemap.xml.
// Validates that both files are present, correctly formatted, and
// contain the expected content after each deploy.

// Where we fetch from (may be localhost for local testing).
const SITE_ORIGIN = process.env.SITE_URL ?? 'https://convocate.asymmetric-effort.com';

// The canonical production URL baked into the generated files at build time.
const CANONICAL_ORIGIN = 'https://convocate.asymmetric-effort.com';

// Canonical routes defined in site/src/routes.ts — these must all appear
// in the sitemap as direct (non-hash) URLs.
const EXPECTED_ROUTES = [
  '/',
  '/getting-started',
  '/architecture',
  '/architecture/three-binaries',
  '/architecture/control-plane',
  '/architecture/session-lifecycle',
  '/architecture/image-distribution',
  '/architecture/capacity-and-isolation',
  '/architecture/security-posture',
  '/guides/using-the-tui',
  '/guides/session-management',
  '/guides/adding-an-agent',
  '/guides/updating-the-cluster',
  '/guides/migrating-orphans',
  '/guides/dns-and-networking',
  '/guides/create-vm',
  '/reference/cli/convocate',
  '/reference/cli/convocate-host',
  '/reference/cli/convocate-agent',
  '/reference/protocol/ssh-subsystems',
  '/reference/protocol/rpc-ops',
  '/reference/protocol/status-events',
  '/reference/filesystem-layout',
  '/reference/systemd-units',
  '/reference/releases/changelog',
  '/reference/releases/v2.0.0',
  '/glossary',
  '/troubleshooting',
  '/project/contributing',
  '/project/security',
  '/project/code-of-conduct',
];

test.describe('robots.txt verification', () => {
  let body: string;

  test.beforeAll(async ({ request }) => {
    const res = await request.get(`${SITE_ORIGIN}/robots.txt`);
    expect(res.status()).toBe(200);
    body = await res.text();
  });

  test('is served with correct content type', async ({ request }) => {
    const res = await request.get(`${SITE_ORIGIN}/robots.txt`);
    const ct = res.headers()['content-type'] ?? '';
    expect(ct).toContain('text/plain');
  });

  test('allows all user agents', () => {
    expect(body).toContain('User-agent: *');
    expect(body).toContain('Allow: /');
  });

  test('references the sitemap URL', () => {
    expect(body).toContain(`Sitemap: ${CANONICAL_ORIGIN}/sitemap.xml`);
  });

  test('does not disallow any paths', () => {
    expect(body).not.toContain('Disallow:');
  });
});

test.describe('sitemap.xml verification', () => {
  let body: string;

  test.beforeAll(async ({ request }) => {
    const res = await request.get(`${SITE_ORIGIN}/sitemap.xml`);
    expect(res.status()).toBe(200);
    body = await res.text();
  });

  test('is valid XML with correct namespace', () => {
    expect(body).toMatch(/^<\?xml version="1\.0" encoding="UTF-8"\?>/);
    expect(body).toContain('xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"');
    expect(body).toContain('<urlset');
    expect(body).toContain('</urlset>');
  });

  test('contains all expected route URLs', () => {
    for (const route of EXPECTED_ROUTES) {
      const expectedUrl = route === '/'
        ? `${CANONICAL_ORIGIN}/`
        : `${CANONICAL_ORIGIN}${route}`;
      expect(body, `missing route: ${route}`).toContain(`<loc>${expectedUrl}</loc>`);
    }
  });

  test('every <url> entry has a <lastmod> date', () => {
    const urlBlocks = body.match(/<url>[\s\S]*?<\/url>/g) ?? [];
    expect(urlBlocks.length).toBeGreaterThan(0);

    for (const block of urlBlocks) {
      expect(block).toMatch(/<lastmod>\d{4}-\d{2}-\d{2}<\/lastmod>/);
    }
  });

  test('all <loc> values are absolute HTTPS URLs', () => {
    const locs = [...body.matchAll(/<loc>(.*?)<\/loc>/g)].map((m) => m[1]);
    expect(locs.length).toBeGreaterThan(0);

    for (const loc of locs) {
      expect(loc, `non-HTTPS URL: ${loc}`).toMatch(/^https:\/\//);
    }
  });

  test('includes at least as many URLs as defined routes', () => {
    const locs = [...body.matchAll(/<loc>/g)];
    expect(locs.length).toBeGreaterThanOrEqual(EXPECTED_ROUTES.length);
  });
});
