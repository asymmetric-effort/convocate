import { Component } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { ProjectInfo } from "../api/client";

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
