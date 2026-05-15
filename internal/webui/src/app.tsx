import { createElement, useState } from "@asymmetric-effort/specifyjs";
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
      content = createElement(Dashboard, null);
      break;
    case "create-project":
      content = createElement(CreateProject, { onDone: () => setPage("dashboard") });
      break;
    case "cluster-auth":
      content = createElement(ClusterAuth, null);
      break;
    case "adhoc":
      content = createElement(AdHocSubmit, null);
      break;
  }

  return createElement(Layout, { currentPage: page, onNavigate: setPage }, content);
}
