import { Component } from "@asymmetric-effort/specifyjs";
import { TopNav } from "./TopNav";
import { SideNav } from "./SideNav";
import { Footer } from "./Footer";
import type { MeResponse } from "../api/client";

interface SideNavItem {
  id: string;
  label: string;
}

interface LayoutProps {
  topNav: string;
  sideNav: string;
  sideNavItems: SideNavItem[];
  onTopNav: (id: string) => void;
  onSideNav: (id: string) => void;
  user?: MeResponse | null;
  authenticated?: boolean;
  children?: unknown;
}

const TOP_NAV_ITEMS = [
  { id: "dashboard", label: "Dashboard" },
  { id: "projects", label: "Projects" },
  { id: "agents", label: "Agents" },
  { id: "dev-envs", label: "Dev Environments" },
  { id: "console", label: "Console" },
];

export class Layout extends Component<LayoutProps, Record<string, never>> {
  state = {};

  render() {
    const { topNav, sideNav, sideNavItems, onTopNav, onSideNav, user, authenticated, children } = this.props;
    const hasSideNav = sideNavItems.length > 0;

    return (
      <div className="app">
        <TopNav
          items={TOP_NAV_ITEMS}
          active={topNav}
          onNavigate={onTopNav}
          user={user}
          authenticated={authenticated ?? true}
        />
        <div className="body-row">
          {hasSideNav ? (
            <SideNav
              items={sideNavItems}
              active={sideNav}
              onNavigate={onSideNav}
            />
          ) : null}
          <main className={`main${hasSideNav ? " with-side-nav" : ""}`}>
            {children}
          </main>
        </div>
        <Footer />
      </div>
    );
  }
}
