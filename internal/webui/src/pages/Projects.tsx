import { Component } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { ProjectInfo } from "../api/client";

interface ProjectsProps {
  activeSideNav: string;
}

// ---- List Projects ----

interface ListProjectsState {
  projects: ProjectInfo[];
  error: string;
}

export class ListProjects extends Component<Record<string, never>, ListProjectsState> {
  state: ListProjectsState = { projects: [], error: "" };

  componentDidMount() {
    api.listProjects()
      .then((projects) => this.setState({ projects }))
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  render() {
    const { projects, error } = this.state;
    return (
      <div>
        <h1>Projects</h1>
        {error ? <div className="error">{error}</div> : null}
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
      </div>
    );
  }
}

// ---- Create Project ----

function validateRepository(value: string): string {
  if (!value.trim()) return "Repository is required.";
  // Must be git@github.com:org/repo.git or org/repo format.
  const sshPattern = /^git@[\w.-]+:[\w.-]+\/[\w.-]+(\.git)?$/;
  const shortPattern = /^[\w.-]+\/[\w.-]+$/;
  if (!sshPattern.test(value) && !shortPattern.test(value)) {
    return "Repository must be in format 'org/repo' or 'git@github.com:org/repo.git'.";
  }
  return "";
}

function validateSSHKey(value: string): string {
  if (!value.trim()) return "SSH private key is required.";
  if (!value.includes("BEGIN") || !value.includes("PRIVATE KEY")) {
    return "SSH key must be a valid PEM-encoded private key.";
  }
  return "";
}

function validatePAT(value: string): string {
  if (!value.trim()) return "GitHub PAT is required.";
  return "";
}

export class CreateProject extends Component<Record<string, never>, Record<string, never>> {
  state = {};

  handleSubmit = () => {
    const repoInput = document.getElementById("cp-repo") as HTMLInputElement | null;
    const keyInput = document.getElementById("cp-key") as HTMLTextAreaElement | null;
    const patInput = document.getElementById("cp-pat") as HTMLInputElement | null;
    const errorDiv = document.getElementById("cp-error");
    const btn = document.getElementById("cp-submit") as HTMLButtonElement | null;

    if (!repoInput || !keyInput || !patInput || !errorDiv || !btn) return;

    const repo = repoInput.value.trim();
    const key = keyInput.value.trim();
    const pat = patInput.value.trim();

    // Validate.
    const repoErr = validateRepository(repo);
    if (repoErr) { errorDiv.textContent = repoErr; errorDiv.style.display = "block"; return; }
    const keyErr = validateSSHKey(key);
    if (keyErr) { errorDiv.textContent = keyErr; errorDiv.style.display = "block"; return; }
    const patErr = validatePAT(pat);
    if (patErr) { errorDiv.textContent = patErr; errorDiv.style.display = "block"; return; }

    errorDiv.style.display = "none";
    btn.disabled = true;
    btn.textContent = "Creating...";

    api.createProject({
      repository: repo,
      ssh_private_key: key,
      github_pat: pat,
    }).then((resp) => {
      // Show result in the same container.
      const container = document.getElementById("cp-form");
      if (container) {
        container.innerHTML = `
          <h1>Project Created</h1>
          <p>Repository: ${resp.repository}</p>
          <p>Project ID: ${resp.project_id}</p>
          <div class="token-display">
            <h2>CONVOCATE_API_TOKEN</h2>
            <p class="warning">Copy this token now. It will not be shown again.</p>
            <code class="token">${resp.api_token}</code>
          </div>
          <div class="setup-instructions">
            <h2>GitHub Setup</h2>
            <p>Add to your repository's Actions secrets and variables:</p>
            <table>
              <thead><tr><th>Type</th><th>Name</th><th>Value</th></tr></thead>
              <tbody>
                <tr><td>Variable</td><td>CONVOCATE_BOT_ACCOUNT</td><td>(your bot account)</td></tr>
                <tr><td>Variable</td><td>CONVOCATE_ROUTER_URL</td><td>(Router API URL)</td></tr>
                <tr><td>Secret</td><td>CONVOCATE_API_TOKEN</td><td>${resp.api_token}</td></tr>
              </tbody>
            </table>
          </div>
        `;
      }
    }).catch((err: unknown) => {
      errorDiv.textContent = err instanceof Error ? err.message : "Unknown error";
      errorDiv.style.display = "block";
      btn.disabled = false;
      btn.textContent = "Create Project";
    });
  };

