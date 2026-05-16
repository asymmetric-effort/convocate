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

interface TopNavState {
  time: string;
}

export class TopNav extends Component<TopNavProps, TopNavState> {
  state: TopNavState = { time: new Date().toLocaleTimeString() };
  private timer: ReturnType<typeof setInterval> | null = null;

  componentDidMount() {
    this.timer = setInterval(() => {
      this.setState({ time: new Date().toLocaleTimeString() });
    }, 1000);
  }

  componentWillUnmount() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  render() {
    const { items, active, onNavigate } = this.props;
    const { time } = this.state;
    const date = new Date().toLocaleDateString(undefined, {
      weekday: "short", month: "short", day: "numeric",
    });

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
        <div className="nav-status">
          <span className="nav-status-user">operator</span>
          <span className="nav-status-time">{date} {time}</span>
        </div>
      </nav>
    );
  }
}
