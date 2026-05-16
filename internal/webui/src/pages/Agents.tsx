import { Component } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { HostHealthInfo } from "../api/client";

interface AgentsProps {
  activeSideNav: string;
}

interface ListAgentsState {
  hosts: HostHealthInfo[];
  error: string;
}

class ListAgents extends Component<Record<string, never>, ListAgentsState> {
  state: ListAgentsState = { hosts: [], error: "" };
  private pollTimer: ReturnType<typeof setInterval> | null = null;

  componentDidMount() {
    this.fetchHosts();
    this.pollTimer = setInterval(() => this.fetchHosts(), 15000);
  }

  componentWillUnmount() {
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  private fetchHosts() {
    api.listHosts()
      .then((hosts) => this.setState({ hosts }))
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  render() {
    const { hosts, error } = this.state;
    return (
      <div>
        <h1>Agent Hosts</h1>
        <p className="text-muted">Auto-refreshes every 15s</p>
        {error ? <div className="error">{error}</div> : null}
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
                <th>Last Heartbeat</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {hosts.map((host) => (
                <tr key={host.host_id}>
                  <td>{host.host_id}</td>
                  <td>{String(host.container_count)}</td>
                  <td>{host.cpu_percent.toFixed(1)}</td>
                  <td>{host.memory_percent.toFixed(1)}</td>
                  <td>{host.last_heartbeat_unix
                    ? new Date(host.last_heartbeat_unix * 1000).toLocaleTimeString()
                    : "—"}</td>
                  <td className={host.healthy ? "status-running" : "status-stopped"}>
                    {host.healthy ? "healthy" : "unhealthy"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    );
  }
}

class RegisterAgent extends Component<Record<string, never>, Record<string, never>> {
  state = {};
  render() {
    return (
      <div>
        <h1>Register Agent Host</h1>
        <p>Agent host registration is handled via <code>convocate-cli host issue-cert</code> on the control plane.</p>
        <p>See deployment documentation for instructions.</p>
      </div>
    );
  }
}

export class Agents extends Component<AgentsProps, Record<string, never>> {
  state = {};

  render() {
    const { activeSideNav } = this.props;
    switch (activeSideNav) {
      case "list-agents":
        return <ListAgents />;
      case "register-agent":
        return <RegisterAgent />;
      default:
        return <ListAgents />;
    }
  }
}
