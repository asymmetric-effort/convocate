import { createElement } from "@asymmetric-effort/specifyjs";

interface LayoutProps {
  currentPage: string;
  onNavigate: (page: string) => void;
  children?: unknown;
}

export function Layout({ currentPage, onNavigate, children }: LayoutProps) {
  const navItems = [
    { id: "dashboard", label: "Dashboard" },
    { id: "create-project", label: "Create Project" },
    { id: "cluster-auth", label: "Cluster Auth" },
    { id: "adhoc", label: "Ad-hoc Submit" },
  ];

  return createElement("div", { className: "app" },
    createElement("nav", { className: "nav" },
      createElement("div", { className: "nav-brand" }, "convocate"),
      createElement("div", { className: "nav-links" },
        ...navItems.map((item) =>
          createElement("button", {
            key: item.id,
            className: `nav-link ${currentPage === item.id ? "active" : ""}`,
            onClick: () => onNavigate(item.id),
          }, item.label)
        )
      )
    ),
    createElement("main", { className: "main" }, children)
  );
}