  render() {
    return (
      <div id="cp-form" className="create-project">
        <h1>Create Project</h1>

        <div id="cp-error" className="error" style={{ display: "none" }}></div>

        <div className="form-group">
          <label>Repository (org/repo or git@github.com:org/repo.git)</label>
          <input id="cp-repo" type="text" placeholder="org/repo" />
        </div>

        <div className="form-group">
          <label>SSH Private Key (ed25519)</label>
          <textarea id="cp-key" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;..." rows={6}></textarea>
        </div>

        <div className="form-group">
          <label>GitHub PAT (fine-grained, repo-scoped)</label>
          <input id="cp-pat" type="password" placeholder="ghp_..." />
        </div>

        <button id="cp-submit" onClick={this.handleSubmit}>Create Project</button>
      </div>
    );
  }
}

// ---- Configure Project ----

interface ConfigureProjectState {
  projects: ProjectInfo[];
  selectedId: string;
  error: string;
}

export class ConfigureProject extends Component<Record<string, never>, ConfigureProjectState> {
  state: ConfigureProjectState = { projects: [], selectedId: "", error: "" };

  componentDidMount() {
    api.listProjects()
      .then((projects) => this.setState({
        projects,
        selectedId: projects.length > 0 ? projects[0].project_id : "",
      }))
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  render() {
    const { projects, selectedId, error } = this.state;
    const selected = projects.find((p) => p.project_id === selectedId);

    return (
      <div>
        <h1>Configure Project</h1>
        {error ? <div className="error">{error}</div> : null}

        <div className="form-group">
          <label>Project</label>
          <select
            value={selectedId}
            onChange={(e: Event) => this.setState({ selectedId: (e.target as HTMLSelectElement).value })}
          >
            {projects.length === 0 ? (
              <option value="">No projects available</option>
            ) : null}
            {projects.map((p) => (
              <option key={p.project_id} value={p.project_id}>{p.repository}</option>
            ))}
          </select>
        </div>

        {selected ? (
          <table>
            <tbody>
              <tr><th>Project ID</th><td>{selected.project_id}</td></tr>
              <tr><th>Repository</th><td>{selected.repository}</td></tr>
              <tr><th>Host ID</th><td>{selected.host_id}</td></tr>
              <tr><th>Container ID</th><td>{selected.container_id}</td></tr>
              <tr><th>Container State</th><td>{selected.container_state}</td></tr>
              <tr><th>Container Image</th><td>{selected.container_image}</td></tr>
              <tr><th>Active Jobs</th><td>{String(selected.active_jobs)}</td></tr>
              <tr><th>Created At</th><td>{selected.created_at}</td></tr>
            </tbody>
          </table>
        ) : null}
      </div>
    );
  }
}

// ---- Delete Project ----

interface DeleteProjectState {
  projects: ProjectInfo[];
  selectedId: string;
  forceTerminate: boolean;
  error: string;
  success: string;
  submitting: boolean;
}

export class DeleteProject extends Component<Record<string, never>, DeleteProjectState> {
  state: DeleteProjectState = {
    projects: [],
    selectedId: "",
    forceTerminate: false,
    error: "",
    success: "",
    submitting: false,
  };

