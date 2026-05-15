import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
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

  return createElement("div", { className: "adhoc-submit" },
    createElement("h1", null, "Ad-hoc Job Submission"),

    error ? createElement("div", { className: "error" }, error) : null,
    result
      ? createElement("div", { className: "success" },
          `Job submitted: ${result.job_id}`
        )
      : null,

    createElement("div", { className: "form-group" },
      createElement("label", null, "Project"),
      createElement("select", {
        value: selectedProject,
        onChange: (e: Event) => setSelectedProject((e.target as HTMLSelectElement).value),
      },
        projects.length === 0
          ? createElement("option", { value: "" }, "No projects available")
          : null,
        ...projects.map((project) =>
          createElement("option", {
            key: project.project_id,
            value: project.project_id,
          }, project.repository)
        ),
      ),
    ),

    createElement("div", { className: "form-group" },
      createElement("label", null, "Prompt"),
      createElement("textarea", {
        value: prompt,
        placeholder: "Describe what you want the agent to implement...",
        rows: 8,
        onInput: (e: Event) => setPrompt((e.target as HTMLTextAreaElement).value),
      }),
    ),

    createElement("button", {
      onClick: handleSubmit,
      disabled: submitting || projects.length === 0,
    }, submitting ? "Submitting..." : "Submit"),
  );
}
