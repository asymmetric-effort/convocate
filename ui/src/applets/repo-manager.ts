/**
 * Repo Manager Applet
 *
 * GitHub-backed git repository management: repo list, file browser,
 * pull requests with CI checks, and merge gating.
 * Data source: /api/v1/repo/
 */

import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import { Button, Modal, TextField, Spinner, Tag, DataGrid, Tabs } from "@asymmetric-effort/specifyjs/components";

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

async function fetchRepos(): Promise<Repo[]> {
  const res = await fetch("/api/v1/repo/repo?limit=200", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return (await res.json()).items || [];
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

export function RepoManager() {
  const [repos, setRepos] = useState<Repo[]>([]);
  const [activeRepo, setActiveRepo] = useState<Repo | null>(null);
  const [files, setFiles] = useState<RepoFile[]>([]);
  const [prs, setPrs] = useState<PullRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showNewRepo, setShowNewRepo] = useState(false);
  const [newRepoName, setNewRepoName] = useState("");
  const [newRepoDesc, setNewRepoDesc] = useState("");

  useEffect(() => {
    setLoading(true);
    fetchRepos()
      .then((r) => { setRepos(r); if (r.length > 0) setActiveRepo(r[0]); })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  // Load files and PRs when active repo changes
  useEffect(() => {
    if (!activeRepo) return;
    Promise.all([fetchFiles(activeRepo.id), fetchPRs(activeRepo.id)])
      .then(([f, p]) => { setFiles(f); setPrs(p); })
      .catch((err) => setError(err.message));
  }, [activeRepo]);

  const handleCreateRepo = useCallback(async () => {
    if (!newRepoName.trim()) { setError("Repository name is required."); return; }
    try {
      const repo = await createRepo(newRepoName.trim(), newRepoDesc.trim());
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
      ? h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
          h(DataGrid, {
            columns: [
              { key: "name", header: "Name", width: 250 },
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
        onChange: (e: Event) => { const r = repos.find((r) => r.id === (e.target as HTMLSelectElement).value); if (r) setActiveRepo(r); },
      }, ...repos.map((r) => h("option", { key: r.id, value: r.id }, r.name))),
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
  );
}
