import { Component } from "@asymmetric-effort/specifyjs";
import type { MeResponse } from "../api/client";

interface TopNavItem {
  id: string;
  label: string;
}

interface TopNavProps {
  items: TopNavItem[];
  active: string;
  onNavigate: (id: string) => void;
  user?: MeResponse | null;
  authenticated: boolean;
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
    const { items, active, onNavigate, user, authenticated } = this.props;
    const { time } = this.state;
    const date = new Date().toLocaleDateString(undefined, {
      weekday: "short", month: "short", day: "numeric",
    });

    const displayName = user?.username || (authenticated ? "operator" : "not signed in");

    return (
      <nav className="top-nav">
        <div className="nav-brand">convocate</div>
        <div className="nav-links">
          {items.map((item) => {
            const disabled = !authenticated && item.id !== "dashboard";
            return (
              <button
                key={item.id}
                className={`nav-link${active === item.id ? " active" : ""}${disabled ? " disabled" : ""}`}
                onClick={() => !disabled && onNavigate(item.id)}
                disabled={disabled}
              >
                {item.label}
              </button>
            );
          })}
        </div>
        <div className="nav-status">
          {authenticated ? (
            <span className="nav-status-user">{displayName}</span>
          ) : (
            <a href="/auth/login" className="nav-login">Sign in with GitHub</a>
          )}
          {user ? (
            <a href="/auth/logout" className="nav-logout">Logout</a>
          ) : null}
          <span className="nav-status-time">{date} {time}</span>
        </div>
      </nav>
    );
  }
}
