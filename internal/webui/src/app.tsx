import { Component } from "@asymmetric-effort/specifyjs";
import { Layout } from "./components/Layout";
import { Dashboard } from "./pages/Dashboard";
import { Projects } from "./pages/Projects";
import { Agents } from "./pages/Agents";
import { DevEnvs } from "./pages/DevEnvs";
import { Console } from "./pages/Console";

type TopNav = "dashboard" | "projects" | "agents" | "dev-envs" | "console";

interface SideNavItem {
  id: string;
  label: string;
}

const SIDE_NAV_ITEMS: Record<TopNav, SideNavItem[]> = {
  dashboard: [],
  projects: [
    { id: "list-projects", label: "List Projects" },
    { id: "create-project", label: "Create Project" },
    { id: "configure-project", label: "Configure Project" },
    { id: "delete-project", label: "Delete Project" },
    { id: "start-project", label: "Start Project" },
    { id: "stop-project", label: "Stop Project" },
    { id: "restart-project", label: "Restart Project" },
  ],
  agents: [
    { id: "list-agents", label: "List Agents" },
    { id: "register-agent", label: "Register Agent" },
  ],
  "dev-envs": [],
  console: [
    { id: "adhoc", label: "Ad-hoc Submit" },
    { id: "cluster-auth", label: "Cluster Auth" },
  ],
};

const DEFAULT_SIDE_NAV: Record<TopNav, string> = {
  dashboard: "",
  projects: "list-projects",
  agents: "list-agents",
  "dev-envs": "",
  console: "adhoc",
};

interface AppState {
  topNav: TopNav;
  sideNav: string;
}

export class App extends Component<Record<string, never>, AppState> {
  state: AppState = {
    topNav: "dashboard",
    sideNav: "",
  };

  handleTopNav = (id: string) => {
    const topNav = id as TopNav;
    this.setState({ topNav, sideNav: DEFAULT_SIDE_NAV[topNav] });
  };

  handleSideNav = (id: string) => {
    this.setState({ sideNav: id });
  };

  render() {
    const { topNav, sideNav } = this.state;
    const sideNavItems = SIDE_NAV_ITEMS[topNav];

    let content: unknown;
    switch (topNav) {
      case "dashboard":
        content = <Dashboard />;
        break;
      case "projects":
        content = <Projects activeSideNav={sideNav} />;
        break;
      case "agents":
        content = <Agents activeSideNav={sideNav} />;
        break;
      case "dev-envs":
        content = <DevEnvs activeSideNav={sideNav} />;
        break;
      case "console":
        content = <Console activeSideNav={sideNav} />;
        break;
    }

    return (
      <Layout
        topNav={topNav}
        sideNav={sideNav}
        sideNavItems={sideNavItems}
        onTopNav={this.handleTopNav}
        onSideNav={this.handleSideNav}
      >
        {content}
      </Layout>
    );
  }
}
