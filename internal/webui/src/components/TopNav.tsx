import { Component } from "@asymmetric-effort/specifyjs";

interface TopNavItem {
  id: string;
  label: string;
}

interface TopNavProps {
  items: TopNavItem[];
  active: string;
  onNavigate: (id: string) => void;
}

export class TopNav extends Component<TopNavProps, Record<string, never>> {
  state = {};

  render() {
    const { items, active, onNavigate } = this.props;
    return (
      <nav className="top-nav">
        <div className="nav-brand">convocate</div>
        <div className="nav-links">
          {items.map((item) => (
            <button
              key={item.id}
              className={`nav-link${active === item.id ? " active" : ""}`}
              onClick={() => onNavigate(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
      </nav>
    );
  }
}