  componentDidMount() {
    api.listProjects()
      .then((projects) => this.setState({
        projects,
        selectedId: projects.length > 0 ? projects[0].project_id : "",
      }))
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  handleDelete = async () => {
    const { selectedId, forceTerminate } = this.state;
    if (!selectedId) {
      this.setState({ error: "Select a project." });
      return;
    }
    this.setState({ submitting: true, error: "", success: "" });
    try {
      await api.deleteProject({ project_id: selectedId, force_terminate: forceTerminate });
      this.setState({ success: "Project deleted.", selectedId: "" });
      const projects = await api.listProjects();
      this.setState({ projects, selectedId: projects.length > 0 ? projects[0].project_id : "" });
    } catch (err: unknown) {
      this.setState({ error: err instanceof Error ? err.message : "Unknown error" });
    } finally {
      this.setState({ submitting: false });
    }
  };

  render() {
    const { projects, selectedId, forceTerminate, error, success, submitting } = this.state;

    return (
      <div>
        <h1>Delete Project</h1>
        {error ? <div className="error">{error}</div> : null}
        {success ? <div className="success">{success}</div> : null}

        <div className="form-group">
          <label>Project</label>
          <select
            value={selectedId}
            onChange={(e: Event) => this.setState({ selectedId: (e.target as HTMLSelectElement).value })}
          >
            {projects.length === 0 ? (
              <option value="">No projects available</option>
            ) : null}
            {projects.map((p) => (
              <option key={p.project_id} value={p.project_id}>{p.repository}</option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label>
            <input
              type="checkbox"
              checked={forceTerminate}
              onChange={(e: Event) => this.setState({ forceTerminate: (e.target as HTMLInputElement).checked })}
            />
            {" "}Force terminate (stop running jobs)
          </label>
        </div>

        <button onClick={this.handleDelete} disabled={submitting || !selectedId}>
          {submitting ? "Deleting..." : "Delete Project"}
        </button>
      </div>
    );
  }
}

// ---- Container Actions (Start / Stop / Restart) ----

type ContainerAction = "start" | "stop" | "restart";

interface ContainerActionProps {
  action: ContainerAction;
}

interface ContainerActionState {
  projects: ProjectInfo[];
  selectedId: string;
  error: string;
  success: string;
  submitting: boolean;
}

const ACTION_LABELS: Record<ContainerAction, string> = {
  start: "Start",
  stop: "Stop",
  restart: "Restart",
};

export class ContainerActionPage extends Component<ContainerActionProps, ContainerActionState> {
  state: ContainerActionState = {
    projects: [],
    selectedId: "",
    error: "",
    success: "",
    submitting: false,
  };

  componentDidMount() {
    api.listProjects()
      .then((projects) => this.setState({
        projects,
        selectedId: projects.length > 0 ? projects[0].project_id : "",
      }))
      .catch((err: Error) => this.setState({ error: err.message }));
  }

  handleAction = async () => {
    const { selectedId } = this.state;
    const { action } = this.props;
    if (!selectedId) {
      this.setState({ error: "Select a project." });
      return;
    }
    this.setState({ submitting: true, error: "", success: "" });
    try {
      await api.upgradeContainer(selectedId);
      this.setState({ success: `${ACTION_LABELS[action]} triggered for project.` });
    } catch (err: unknown) {
      this.setState({ error: err instanceof Error ? err.message : "Unknown error" });
    } finally {
      this.setState({ submitting: false });
    }
  };

  render() {
    const { projects, selectedId, error, success, submitting } = this.state;
    const { action } = this.props;
    const label = ACTION_LABELS[action];

    return (
      <div>
        <h1>{label} Project</h1>
        {error ? <div className="error">{error}</div> : null}
        {success ? <div className="success">{success}</div> : null}

        <div className="form-group">
          <label>Project</label>
          <select
            value={selectedId}
            onChange={(e: Event) => this.setState({ selectedId: (e.target as HTMLSelectElement).value })}
          >
            {projects.length === 0 ? (
              <option value="">No projects available</option>
            ) : null}
            {projects.map((p) => (
              <option key={p.project_id} value={p.project_id}>{p.repository}</option>
            ))}
          </select>
        </div>

        <button onClick={this.handleAction} disabled={submitting || !selectedId}>
          {submitting ? `${label}ing...` : `${label} Project`}
        </button>
      </div>
    );
  }
}

// ---- Projects top-level router ----

export class Projects extends Component<ProjectsProps, Record<string, never>> {
  state = {};

  render() {
    const { activeSideNav } = this.props;

    switch (activeSideNav) {
      case "list-projects":
        return <ListProjects />;
      case "create-project":
        return <CreateProject />;
      case "configure-project":
        return <ConfigureProject />;
      case "delete-project":
        return <DeleteProject />;
      case "start-project":
        return <ContainerActionPage action="start" />;
      case "stop-project":
        return <ContainerActionPage action="stop" />;
      case "restart-project":
        return <ContainerActionPage action="restart" />;
      default:
        return <ListProjects />;
    }
  }
}
