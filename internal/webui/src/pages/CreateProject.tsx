import { createElement, useState } from "@asymmetric-effort/specifyjs";
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
    return createElement("div", { className: "create-project-result" },
      createElement("h1", null, "Project Created"),
      createElement("p", null, `Repository: ${result.repository}`),
      createElement("p", null, `Project ID: ${result.project_id}`),

      createElement("div", { className: "token-display" },
        createElement("h2", null, "CONVOCATE_API_TOKEN"),
        createElement("p", { className: "warning" },
          "Copy this token now. It will not be shown again."
        ),
        createElement("code", { className: "token" }, result.api_token),
      ),

      createElement("div", { className: "setup-instructions" },
        createElement("h2", null, "GitHub Setup"),
        createElement("p", null, "Add these to your repository's Actions secrets and variables:"),
        createElement("table", null,
          createElement("thead", null,
            createElement("tr", null,
              createElement("th", null, "Type"),
              createElement("th", null, "Name"),
              createElement("th", null, "Value"),
            )
          ),
          createElement("tbody", null,
            createElement("tr", null,
              createElement("td", null, "Variable"),
              createElement("td", null, "CONVOCATE_BOT_ACCOUNT"),
              createElement("td", null, "(your bot account name)"),
            ),
            createElement("tr", null,
              createElement("td", null, "Variable"),
              createElement("td", null, "CONVOCATE_ROUTER_URL"),
              createElement("td", null, "(your Router API URL)"),
            ),
            createElement("tr", null,
              createElement("td", null, "Secret"),
              createElement("td", null, "CONVOCATE_API_TOKEN"),
              createElement("td", null, result.api_token),
            ),
          )
        ),
      ),

      createElement("button", { onClick: onDone }, "Done"),
    );
  }

  return createElement("div", { className: "create-project" },
    createElement("h1", null, "Create Project"),

    error ? createElement("div", { className: "error" }, error) : null,

    createElement("div", { className: "form-group" },
      createElement("label", null, "Repository (org/repo)"),
      createElement("input", {
        type: "text",
        value: repository,
        placeholder: "org/repo",
        onInput: (e: Event) => setRepository((e.target as HTMLInputElement).value),
      }),
    ),

    createElement("div", { className: "form-group" },
      createElement("label", null, "SSH Private Key (ed25519)"),
      createElement("textarea", {
        value: sshPrivateKey,
        placeholder: "-----BEGIN OPENSSH PRIVATE KEY-----\n...",
        rows: 6,
        onInput: (e: Event) => setSSHPrivateKey((e.target as HTMLTextAreaElement).value),
      }),
    ),

    createElement("div", { className: "form-group" },
      createElement("label", null, "GitHub PAT (fine-grained, repo-scoped)"),
      createElement("input", {
        type: "password",
        value: githubPAT,
        placeholder: "ghp_...",
        onInput: (e: Event) => setGithubPAT((e.target as HTMLInputElement).value),
      }),
    ),

    createElement("button", {
      onClick: handleSubmit,
      disabled: submitting,
    }, submitting ? "Creating..." : "Create Project"),
  );
}
