import { Component } from "@asymmetric-effort/specifyjs";

interface SideNavItem {
  id: string;
  label: string;
}

interface SideNavProps {
  items: SideNavItem[];
  active: string;
  onNavigate: (id: string) => void;
}

export class SideNav extends Component<SideNavProps, Record<string, never>> {
  state = {};

  render() {
    const { items, active, onNavigate } = this.props;
    if (items.length === 0) {
      return null;
    }
    return (
      <aside className="side-nav">
        {items.map((item) => (
          <button
            key={item.id}
            className={`side-nav-link${active === item.id ? " active" : ""}`}
            onClick={() => onNavigate(item.id)}
          >
            {item.label}
          </button>
        ))}
      </aside>
    );
  }
}
