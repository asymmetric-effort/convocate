import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import type { Repo, PullRequest, Page } from "../../types/api";
import { apiGet, apiPost } from "../../lib/api";

const h = createElement;

export function RepoManager() {
  const [repos, setRepos] = useState<Repo[]>([]);
  const [selectedRepo, setSelectedRepo] = useState<string | null>(null);
  const [prs, setPrs] = useState<PullRequest[]>([]);
  const [tab, setTab] = useState<"files" | "prs">("files");
  const [loading, setLoading] = useState(true);

  useEffect(() => { loadRepos(); }, []);

  async function loadRepos() {
    setLoading(true);
    const page = await apiGet<Page<Repo>>("/repo/repo?limit=200");
    setRepos(page.items);
    setLoading(false);
  }

  async function selectRepo(id: string) {
    setSelectedRepo(id);
    const page = await apiGet<Page<PullRequest>>(`/repo/repo/${id}/pr?limit=200`);
    setPrs(page.items);
  }

  if (loading) return h("div", { className: "applet-loading" }, "Loading repositories...");

  if (selectedRepo) {
    const repo = repos.find((r: Repo) => r.id === selectedRepo);
    return h("div", { className: "repo-detail" },
      h("div", { className: "applet-toolbar" },
        h("button", { className: "btn", onClick: () => setSelectedRepo(null) }, "\u2190 Back"),
        h("span", { style: { fontWeight: 600, marginLeft: "12px" } }, repo?.name)
      ),
      h("div", { className: "tab-bar" },
        h("button", { className: `tab ${tab === "files" ? "active" : ""}`, onClick: () => setTab("files") }, "Files"),
        h("button", { className: `tab ${tab === "prs" ? "active" : ""}`, onClick: () => setTab("prs") }, `Pull Requests (${prs.length})`)
      ),
      tab === "prs" ? h("div", { className: "grid-list" },
        h("div", { className: "grid-header", style: { gridTemplateColumns: "1fr 2fr 1fr 100px 1fr" } },
          h("span", null, "ID"), h("span", null, "Title"), h("span", null, "Branch"), h("span", null, "Status"), h("span", null, "Author")
        ),
        prs.map((pr: PullRequest, i: number) =>
          h("div", { key: pr.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "1fr 2fr 1fr 100px 1fr" } },
            h("span", { className: "cell-id" }, pr.id),
            h("span", null, pr.title),
            h("span", null, pr.branch),
            h("span", null, h("span", { className: "status-dot", style: { backgroundColor: pr.status === "merged" ? "#4caf50" : pr.status === "open" ? "#2196f3" : "#999" } }), " ", pr.status),
            h("span", null, pr.author)
          )
        )
      ) : h("div", { className: "muted", style: { padding: "20px" } }, "File browser — coming soon")
    );
  }

  return h("div", { className: "repo" },
    h("div", { className: "applet-toolbar" },
      h("span", { className: "applet-count" }, `${repos.length} repositories`)
    ),
    h("div", { className: "grid-list" },
      h("div", { className: "grid-header", style: { gridTemplateColumns: "1fr 2fr 1fr 1fr" } },
        h("span", null, "Name"), h("span", null, "Description"), h("span", null, "Visibility"), h("span", null, "Updated")
      ),
      repos.map((repo: Repo, i: number) =>
        h("div", { key: repo.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "1fr 2fr 1fr 1fr" }, onDblClick: () => selectRepo(repo.id) },
          h("span", { className: "cell-id" }, repo.name),
          h("span", null, repo.description),
          h("span", null, repo.visibility),
          h("span", null, repo.updatedAt?.split("T")[0] || "")
        )
      )
    )
  );
}
