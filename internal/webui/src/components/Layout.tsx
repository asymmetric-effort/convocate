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

  return (
    <div className="app">
      <nav className="nav">
        <div className="nav-brand">convocate</div>
        <div className="nav-links">
          {navItems.map((item) => (
            <button
              key={item.id}
              className={`nav-link ${currentPage === item.id ? "active" : ""}`}
              onClick={() => onNavigate(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
      </nav>
      <main className="main">{children}</main>
    </div>
  );
}
