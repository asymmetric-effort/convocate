import { Component } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { CreateProjectResponse } from "../api/client";

interface CreateProjectProps {
  onDone: () => void;
}

interface CreateProjectState {
  repository: string;
  sshPrivateKey: string;
  githubPAT: string;
  error: string;
  result: CreateProjectResponse | null;
  submitting: boolean;
}

export class CreateProject extends Component<CreateProjectProps, CreateProjectState> {
  state: CreateProjectState = {
    repository: "",
    sshPrivateKey: "",
    githubPAT: "",
    error: "",
    result: null,
    submitting: false,
  };

  handleSubmit = async () => {
    const { repository, sshPrivateKey, githubPAT } = this.state;
    if (!repository || !sshPrivateKey || !githubPAT) {
      this.setState({ error: "All fields are required." });
      return;
    }
    this.setState({ submitting: true, error: "" });
    try {
      const resp = await api.createProject({
        repository,
        ssh_private_key: sshPrivateKey,
        github_pat: githubPAT,
      });
      this.setState({ result: resp });
    } catch (err: unknown) {
      this.setState({ error: err instanceof Error ? err.message : "Unknown error" });
    } finally {
      this.setState({ submitting: false });
    }
  };

  render() {
    const { repository, sshPrivateKey, githubPAT, error, result, submitting } = this.state;
    const { onDone } = this.props;

    if (result) {
      return (
        <div className="create-project-result">
          <h1>Project Created</h1>
          <p>Repository: {result.repository}</p>
          <p>Project ID: {result.project_id}</p>

          <div className="token-display">
            <h2>CONVOCATE_API_TOKEN</h2>
            <p className="warning">Copy this token now. It will not be shown again.</p>
            <code className="token">{result.api_token}</code>
          </div>

          <div className="setup-instructions">
            <h2>GitHub Setup</h2>
            <p>Add these to your repository's Actions secrets and variables:</p>
            <table>
              <thead>
                <tr>
                  <th>Type</th>
                  <th>Name</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>Variable</td>
                  <td>CONVOCATE_BOT_ACCOUNT</td>
                  <td>(your bot account name)</td>
                </tr>
                <tr>
                  <td>Variable</td>
                  <td>CONVOCATE_ROUTER_URL</td>
                  <td>(your Router API URL)</td>
                </tr>
                <tr>
                  <td>Secret</td>
                  <td>CONVOCATE_API_TOKEN</td>
                  <td>{result.api_token}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <button onClick={onDone}>Done</button>
        </div>
      );
    }

    return (
      <div className="create-project">
        <h1>Create Project</h1>

        {error ? <div className="error">{error}</div> : null}

        <div className="form-group">
          <label>Repository (org/repo)</label>
          <input
            type="text"
            value={repository}
            placeholder="org/repo"
            onInput={(e: Event) => this.setState({ repository: (e.target as HTMLInputElement).value })}
          />
        </div>

        <div className="form-group">
          <label>SSH Private Key (ed25519)</label>
          <textarea
            value={sshPrivateKey}
            placeholder="-----BEGIN OPENSSH PRIVATE KEY-----\n..."
            rows={6}
            onInput={(e: Event) => this.setState({ sshPrivateKey: (e.target as HTMLTextAreaElement).value })}
          />
        </div>

        <div className="form-group">
          <label>GitHub PAT (fine-grained, repo-scoped)</label>
          <input
            type="password"
            value={githubPAT}
            placeholder="ghp_..."
            onInput={(e: Event) => this.setState({ githubPAT: (e.target as HTMLInputElement).value })}
          />
        </div>

        <button onClick={this.handleSubmit} disabled={submitting}>
          {submitting ? "Creating..." : "Create Project"}
        </button>
      </div>
    );
  }
}
