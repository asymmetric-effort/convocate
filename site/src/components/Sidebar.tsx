import { Link, useRouter } from '@asymmetric-effort/specifyjs';

import { sections } from '../routes';

const normalize = (p: string): string => {
  if (!p || p === '') return '/';
  if (p !== '/' && p.endsWith('/')) return p.slice(0, -1);
  return p;
};

export function Sidebar() {
  const { pathname } = useRouter();
  const current = normalize(pathname);

  return (
    <aside class="sidebar" aria-label="Documentation navigation">
      <nav>
        {sections.map((section) => (
          <div class="sidebar-section" key={section.heading}>
            <h2 class="sidebar-heading">{section.heading}</h2>
            <ul class="sidebar-list">
              {section.routes.map((r) => {
                const active = r.path === current;
                return (
                  <li key={r.path}>
                    <Link
                      to={r.path}
                      className={active ? 'sidebar-link sidebar-link-active' : 'sidebar-link'}
                    >
                      {r.title}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
    </aside>
  );
}
