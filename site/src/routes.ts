/**
 * Single source of truth for the docs site nav + routing.
 *
 * Each entry maps a hash route (e.g. "/architecture/three-binaries") to the
 * markdown file under <repo>/docs that supplies its content. The build script
 * (scripts/build-docs-data.ts) walks this list, renders each markdown file to
 * HTML with `marked` + `highlight.js`, and emits src/docs-data.ts.
 *
 * Sections shape the sidebar grouping; order here is the order rendered.
 */

export interface DocRoute {
  /** Hash route, no leading "#". Empty string is the home route. */
  path: string;
  /** Repo-relative path to source markdown, e.g. "docs/index.md". */
  source: string;
  /** Sidebar / nav title. */
  title: string;
}

export interface DocSection {
  /** Section heading shown in the sidebar. */
  heading: string;
  routes: DocRoute[];
}

export const sections: DocSection[] = [
  {
    heading: 'Overview',
    routes: [
      { path: '/', source: 'docs/index.md', title: 'Home' },
      { path: '/getting-started', source: 'docs/getting-started.md', title: 'Getting started' },
    ],
  },
  {
    heading: 'Architecture',
    routes: [
      { path: '/architecture', source: 'docs/architecture/index.md', title: 'Overview' },
      {
        path: '/architecture/three-binaries',
        source: 'docs/architecture/three-binaries.md',
        title: 'Three binaries',
      },
      {
        path: '/architecture/control-plane',
        source: 'docs/architecture/control-plane.md',
        title: 'Control plane',
      },
      {
        path: '/architecture/session-lifecycle',
        source: 'docs/architecture/session-lifecycle.md',
        title: 'Session lifecycle',
      },
      {
        path: '/architecture/image-distribution',
        source: 'docs/architecture/image-distribution.md',
        title: 'Image distribution',
      },
      {
        path: '/architecture/capacity-and-isolation',
        source: 'docs/architecture/capacity-and-isolation.md',
        title: 'Capacity and isolation',
      },
      {
        path: '/architecture/security-posture',
        source: 'docs/architecture/security-posture.md',
        title: 'Security posture',
      },
    ],
  },
  {
    heading: 'Guides',
    routes: [
      {
        path: '/guides/using-the-tui',
        source: 'docs/guides/using-the-tui.md',
        title: 'Using the TUI',
      },
      {
        path: '/guides/session-management',
        source: 'docs/guides/session-management.md',
        title: 'Session management',
      },
      {
        path: '/guides/adding-an-agent',
        source: 'docs/guides/adding-an-agent.md',
        title: 'Adding a new agent',
      },
      {
        path: '/guides/updating-the-cluster',
        source: 'docs/guides/updating-the-cluster.md',
        title: 'Updating the cluster',
      },
      {
        path: '/guides/migrating-orphans',
        source: 'docs/guides/migrating-orphans.md',
        title: 'Migrating pre-v2 orphans',
      },
      {
        path: '/guides/dns-and-networking',
        source: 'docs/guides/dns-and-networking.md',
        title: 'DNS and networking',
      },
      {
        path: '/guides/create-vm',
        source: 'docs/guides/create-vm.md',
        title: 'Provisioning a hypervisor',
      },
    ],
  },
  {
    heading: 'Reference / CLI',
    routes: [
      { path: '/reference/cli/convocate', source: 'docs/reference/cli/convocate.md', title: 'convocate' },
      {
        path: '/reference/cli/convocate-host',
        source: 'docs/reference/cli/convocate-host.md',
        title: 'convocate-host',
      },
      {
        path: '/reference/cli/convocate-agent',
        source: 'docs/reference/cli/convocate-agent.md',
        title: 'convocate-agent',
      },
    ],
  },
  {
    heading: 'Reference / Protocol',
    routes: [
      {
        path: '/reference/protocol/ssh-subsystems',
        source: 'docs/reference/protocol/ssh-subsystems.md',
        title: 'SSH subsystems',
      },
      {
        path: '/reference/protocol/rpc-ops',
        source: 'docs/reference/protocol/rpc-ops.md',
        title: 'RPC ops',
      },
      {
        path: '/reference/protocol/status-events',
        source: 'docs/reference/protocol/status-events.md',
        title: 'Status events',
      },
    ],
  },
  {
    heading: 'Reference',
    routes: [
      {
        path: '/reference/filesystem-layout',
        source: 'docs/reference/filesystem-layout.md',
        title: 'Filesystem layout',
      },
      {
        path: '/reference/systemd-units',
        source: 'docs/reference/systemd-units.md',
        title: 'Systemd units',
      },
      {
        path: '/reference/releases/changelog',
        source: 'docs/reference/releases/changelog.md',
        title: 'Changelog',
      },
      {
        path: '/reference/releases/v2.0.0',
        source: 'docs/reference/releases/v2.0.0.md',
        title: 'v2.0.0 architectural snapshot',
      },
    ],
  },
  {
    heading: 'Help',
    routes: [
      { path: '/glossary', source: 'docs/glossary.md', title: 'Glossary' },
      { path: '/troubleshooting', source: 'docs/troubleshooting.md', title: 'Troubleshooting' },
    ],
  },
  {
    heading: 'Project',
    routes: [
      {
        path: '/project/contributing',
        source: 'docs/project/contributing.md',
        title: 'Contributing',
      },
      { path: '/project/security', source: 'docs/project/security.md', title: 'Security policy' },
      {
        path: '/project/code-of-conduct',
        source: 'docs/project/code-of-conduct.md',
        title: 'Code of conduct',
      },
    ],
  },
];

export const allRoutes: DocRoute[] = sections.flatMap((s) => s.routes);
