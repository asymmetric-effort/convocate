import { defineConfig } from 'vite';
import { execSync } from 'child_process';

const buildYear = new Date().getFullYear().toString();
const projectVersion = (() => {
  try {
    return execSync('git describe --tags --abbrev=0', { encoding: 'utf8' }).trim();
  } catch {
    return 'dev';
  }
})();

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
});
