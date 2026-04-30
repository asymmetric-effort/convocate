import { defineConfig } from 'vite';
import { execSync } from 'child_process';
import { specifyJsSeoPlugin, specifyJsNoscriptPlugin } from '@asymmetric-effort/specifyjs/build';
import { allRoutes } from './src/routes';
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Marked } from 'marked';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, '..');

const buildYear = new Date().getFullYear().toString();
const projectVersion = (() => {
  try {
    return execSync('git describe --tags --abbrev=0', { encoding: 'utf8' }).trim();
  } catch {
    return 'dev';
  }
})();

/** Build noscript sections from the same markdown sources the app uses. */
function buildNoscriptSections() {
  const marked = new Marked();
  const sections: Array<{ id: string; title: string; html: string }> = [];

  for (const route of allRoutes) {
    const srcPath = resolve(repoRoot, route.source);
    const md = readFileSync(srcPath, 'utf8');
    const html = marked.parse(md);
    const id = route.path === '/' ? 'home' : route.path.replace(/^\//, '').replace(/\//g, '-');
    sections.push({ id, title: route.title, html: typeof html === 'string' ? html : String(html) });
  }

  return sections;
}

export default defineConfig({
  base: '/',
  define: {
    __BUILD_YEAR__: JSON.stringify(buildYear),
    __PROJECT_VERSION__: JSON.stringify(projectVersion),
  },
  esbuild: {
    jsx: 'automatic',
    jsxImportSource: '@asymmetric-effort/specifyjs',
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    target: 'es2022',
  },
  server: {
    port: 5173,
  },
  plugins: [
    specifyJsSeoPlugin({
      siteUrl: 'https://convocate.asymmetric-effort.com',
      title: 'Convocate',
      description:
        'A Go-based system for orchestrating isolated, containerized Claude CLI sessions across one or many Linux hosts.',
      routes: allRoutes.map((r) => r.path),
      docsDir: resolve(repoRoot, 'docs'),
      author: 'Asymmetric Effort, LLC',
      license: 'MIT',
      repository: 'https://github.com/asymmetric-effort/convocate',
    }),
    specifyJsNoscriptPlugin({
      title: 'Convocate',
      description:
        'A Go-based system for orchestrating isolated, containerized Claude CLI sessions across one or many Linux hosts.',
      sections: buildNoscriptSections(),
      copyright: `© 2025-${buildYear} Asymmetric Effort, LLC. MIT License.`,
    }),
  ],
});
