/**
 * Repo Manager Applet
 *
 * GitHub-backed git repository management: repo list, file browser,
 * pull requests with CI checks, and merge gating.
 * Data source: /api/v1/repo/
 */

import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import { Button, Modal, TextField, Spinner, Tag, DataGrid, Tabs } from "@asymmetric-effort/specifyjs/components";
import { useMenuBar } from "./use-menu-bar";
import { fetchProjects, createProject as createIdeProject, updateProject, UnifiedProject } from "./shared-projects";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Repo { id: string; name: string; description: string; defaultBranch: string; visibility: string; updatedAt: string }
interface RepoFile { name: string; type: string; size: number; path: string }
interface PRCheck { name: string; status: string }
interface PullRequest { id: string; repoId: string; title: string; branch: string; targetBranch: string; status: string; author: string; files: string[]; checks: PRCheck[] }

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) };
}

async function fetchRepo(repoId: string): Promise<Repo> {
  const res = await fetch(`/api/v1/repo/repo/${repoId}`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

/** Fetch IDE projects and resolve each project's linked repo.
 *  Returns parallel arrays: unified projects and their resolved repos. */
async function fetchProjectRepos(): Promise<{ projects: UnifiedProject[]; repos: Repo[] }> {
  const projects = await fetchProjects();
  const repos: Repo[] = [];
  for (const proj of projects) {
    if (proj.repoId) {
      try {
        const repo = await fetchRepo(proj.repoId);
        repos.push(repo);
      } catch {
        // Repo may have been deleted — show a placeholder
        repos.push({ id: proj.repoId, name: proj.name, description: "", defaultBranch: "main", visibility: "private", updatedAt: "" });
      }
    } else {
      // Project with no repo — placeholder entry so the selector
      // still lists it (repo can be created on demand)
      repos.push({ id: `__project__${proj.id}`, name: proj.name, description: "(no repo linked)", defaultBranch: "", visibility: "", updatedAt: "" });
    }
  }
  return { projects, repos };
}

async function createRepo(name: string, description: string): Promise<Repo> {
  const res = await fetch("/api/v1/repo/repo", { method: "POST", headers: authHeaders(), body: JSON.stringify({ name, description }) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function fetchFiles(repoId: string): Promise<RepoFile[]> {
  const res = await fetch(`/api/v1/repo/repo/${repoId}/file`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function fetchPRs(repoId: string): Promise<PullRequest[]> {
  const res = await fetch(`/api/v1/repo/repo/${repoId}/pr?limit=100`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return (await res.json()).items || [];
}

async function mergePR(repoId: string, prId: string): Promise<void> {
  const res = await fetch(`/api/v1/repo/repo/${repoId}/pr/${prId}/merge`, { method: "POST", headers: authHeaders() });
  if (!res.ok) { const err = await res.json().catch(() => ({})); throw new Error(err.message || `Merge failed: ${res.status}`); }
}

// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------

export function RepoManager({ principal }: { principal?: any } = {}) {
  const [ideProjects, setIdeProjects] = useState<UnifiedProject[]>([]);
  const [repos, setRepos] = useState<Repo[]>([]);
  const [activeRepo, setActiveRepo] = useState<Repo | null>(null);
  const [files, setFiles] = useState<RepoFile[]>([]);
  const [prs, setPrs] = useState<PullRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showNewRepo, setShowNewRepo] = useState(false);
  const [fileContextMenu, setFileContextMenu] = useState<{ name: string; path: string; x: number; y: number } | null>(null);

  useMenuBar("repo", [
    { label: "Repository", items: [
      { label: "New Repository", onClick: () => setShowNewRepo(true) },
    ]},
  ]);
  const [newRepoName, setNewRepoName] = useState("");
  const [newRepoDesc, setNewRepoDesc] = useState("");

  // Load IDE projects as canonical source, resolve linked repos
  useEffect(() => {
    setLoading(true);
    fetchProjectRepos()
      .then(({ projects, repos: r }) => {
        setIdeProjects(projects);
        setRepos(r);
        // Auto-select the first project that has a real repo
        const firstReal = r.find((repo) => !repo.id.startsWith("__project__"));
        if (firstReal) setActiveRepo(firstReal);
        else if (r.length > 0) setActiveRepo(r[0]);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  // Load files and PRs when active repo changes (skip placeholders)
  useEffect(() => {
    if (!activeRepo || activeRepo.id.startsWith("__project__")) {
      setFiles([]);
      setPrs([]);
      return;
    }
    Promise.all([fetchFiles(activeRepo.id), fetchPRs(activeRepo.id)])
      .then(([f, p]) => { setFiles(f); setPrs(p); })
      .catch((err) => setError(err.message));
  }, [activeRepo]);

  const handleCreateRepo = useCallback(async () => {
    if (!newRepoName.trim()) { setError("Repository name is required."); return; }
    try {
      const repo = await createRepo(newRepoName.trim(), newRepoDesc.trim());
      // Create a canonical IDE project and link the repo
      try {
        const project = await createIdeProject(newRepoName.trim());
        await updateProject(project.id, { repoId: repo.id });
        setIdeProjects((prev) => [...prev, { ...project, repoId: repo.id }]);
      } catch {
        // Non-fatal — repo was still created
      }
      setRepos((prev) => [...prev, repo]);
      setActiveRepo(repo);
      setNewRepoName(""); setNewRepoDesc(""); setShowNewRepo(false);
    } catch (err: any) { setError(err.message); }
  }, [newRepoName, newRepoDesc]);

  const handleMerge = useCallback(async (prId: string) => {
    if (!activeRepo) return;
    try {
      await mergePR(activeRepo.id, prId);
      const updated = await fetchPRs(activeRepo.id);
      setPrs(updated);
    } catch (err: any) { setError(err.message); }
  }, [activeRepo]);

  if (loading) {
    return h("div", { style: { display: "flex", alignItems: "center", justifyContent: "center", height: "100%", backgroundColor: "#1e1e1e" }, "data-testid": "repo-manager" }, h(Spinner, null));
  }

  // Files tab
  const filesTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    files.length > 0
      ? h("div", {
          style: { backgroundColor: "#fff", borderRadius: "4px" },
          onContextMenu: (e: MouseEvent) => {
            let el = e.target as HTMLElement | null;
            while (el && !el.dataset?.filepath) el = el.parentElement;
            if (el?.dataset?.filepath && el?.dataset?.filetype === "file") {
              e.preventDefault();
              setFileContextMenu({ name: el.dataset.filename || "", path: el.dataset.filepath, x: e.clientX, y: e.clientY });
            }
          },
        },
          h(DataGrid, {
            columns: [
              { key: "name", header: "Name", width: 250, render: (v: string, row: any) =>
                row.type === "file"
                  ? h("span", {
                      style: { color: "#2563eb", cursor: "pointer" },
                      title: "Open in Code Monkey IDE",
                      "data-filepath": row.path,
                      "data-filename": v,
                      "data-filetype": "file",
                    }, v)
                  : h("span", { style: { fontWeight: "600" } }, v)
              },
              { key: "type", header: "Type", width: 80 },
              { key: "size", header: "Size", width: 100 },
              { key: "path", header: "Path", width: 250 },
            ],
            data: files.map((f) => ({ id: f.path, name: f.name, type: f.type, size: f.type === "dir" ? "—" : `${f.size} B`, path: f.path })),
            striped: true,
          })
        )
      : h("div", { style: { color: "#aaa", padding: "16px", textAlign: "center" } }, "No files")
  );

  // Pull Requests tab
  const prsTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    prs.length > 0
      ? h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
          h(DataGrid, {
            columns: [
              { key: "title", header: "Title", width: 250 },
              { key: "branch", header: "Branch", width: 120 },
              { key: "author", header: "Author", width: 100 },
              { key: "status", header: "Status", width: 100, render: (v: string) => {
                const color = v === "open" ? "#3b82f6" : v === "merged" ? "#22c55e" : "#6b7280";
                return h(Tag, { label: v, color, variant: "solid" as const, size: "sm" as const });
              }},
              { key: "checks", header: "CI", width: 100, render: (v: string) => {
                const color = v === "passing" ? "green" : v === "failed" ? "red" : "#b45309";
                return h(Tag, { label: v, color, variant: "solid" as const, size: "sm" as const });
              }},
              { key: "actions", header: "", width: 80, render: (_: string, row: any) => {
                if (row._status === "open" && row._checksOk) {
                  return h(Button, { variant: "primary" as any, onClick: () => handleMerge(row.id) }, "Merge");
                }
                return null;
              }},
            ],
            data: prs.map((pr) => {
              const allPass = pr.checks.every((c) => c.status === "passing");
              const checkSummary = pr.checks.length === 0 ? "—" : allPass ? "passing" : pr.checks.some((c) => c.status === "failed") ? "failed" : "running";
              return { id: pr.id, title: pr.title, branch: pr.branch, author: pr.author, status: pr.status, _status: pr.status, checks: checkSummary, _checksOk: allPass, actions: "" };
            }),
            striped: true,
          })
        )
      : h("div", { style: { color: "#aaa", padding: "16px", textAlign: "center" } }, "No pull requests")
  );

  return h("div", { style: { display: "flex", flexDirection: "column", width: "100%", height: "100%", backgroundColor: "#1e1e1e", color: "#e0e0e0" }, "data-testid": "repo-manager" },
    // Toolbar
    h("div", { style: { display: "flex", alignItems: "center", justifyContent: "space-between", padding: "8px 12px", borderBottom: "1px solid #333", flexShrink: 0 } },
      h("select", {
        style: { backgroundColor: "#333", color: "#e0e0e0", border: "1px solid #555", borderRadius: "4px", padding: "4px 8px", fontSize: "13px" },
        value: activeRepo?.id || "",
        onChange: (e: Event) => { const r = repos.find((repo) => repo.id === (e.target as HTMLSelectElement).value); if (r) setActiveRepo(r); },
      }, ...repos.map((r) => h("option", { key: r.id, value: r.id }, r.name + (r.id.startsWith("__project__") ? " (no repo)" : "")))),
      h("div", { style: { display: "flex", gap: "8px" } },
        h(Button, { variant: "primary" as const, onClick: () => setShowNewRepo(true) }, "New Repo"),
      )
    ),
    error ? h("div", { style: { padding: "4px 8px", backgroundColor: "#3d1c1c", color: "#ff8888", fontSize: "12px" }, onClick: () => setError("") }, error) : null,
    h(Tabs, { tabs: [
      { id: "files", label: "Files", content: filesTab },
      { id: "prs", label: "Pull Requests", content: prsTab },
    ] }),
    // New Repo dialog
    showNewRepo ? h(Modal, { open: true, onClose: () => setShowNewRepo(false), title: "New Repository", size: "sm" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h(TextField, { placeholder: "Repository name", value: newRepoName, onChange: (v: string) => setNewRepoName(v) }),
        h(TextField, { placeholder: "Description (optional)", value: newRepoDesc, onChange: (v: string) => setNewRepoDesc(v) }),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setShowNewRepo(false) }, "Cancel"),
          h(Button, { variant: "primary" as const, onClick: handleCreateRepo }, "Create")
        )
      )
    ) : null,
    // File context menu overlay
    fileContextMenu ? h("div", {
      style: { position: "fixed", inset: 0, zIndex: 1000 },
      onClick: () => setFileContextMenu(null),
    },
      h("div", {
        style: {
          position: "absolute", left: `${fileContextMenu.x}px`, top: `${fileContextMenu.y}px`,
          backgroundColor: "#2d2d2d", border: "1px solid #555", borderRadius: "4px",
          boxShadow: "0 4px 12px rgba(0,0,0,0.4)", minWidth: "160px",
        },
        onClick: (e: Event) => e.stopPropagation(),
      },
        h("div", {
          style: { padding: "6px 12px", cursor: "pointer", fontSize: "12px", color: "#e0e0e0" },
          onMouseEnter: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "#3c3c3c"; },
          onMouseLeave: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "transparent"; },
          onClick: () => {
            // Open in IDE — placeholder
            setFileContextMenu(null);
          },
        }, "Open in IDE"),
        h("div", {
          style: { padding: "6px 12px", cursor: "pointer", fontSize: "12px", color: "#e0e0e0" },
          onMouseEnter: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "#3c3c3c"; },
          onMouseLeave: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "transparent"; },
          onClick: () => {
            // Download — placeholder
            setFileContextMenu(null);
          },
        }, "Download"),
      )
    ) : null,
  );
}
