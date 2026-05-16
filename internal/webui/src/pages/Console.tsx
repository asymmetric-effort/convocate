import { Component } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { ProjectInfo } from "../api/client";

// ---- Ad-hoc Submit ----

interface AdHocSubmitState {
  projects: ProjectInfo[];
  selectedProject: string;
  prompt: string;
  error: string;
  result: { job_id: string } | null;
  submitting: boolean;
}

export class AdHocSubmit extends Component<Record<string, never>, AdHocSubmitState> {
  state: AdHocSubmitState = {
    projects: [],
    selectedProject: "",
    prompt: "",
    error: "",
    result: null,
    submitting: false,
  };

  componentDidMount() {
    api.listProjects()
      .then((data) => {
        this.setState({
          projects: data,
          selectedProject: data.length > 0 ? data[0].project_id : "",
        });
      })
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  handleSubmit = async () => {
    const { selectedProject, prompt } = this.state;
    if (!selectedProject || !prompt) {
      this.setState({ error: "Select a project and enter a prompt." });
      return;
    }
    this.setState({ submitting: true, error: "", result: null });
    try {
      const resp = await api.submitAdHoc({
        project_id: selectedProject,
        prompt,
      });
      this.setState({ result: resp });
    } catch (err: unknown) {
      this.setState({ error: err instanceof Error ? err.message : "Unknown error" });
    } finally {
      this.setState({ submitting: false });
    }
  };

  render() {
    const { projects, selectedProject, prompt, error, result, submitting } = this.state;

    return (
      <div className="adhoc-submit">
        <h1>Ad-hoc Job Submission</h1>

        {error ? <div className="error">{error}</div> : null}
        {result ? <div className="success">Job submitted: {result.job_id}</div> : null}

        <div className="form-group">
          <label>Project</label>
          <select
            value={selectedProject}
            onChange={(e: Event) => this.setState({ selectedProject: (e.target as HTMLSelectElement).value })}
          >
            {projects.length === 0 ? (
              <option value="">No projects available</option>
            ) : null}
            {projects.map((project) => (
              <option key={project.project_id} value={project.project_id}>
                {project.repository}
              </option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label>Prompt</label>
          <textarea
            value={prompt}
            placeholder="Describe what you want the agent to implement..."
            rows={8}
            onInput={(e: Event) => this.setState({ prompt: (e.target as HTMLTextAreaElement).value })}
          />
        </div>

        <button onClick={this.handleSubmit} disabled={submitting || projects.length === 0}>
          {submitting ? "Submitting..." : "Submit"}
        </button>
      </div>
    );
  }
}

// ---- Cluster Auth ----

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

// ---- Console top-level router ----

interface ConsoleProps {
  activeSideNav: string;
}

export class Console extends Component<ConsoleProps, Record<string, never>> {
  state = {};

  render() {
    const { activeSideNav } = this.props;

    switch (activeSideNav) {
      case "adhoc":
        return <AdHocSubmit />;
      case "cluster-auth":
        return <ClusterAuth />;
      default:
        return <AdHocSubmit />;
    }
  }
}
