import { useEffect, useHead } from '@asymmetric-effort/specifyjs';

import { docsData } from '../docs-data';

interface Props {
  path: string;
}

export function MarkdownPage({ path }: Props) {
  const page = docsData[path];

  useHead({
    title: page ? `${page.title} — Convocate` : 'Convocate',
    description: page?.description ?? '',
  });

  useEffect(() => {
    // Scroll to top on route change so each page starts at the heading.
    if (typeof window !== 'undefined') {
      window.scrollTo({ top: 0, behavior: 'instant' });
    }
  }, [path]);

  if (!page) {
    return <div class="markdown-body">Loading…</div>;
  }

  return <article class="markdown-body" dangerouslySetInnerHTML={{ __html: page.html }} />;
}
