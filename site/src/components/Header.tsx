import { Link, useState, useEffect } from '@asymmetric-effort/specifyjs';

type Theme = 'light' | 'dark';

const STORAGE_KEY = 'convocate-theme';

const initialTheme = (): Theme => {
  if (typeof window === 'undefined') return 'light';
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (stored === 'light' || stored === 'dark') return stored;
  if (window.matchMedia?.('(prefers-color-scheme: dark)').matches) return 'dark';
  return 'light';
};

export function Header() {
  const [theme, setTheme] = useState<Theme>(initialTheme());

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    try {
      window.localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      // localStorage may be disabled (private mode); not load-bearing.
    }
  }, [theme]);

  const toggle = () => setTheme((t) => (t === 'light' ? 'dark' : 'light'));

  return (
    <header class="site-header">
      <div class="site-header-inner">
        <Link to="/" class="brand" aria-label="Convocate home">
          <span class="brand-mark">C</span>
          <span class="brand-text">Convocate</span>
        </Link>
        <nav class="top-nav">
          <Link to="/getting-started" class="top-nav-link">
            Getting started
          </Link>
          <Link to="/architecture" class="top-nav-link">
            Architecture
          </Link>
          <Link to="/reference/cli/convocate" class="top-nav-link">
            Reference
          </Link>
          <a
            class="top-nav-link"
            href="https://github.com/asymmetric-effort/convocate"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub
          </a>
        </nav>
        <button
          type="button"
          class="theme-toggle"
          onClick={toggle}
          aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
          title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
        >
          {theme === 'light' ? '🌙' : '☀'}
        </button>
      </div>
    </header>
  );
}
