import { useState } from "@asymmetric-effort/specifyjs";
import { Layout } from "./components/Layout";
import { Dashboard } from "./pages/Dashboard";
import { CreateProject } from "./pages/CreateProject";
import { ClusterAuth } from "./pages/ClusterAuth";
import { AdHocSubmit } from "./pages/AdHocSubmit";

type Page = "dashboard" | "create-project" | "cluster-auth" | "adhoc";

export function App() {
  const [page, setPage] = useState<Page>("dashboard");

  let content;
  switch (page) {
    case "dashboard":
      content = <Dashboard />;
      break;
    case "create-project":
      content = <CreateProject onDone={() => setPage("dashboard")} />;
      break;
    case "cluster-auth":
      content = <ClusterAuth />;
      break;
    case "adhoc":
      content = <AdHocSubmit />;
      break;
  }

  return <Layout currentPage={page} onNavigate={setPage as (page: string) => void}>{content}</Layout>;
}
