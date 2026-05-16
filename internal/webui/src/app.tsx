import { Component } from "@asymmetric-effort/specifyjs";
import { Layout } from "./components/Layout";
import { Dashboard } from "./pages/Dashboard";
import { Projects } from "./pages/Projects";
import { Agents } from "./pages/Agents";
import { DevEnvs } from "./pages/DevEnvs";
import { Console } from "./pages/Console";
import { api, UnauthorizedError } from "./api/client";
import type { MeResponse } from "./api/client";

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
  authenticated: boolean;
  authChecked: boolean;
  user: MeResponse | null;
}

export class App extends Component<Record<string, never>, AppState> {
  state: AppState = {
    topNav: "dashboard",
    sideNav: "",
    authenticated: false,
    authChecked: false,
    user: null,
  };

  componentDidMount() {
    this.checkAuth();
  }

  checkAuth = () => {
    api.getMe().then((user) => {
      this.setState({ authenticated: true, authChecked: true, user });
    }).catch((err) => {
      if (err instanceof UnauthorizedError) {
        this.setState({ authenticated: false, authChecked: true, user: null });
      } else {
        // Network error or server down — treat as authenticated to avoid
        // blocking the UI in dev mode without OAuth configured.
        this.setState({ authenticated: true, authChecked: true, user: null });
      }
    });
  };

  handleTopNav = (id: string) => {
    if (!this.state.authenticated && id !== "dashboard") return;
    const topNav = id as TopNav;
    this.setState({ topNav, sideNav: DEFAULT_SIDE_NAV[topNav] });
  };

  handleSideNav = (id: string) => {
    this.setState({ sideNav: id });
  };

  render() {
    const { topNav, sideNav, authenticated, authChecked, user } = this.state;

    if (!authChecked) {
      return <div className="loading">Loading...</div>;
    }

    const sideNavItems = authenticated ? SIDE_NAV_ITEMS[topNav] : [];

    let content: unknown;
    switch (topNav) {
      case "dashboard":
        content = <Dashboard authenticated={authenticated} />;
        break;
      case "projects":
        content = authenticated
          ? <Projects activeSideNav={sideNav} />
          : <Dashboard authenticated={false} />;
        break;
      case "agents":
        content = authenticated
          ? <Agents activeSideNav={sideNav} />
          : <Dashboard authenticated={false} />;
        break;
      case "dev-envs":
        content = authenticated
          ? <DevEnvs activeSideNav={sideNav} />
          : <Dashboard authenticated={false} />;
        break;
      case "console":
        content = authenticated
          ? <Console activeSideNav={sideNav} />
          : <Dashboard authenticated={false} />;
        break;
    }

    return (
      <Layout
        topNav={topNav}
        sideNav={sideNav}
        sideNavItems={sideNavItems}
        onTopNav={this.handleTopNav}
        onSideNav={this.handleSideNav}
        user={user}
        authenticated={authenticated}
      >
        {content}
      </Layout>
    );
  }
}
