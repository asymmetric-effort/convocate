/**
 * Shared Projects — unified project list from the IDE project API.
 *
 * All applets (Code IDE, Project Board, Repo Manager) use this as
 * the canonical source of projects.  Each project may link to a
 * boardId and/or repoId so applet-specific data can be loaded from
 * the respective sub-APIs.
 */

export interface UnifiedProject {
  id: string;
  name: string;
  repoId: string;
  boardId: string;
}

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

/** Fetch all IDE projects — the canonical project list. */
export async function fetchProjects(): Promise<UnifiedProject[]> {
  const res = await fetch("/api/v1/ide/project?limit=100", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch projects: ${res.status}`);
  const page = await res.json();
  return (page.items || []).map((p: any) => ({
    id: p.id,
    name: p.name,
    repoId: p.repoId || "",
    boardId: p.boardId || "",
  }));
}

/** Create a new IDE project (canonical). */
export async function createProject(name: string): Promise<UnifiedProject> {
  const res = await fetch("/api/v1/ide/project", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ name }),
  });
  if (!res.ok) throw new Error(`Failed to create project: ${res.status}`);
  const p = await res.json();
  return { id: p.id, name: p.name, repoId: p.repoId || "", boardId: p.boardId || "" };
}

/** Update an IDE project's linked IDs (boardId, repoId). */
export async function updateProject(
  projectId: string,
  updates: { boardId?: string; repoId?: string },
): Promise<UnifiedProject> {
  const res = await fetch(`/api/v1/ide/project/${projectId}`, {
    method: "PATCH",
    headers: authHeaders(),
    body: JSON.stringify(updates),
  });
  if (!res.ok) throw new Error(`Failed to update project: ${res.status}`);
  const p = await res.json();
  return { id: p.id, name: p.name, repoId: p.repoId || "", boardId: p.boardId || "" };
}
