import { useState, useEffect } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { ProjectInfo } from "../api/client";

export function AdHocSubmit() {
  const [projects, setProjects] = useState<ProjectInfo[]>([]);
  const [selectedProject, setSelectedProject] = useState("");
  const [prompt, setPrompt] = useState("");
  const [error, setError] = useState("");
  const [result, setResult] = useState<{ job_id: string } | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    api.listProjects()
      .then((data) => {
        setProjects(data);
        if (data.length > 0) {
          setSelectedProject(data[0].project_id);
        }
      })
      .catch((err: Error) => setError(err.message));
  }, []);

  const handleSubmit = async () => {
    if (!selectedProject || !prompt) {
      setError("Select a project and enter a prompt.");
      return;
    }
    setSubmitting(true);
    setError("");
    setResult(null);
    try {
      const resp = await api.submitAdHoc({
        project_id: selectedProject,
        prompt,
      });
      setResult(resp);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="adhoc-submit">
      <h1>Ad-hoc Job Submission</h1>

      {error ? <div className="error">{error}</div> : null}
      {result ? <div className="success">Job submitted: {result.job_id}</div> : null}

      <div className="form-group">
        <label>Project</label>
        <select
          value={selectedProject}
          onChange={(e: Event) => setSelectedProject((e.target as HTMLSelectElement).value)}
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
          onInput={(e: Event) => setPrompt((e.target as HTMLTextAreaElement).value)}
        />
      </div>

      <button onClick={handleSubmit} disabled={submitting || projects.length === 0}>
        {submitting ? "Submitting..." : "Submit"}
      </button>
    </div>
  );
}
