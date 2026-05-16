import { useState } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { CreateProjectResponse } from "../api/client";

interface CreateProjectProps {
  onDone: () => void;
}

export function CreateProject({ onDone }: CreateProjectProps) {
  const [repository, setRepository] = useState("");
  const [sshPrivateKey, setSSHPrivateKey] = useState("");
  const [githubPAT, setGithubPAT] = useState("");
  const [error, setError] = useState("");
  const [result, setResult] = useState<CreateProjectResponse | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!repository || !sshPrivateKey || !githubPAT) {
      setError("All fields are required.");
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      const resp = await api.createProject({
        repository,
        ssh_private_key: sshPrivateKey,
        github_pat: githubPAT,
      });
      setResult(resp);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setSubmitting(false);
    }
  };

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
          onInput={(e: Event) => setRepository((e.target as HTMLInputElement).value)}
        />
      </div>

      <div className="form-group">
        <label>SSH Private Key (ed25519)</label>
        <textarea
          value={sshPrivateKey}
          placeholder="-----BEGIN OPENSSH PRIVATE KEY-----\n..."
          rows={6}
          onInput={(e: Event) => setSSHPrivateKey((e.target as HTMLTextAreaElement).value)}
        />
      </div>

      <div className="form-group">
        <label>GitHub PAT (fine-grained, repo-scoped)</label>
        <input
          type="password"
          value={githubPAT}
          placeholder="ghp_..."
          onInput={(e: Event) => setGithubPAT((e.target as HTMLInputElement).value)}
        />
      </div>

      <button onClick={handleSubmit} disabled={submitting}>
        {submitting ? "Creating..." : "Create Project"}
      </button>
    </div>
  );
}
