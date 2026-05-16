import { Component } from "@asymmetric-effort/specifyjs";
import { TopNav } from "./TopNav";
import { SideNav } from "./SideNav";

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
  children?: unknown;
}

const TOP_NAV_ITEMS = [
  { id: "dashboard", label: "Dashboard" },
  { id: "projects", label: "Projects" },
  { id: "agents", label: "Agents" },
  { id: "console", label: "Console" },
];

export class Layout extends Component<LayoutProps, Record<string, never>> {
  state = {};

  render() {
    const { topNav, sideNav, sideNavItems, onTopNav, onSideNav, children } = this.props;
    const hasSideNav = sideNavItems.length > 0;

    return (
      <div className="app">
        <TopNav
          items={TOP_NAV_ITEMS}
          active={topNav}
          onNavigate={onTopNav}
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
      </div>
    );
  }
}
