import { useRouter } from '@asymmetric-effort/specifyjs';

import { docsData } from '../docs-data';
import { MarkdownPage } from './MarkdownPage';
import { NotFound } from './NotFound';

const normalize = (p: string): string => {
  if (!p || p === '') return '/';
  if (p !== '/' && p.endsWith('/')) return p.slice(0, -1);
  return p;
};

export function PageRouter() {
  const { pathname } = useRouter();
  const path = normalize(pathname);
  if (Object.prototype.hasOwnProperty.call(docsData, path)) {
    return <MarkdownPage path={path} />;
  }
  return <NotFound />;
}
