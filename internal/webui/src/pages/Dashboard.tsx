import { Component } from "@asymmetric-effort/specifyjs";
import { Card } from "../components/Card";
import type { ProjectInfo, HostHealthInfo } from "../api/client";

interface ComponentStatus {
  name: string;
  status: "running" | "stopped" | "unknown";
}

interface DashboardProps {
  authenticated?: boolean;
  projects: ProjectInfo[];
  hosts: HostHealthInfo[];
  components: ComponentStatus[];
}

export class Dashboard extends Component<DashboardProps, Record<string, never>> {
  state = {};

  render() {
    const { authenticated, projects, hosts, components } = this.props;

    return (
      <div className="dashboard">
        <h1>Dashboard</h1>

        {authenticated !== false ? (
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
        ) : null}

        {authenticated !== false ? (
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
        ) : null}

        <Card title="Convocate Components">
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
                  <td className={`status-${component.status}`}>
                    {component.status}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
    );
  }
}
