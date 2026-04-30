import type { SpecNode } from '@asymmetric-effort/specifyjs';
import { useHead } from '@asymmetric-effort/specifyjs';

import { Header } from './Header';
import { Sidebar } from './Sidebar';
import { Footer } from './Footer';

interface LayoutProps {
  children?: SpecNode;
}

export function Layout({ children }: LayoutProps) {
  useHead({
    title: 'Convocate — Orchestrate Containerized Claude CLI Sessions',
    description:
      'A Go-based system for orchestrating isolated, containerized Claude CLI sessions across one or many Linux hosts.',
    keywords: 'convocate, claude, cli, docker, containers, orchestration, sessions',
    author: 'Asymmetric Effort, LLC',
    canonical: 'https://convocate.asymmetric-effort.com',
    og: {
      title: 'Convocate',
      description:
        'Orchestrate isolated, containerized Claude CLI sessions across one or many Linux hosts.',
      type: 'website',
      url: 'https://convocate.asymmetric-effort.com',
      site_name: 'Convocate',
    },
    twitter: {
      card: 'summary',
      title: 'Convocate',
      description:
        'Orchestrate isolated, containerized Claude CLI sessions across one or many Linux hosts.',
    },
    httpEquiv: {
      contentType: 'nosniff',
      referrer: 'strict-origin-when-cross-origin',
    },
  });

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
