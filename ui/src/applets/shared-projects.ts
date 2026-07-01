/**
 * Shared Projects — unified project list from POST/GET /api/v1/projects.
 *
 * All applets (Code IDE, Project Board, Repo Manager) use this as
 * the canonical source of projects. Creating a project atomically
 * creates an IDE project, board, repo, and agent-container.
 */

export interface UnifiedProject {
  id: string;
  name: string;
  repoId: string;
  boardId: string;
  agentId: string;
}

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

/** Fetch all projects from the unified API. */
export async function fetchProjects(): Promise<UnifiedProject[]> {
  const res = await fetch("/api/v1/projects?limit=200", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch projects: ${res.status}`);
  const page = await res.json();
  return (page.items || []).map((p: any) => ({
    id: p.id,
    name: p.name,
    repoId: p.repoId || "",
    boardId: p.boardId || "",
    agentId: p.agentId || "",
  }));
}

/** Create a new project atomically (board + repo + agent). */
export async function createProject(name: string): Promise<UnifiedProject> {
  const res = await fetch("/api/v1/projects", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `Failed to create project: ${res.status}`);
  }
  const p = await res.json();
  return { id: p.id, name: p.name, repoId: p.repoId || "", boardId: p.boardId || "", agentId: p.agentId || "" };
}

/** Update a project's metadata. */
export async function updateProject(
  projectId: string,
  updates: { boardId?: string; repoId?: string; agentId?: string; name?: string },
): Promise<UnifiedProject> {
  const res = await fetch(`/api/v1/projects/${projectId}`, {
    method: "PATCH",
    headers: authHeaders(),
    body: JSON.stringify(updates),
  });
  if (!res.ok) throw new Error(`Failed to update project: ${res.status}`);
  const p = await res.json();
  return { id: p.id, name: p.name, repoId: p.repoId || "", boardId: p.boardId || "", agentId: p.agentId || "" };
}
