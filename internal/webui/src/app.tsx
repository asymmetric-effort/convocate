import { Component } from "@asymmetric-effort/specifyjs";
import { Layout } from "./components/Layout";
import { Dashboard } from "./pages/Dashboard";
import { Projects } from "./pages/Projects";
import { Agents } from "./pages/Agents";
import { DevEnvs } from "./pages/DevEnvs";
import { Console } from "./pages/Console";
import type { MeResponse, ProjectInfo, HostHealthInfo } from "./api/client";

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

interface ComponentStatus {
  name: string;
  status: "running" | "stopped" | "unknown";
}

interface AppProps {
  initialAuthenticated: boolean;
  initialUser: MeResponse | null;
  initialError: string;
  projects: ProjectInfo[];
  hosts: HostHealthInfo[];
  components: ComponentStatus[];
}

interface AppState {
  topNav: TopNav;
  sideNav: string;
}

export class App extends Component<AppProps, AppState> {
  state: AppState = { topNav: "dashboard", sideNav: "" };

  render() {
    const { initialAuthenticated: authenticated, initialUser: user, initialError } = this.props;
    appProps = this.props;
    const topNav = navState.topNav;
    const sideNav = navState.sideNav;

    if (initialError) {
      return (
        <div className="startup-screen">
          <div className="startup-error-dialog">
            <div className="nav-brand">convocate</div>
            <div className="error">
              Unable to connect to services: {initialError}.
              Verify that the Router API is running and GitHub OAuth is configured.
            </div>
            <button onClick={() => window.location.reload()}>Retry</button>
          </div>
        </div>
      );
    }

    const sideNavItems = authenticated ? SIDE_NAV_ITEMS[topNav] : [];

    const handleTopNav = (id: string) => {
      if (!authenticated && id !== "dashboard") return;
      const nav = id as TopNav;
      navState.topNav = nav;
      navState.sideNav = DEFAULT_SIDE_NAV[nav];
      rerenderApp();
    };

    const handleSideNav = (id: string) => {
      navState.sideNav = id;
      rerenderApp();
    };

    const { projects, hosts, components } = this.props;

    let content: unknown;
    switch (topNav) {
      case "dashboard":
        content = <Dashboard authenticated={authenticated} projects={projects} hosts={hosts} components={components} />;
        break;
      case "projects":
        content = authenticated
          ? <Projects activeSideNav={sideNav} />
          : <Dashboard authenticated={false} projects={[]} hosts={[]} components={components} />;
        break;
      case "agents":
        content = authenticated
          ? <Agents activeSideNav={sideNav} />
          : <Dashboard authenticated={false} projects={[]} hosts={[]} components={components} />;
        break;
      case "dev-envs":
        content = authenticated
          ? <DevEnvs activeSideNav={sideNav} />
          : <Dashboard authenticated={false} projects={[]} hosts={[]} components={components} />;
        break;
      case "console":
        content = authenticated
          ? <Console activeSideNav={sideNav} />
          : <Dashboard authenticated={false} projects={[]} hosts={[]} components={components} />;
        break;
    }

    return (
      <Layout
        topNav={topNav}
        sideNav={sideNav}
        sideNavItems={sideNavItems}
        onTopNav={handleTopNav}
        onSideNav={handleSideNav}
        user={user}
        authenticated={authenticated}
      >
        {content}
      </Layout>
    );
  }
}

// Global nav state — persists across re-renders since specifyjs creates
// new component instances on each render.
const navState = { topNav: "dashboard" as TopNav, sideNav: "" };
let appRoot: { render(el: unknown): void } | null = null;
let appProps: AppProps | null = null;

export function setAppRoot(root: { render(el: unknown): void }) {
  appRoot = root;
}

function rerenderApp() {
  if (!appRoot || !appProps) return;
  appRoot.render(
    <App
      initialAuthenticated={appProps.initialAuthenticated}
      initialUser={appProps.initialUser}
      initialError={appProps.initialError}
    />
  );
}
