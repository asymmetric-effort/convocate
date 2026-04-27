import { Link, useHead } from '@asymmetric-effort/specifyjs';

export function NotFound() {
  useHead({
    title: 'Not found — Convocate',
    description: 'The page you requested does not exist.',
  });

  return (
    <article class="markdown-body">
      <h1>Page not found</h1>
      <p>
        The path you requested does not exist. Use the sidebar to navigate, or{' '}
        <Link to="/">return to the home page</Link>.
      </p>
    </article>
  );
}
