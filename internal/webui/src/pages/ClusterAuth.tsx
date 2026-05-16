import { Component } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";

type AuthMode = "anthropic_api_key" | "claude_session";

interface ClusterAuthState {
  mode: AuthMode;
  apiKey: string;
  sessionToken: string;
  currentMode: string;
  error: string;
  success: string;
  submitting: boolean;
}

export class ClusterAuth extends Component<Record<string, never>, ClusterAuthState> {
  state: ClusterAuthState = {
    mode: "anthropic_api_key",
    apiKey: "",
    sessionToken: "",
    currentMode: "",
    error: "",
    success: "",
    submitting: false,
  };

  componentDidMount() {
    api.getClusterAuth()
      .then((resp) => this.setState({ currentMode: resp.mode || "none" }))
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  handleSubmit = async () => {
    const { mode, apiKey, sessionToken } = this.state;
    this.setState({ error: "", success: "", submitting: true });
    try {
      await api.setClusterAuth({
        mode,
        api_key: mode === "anthropic_api_key" ? apiKey : undefined,
        session_token: mode === "claude_session" ? sessionToken : undefined,
      });
      this.setState({ success: "Cluster authentication updated.", currentMode: mode });
    } catch (err: unknown) {
      this.setState({ error: err instanceof Error ? err.message : "Unknown error" });
    } finally {
      this.setState({ submitting: false });
    }
  };

  render() {
    const { mode, apiKey, sessionToken, currentMode, error, success, submitting } = this.state;

    return (
      <div className="cluster-auth">
        <h1>Cluster Authentication</h1>
        <p>Current mode: {currentMode || "loading..."}</p>

        {error ? <div className="error">{error}</div> : null}
        {success ? <div className="success">{success}</div> : null}

        <div className="form-group">
          <label>Authentication Mode</label>
          <select
            value={mode}
            onChange={(e: Event) => this.setState({ mode: (e.target as HTMLSelectElement).value as AuthMode })}
          >
            <option value="anthropic_api_key">Anthropic API Key</option>
            <option value="claude_session">Claude.ai Session</option>
          </select>
        </div>

        {mode === "anthropic_api_key" ? (
          <div className="form-group">
            <label>Anthropic API Key</label>
            <input
              type="password"
              value={apiKey}
              placeholder="sk-ant-api03-..."
              onInput={(e: Event) => this.setState({ apiKey: (e.target as HTMLInputElement).value })}
            />
          </div>
        ) : (
          <div className="form-group">
            <label>Claude.ai Session Token</label>
            <input
              type="password"
              value={sessionToken}
              placeholder="session token"
              onInput={(e: Event) => this.setState({ sessionToken: (e.target as HTMLInputElement).value })}
            />
          </div>
        )}

        <button onClick={this.handleSubmit} disabled={submitting}>
          {submitting ? "Updating..." : "Update Authentication"}
        </button>
      </div>
    );
  }
}
