import { Component } from "@asymmetric-effort/specifyjs";
import { Card } from "../components/Card";
import { api } from "../api/client";
import type { ProjectInfo, HostHealthInfo } from "../api/client";

interface ComponentStatus {
  name: string;
  status: "running" | "stopped" | "unknown";
  uptime: string;
  checkedAt: string;
}

interface DashboardState {
  projects: ProjectInfo[];
  hosts: HostHealthInfo[];
  components: ComponentStatus[];
  error: string;
}

function formatUptime(startTime: number): string {
  const now = Date.now();
  const diff = Math.floor((now - startTime) / 1000);
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ${diff % 60}s`;
  const hours = Math.floor(diff / 3600);
  const minutes = Math.floor((diff % 3600) / 60);
  if (hours < 24) return `${hours}h ${minutes}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

function timeAgo(isoString: string): string {
  if (!isoString) return "never";
  const diff = Math.floor((Date.now() - new Date(isoString).getTime()) / 1000);
  if (diff < 5) return "just now";
  if (diff < 60) return `${diff}s ago`;
  return `${Math.floor(diff / 60)}m ago`;
}

export class Dashboard extends Component<Record<string, never>, DashboardState> {
  state: DashboardState = {
    projects: [],
    hosts: [],
    components: [
      { name: "router", status: "unknown", uptime: "—", checkedAt: "" },
      { name: "redis", status: "unknown", uptime: "—", checkedAt: "" },
      { name: "openbao", status: "unknown", uptime: "—", checkedAt: "" },
      { name: "dispatch", status: "unknown", uptime: "—", checkedAt: "" },
      { name: "secrets-broker", status: "unknown", uptime: "—", checkedAt: "" },
    ],
    error: "",
  };

  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private startTimes: Map<string, number> = new Map();

  componentDidMount() {
    this.fetchAll();
    this.pollTimer = setInterval(() => this.fetchComponentStatus(), 15000);
  }

  componentWillUnmount() {
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  private fetchAll() {
    api.listProjects()
      .then((projects) => this.setState({ projects }))
      .catch((err: Error) => this.setState({ error: err.message }));
    api.listHosts()
      .then((hosts) => this.setState({ hosts }))
      .catch((err: Error) => this.setState({ error: err.message }));
    this.fetchComponentStatus();
  }

  private async fetchComponentStatus() {
    const componentNames = [
      "router", "redis", "openbao", "dispatch", "secrets-broker"
    ];
    const now = new Date().toISOString();
    const components: ComponentStatus[] = [];

    for (const name of componentNames) {
      let status: "running" | "stopped" | "unknown" = "unknown";

      if (name === "router") {
        // Router is running if we can reach the health endpoint.
        try {
          await api.health();
          status = "running";
        } catch {
          status = "stopped";
        }
      } else {
        // For other components, infer from the health data.
        // If we got a health response, the stack is up.
        try {
          await api.health();
          status = "running";
        } catch {
          status = "stopped";
        }
      }

      if (status === "running" && !this.startTimes.has(name)) {
        this.startTimes.set(name, Date.now());
      } else if (status !== "running") {
        this.startTimes.delete(name);
      }

      const startTime = this.startTimes.get(name);
      components.push({
        name,
        status,
        uptime: startTime ? formatUptime(startTime) : "—",
        checkedAt: now,
      });
    }

    this.setState({ components });
  }

  render() {
    const { projects, hosts, components, error } = this.state;

    return (
      <div className="dashboard">
        <h1>Dashboard</h1>

        {error ? <div className="error">{error}</div> : null}

        <Card title="Projects">
          {projects.length === 0 ? (
            <p>No projects configured.</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Repository</th>
                  <th>Container State</th>
                  <th>Active Jobs</th>
                </tr>
              </thead>
              <tbody>
                {projects.map((project) => (
                  <tr key={project.project_id}>
                    <td>{project.repository}</td>
                    <td>{project.container_state}</td>
                    <td>{String(project.active_jobs)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>

        <Card title="Agents">
          {hosts.length === 0 ? (
            <p>No agent hosts registered.</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Host ID</th>
                  <th>Containers</th>
                  <th>CPU %</th>
                  <th>Memory %</th>
                  <th>Healthy</th>
                </tr>
              </thead>
              <tbody>
                {hosts.map((host) => (
                  <tr key={host.host_id}>
                    <td>{host.host_id}</td>
                    <td>{String(host.container_count)}</td>
                    <td>{host.cpu_percent.toFixed(1)}</td>
                    <td>{host.memory_percent.toFixed(1)}</td>
                    <td>{host.healthy ? "Yes" : "No"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>

        <Card title="Convocate Components">
          <p className="text-muted">
            Auto-refreshes every 15s · Last checked: {
              components.length > 0 ? timeAgo(components[0].checkedAt) : "—"
            }
          </p>
          <table>
            <thead>
              <tr>
                <th>Component</th>
                <th>Status</th>
                <th>Uptime</th>
              </tr>
            </thead>
            <tbody>
              {components.map((component) => (
                <tr key={component.name}>
                  <td>{component.name}</td>
                  <td className={`status-${component.status}`}>
                    {component.status}
                  </td>
                  <td>{component.uptime}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
    );
  }
}
