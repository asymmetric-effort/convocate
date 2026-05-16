import { Component } from "@asymmetric-effort/specifyjs";
import { Layout } from "./components/Layout";
import { Dashboard } from "./pages/Dashboard";
import { CreateProject } from "./pages/CreateProject";
import { ClusterAuth } from "./pages/ClusterAuth";
import { AdHocSubmit } from "./pages/AdHocSubmit";

type Page = "dashboard" | "create-project" | "cluster-auth" | "adhoc";

interface AppState {
  page: Page;
}

export class App extends Component<Record<string, never>, AppState> {
  state: AppState = { page: "dashboard" };

  render() {
    const { page } = this.state;

    let content;
    switch (page) {
      case "dashboard":
        content = <Dashboard />;
        break;
      case "create-project":
        content = <CreateProject onDone={() => this.setState({ page: "dashboard" })} />;
        break;
      case "cluster-auth":
        content = <ClusterAuth />;
        break;
      case "adhoc":
        content = <AdHocSubmit />;
        break;
    }

    return (
      <Layout
        currentPage={page}
        onNavigate={(p: string) => this.setState({ page: p as Page })}
      >
        {content}
      </Layout>
    );
  }
}
