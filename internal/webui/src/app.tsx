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

const STARTUP_TIMEOUT_MS = 60000;

interface AppState {
  topNav: TopNav;
  sideNav: string;
  authenticated: boolean;
  authChecked: boolean;
  user: MeResponse | null;
  startupError: string;
}

export class App extends Component<Record<string, never>, AppState> {
  state: AppState = {
    topNav: "dashboard",
    sideNav: "",
    authenticated: false,
    authChecked: false,
    user: null,
    startupError: "",
  };

  private startupTimer: ReturnType<typeof setTimeout> | null = null;

  componentDidMount() {
    this.startupTimer = setTimeout(() => {
      if (!this.state.authChecked) {
        this.setState({
          authChecked: true,
          startupError: "Startup timeout: the system did not respond within 60 seconds. "
            + "Check that the Router API and identity provider (GitHub) are reachable.",
        });
      }
    }, STARTUP_TIMEOUT_MS);
    this.checkAuth();
  }

  componentWillUnmount() {
    if (this.startupTimer) {
      clearTimeout(this.startupTimer);
      this.startupTimer = null;
    }
  }

  checkAuth = () => {
    api.getMe().then((user) => {
      if (this.startupTimer) clearTimeout(this.startupTimer);
      this.setState({ authenticated: true, authChecked: true, user, startupError: "" });
    }).catch((err) => {
      if (this.startupTimer) clearTimeout(this.startupTimer);
      if (err instanceof UnauthorizedError) {
        this.setState({ authenticated: false, authChecked: true, user: null, startupError: "" });
      } else {
        const message = err instanceof Error ? err.message : "Unknown error";
        this.setState({
          authenticated: false,
          authChecked: true,
          user: null,
          startupError: `Unable to connect to the identity provider: ${message}. `
            + "Verify that the Router API is running and GitHub OAuth is configured correctly.",
        });
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
    const { topNav, sideNav, authenticated, authChecked, user, startupError } = this.state;

    if (!authChecked) {
      return (
        <div className="startup-screen">
          <div className="startup-spinner">
            <div className="nav-brand">convocate</div>
            <p>Connecting to services...</p>
          </div>
        </div>
      );
    }

    if (startupError) {
      return (
        <div className="startup-screen">
          <div className="startup-error-dialog">
            <div className="nav-brand">convocate</div>
            <div className="error">{startupError}</div>
            <button onClick={() => { this.setState({ authChecked: false, startupError: "" }); this.checkAuth(); }}>
              Retry
            </button>
          </div>
        </div>
      );
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
