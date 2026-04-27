import type { SpecNode } from '@asymmetric-effort/specifyjs';

import { Header } from './Header';
import { Sidebar } from './Sidebar';
import { Footer } from './Footer';

interface LayoutProps {
  children?: SpecNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div class="layout">
      <Header />
      <div class="layout-body">
        <Sidebar />
        <main class="content" id="content">
          {children}
        </main>
      </div>
      <Footer />
    </div>
  );
}
