import { Component } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { ProjectInfo, HostHealthInfo } from "../api/client";

interface ComponentStatus {
  name: string;
  status: "running" | "stopped";
}

interface DashboardState {
  projects: ProjectInfo[];
  hosts: HostHealthInfo[];
  components: ComponentStatus[];
  error: string;
}

const CONVOCATE_COMPONENTS: ComponentStatus[] = [
  { name: "router", status: "running" },
  { name: "redis", status: "running" },
  { name: "openbao", status: "running" },
  { name: "dispatch", status: "running" },
  { name: "secrets-broker", status: "running" },
];

export class Dashboard extends Component<Record<string, never>, DashboardState> {
  state: DashboardState = {
    projects: [],
    hosts: [],
    components: CONVOCATE_COMPONENTS,
    error: "",
  };

  componentDidMount() {
    api.listProjects()
      .then((projects) => this.setState({ projects }))
      .catch((err: Error) => this.setState({ error: err.message }));
    api.listHosts()
      .then((hosts) => this.setState({ hosts }))
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  render() {
    const { projects, hosts, components, error } = this.state;

    return (
      <div className="dashboard">
        <h1>Dashboard</h1>

        {error ? <div className="error">{error}</div> : null}

        <section>
          <h2>Projects</h2>
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
        </section>

        <section>
          <h2>Agents</h2>
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
        </section>

        <section>
          <h2>Convocate Components</h2>
          <table>
            <thead>
              <tr>
                <th>Component</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {components.map((component) => (
                <tr key={component.name}>
                  <td>{component.name}</td>
                  <td className={`status-${component.status}`}>{component.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      </div>
    );
  }
}
