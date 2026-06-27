import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import type { Project, FileContent, Page } from "../../types/api";
import { apiGet, apiPost, apiPut } from "../../lib/api";

const h = createElement;

export function CodeIDE() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [files, setFiles] = useState<any[]>([]);
  const [openFile, setOpenFile] = useState<FileContent | null>(null);
  const [editorContent, setEditorContent] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => { loadProjects(); }, []);

  async function loadProjects() {
    setLoading(true);
    const page = await apiGet<Page<Project>>("/ide/project?limit=200");
    setProjects(page.items);
    setLoading(false);
  }

  async function selectProject(project: Project) {
    setSelectedProject(project);
    const tree = await apiGet<any[]>(`/ide/project/${project.id}/tree`);
    setFiles(tree);
  }

  async function openFileInEditor(path: string) {
    if (!selectedProject) return;
    const file = await apiGet<FileContent>(`/ide/project/${selectedProject.id}/file/${encodeURIComponent(path)}`);
    setOpenFile(file);
    setEditorContent(file.content);
  }

  async function saveFile() {
    if (!selectedProject || !openFile) return;
    await apiPut(`/ide/project/${selectedProject.id}/file/${encodeURIComponent(openFile.path)}`, { content: editorContent });
  }

  if (loading) return h("div", { className: "applet-loading" }, "Loading projects...");

  if (!selectedProject) {
    return h("div", { className: "ide" },
      h("div", { className: "applet-toolbar" },
        h("button", { className: "btn btn-primary", onClick: async () => {
          const name = prompt("Project name:");
          if (name) { await apiPost("/ide/project", { name }); loadProjects(); }
        }}, "New Project"),
        h("span", { className: "applet-count" }, `${projects.length} projects`)
      ),
      h("div", { className: "grid-list" },
        h("div", { className: "grid-header", style: { gridTemplateColumns: "1fr 1fr 1fr" } },
          h("span", null, "Name"), h("span", null, "Repository"), h("span", null, "Board")
        ),
        projects.map((p: Project, i: number) =>
          h("div", { key: p.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "1fr 1fr 1fr" }, onDblClick: () => selectProject(p) },
            h("span", null, p.name), h("span", { className: "cell-id" }, p.repoId), h("span", null, p.boardId || "—")
          )
        )
      )
    );
  }

  return h("div", { className: "ide", style: { display: "flex", height: "100%" } },
    h("div", { className: "ide-sidebar", style: { width: "200px", borderRight: "1px solid #444", overflow: "auto", padding: "8px" } },
      h("div", { style: { marginBottom: "8px" } },
        h("button", { className: "btn btn-sm", onClick: () => { setSelectedProject(null); setOpenFile(null); } }, "\u2190 Projects"),
        h("div", { style: { fontWeight: 600, marginTop: "8px" } }, selectedProject.name)
      ),
      files.map((f: any) =>
        h("div", { key: f.path, className: "file-item", style: { padding: "4px 8px", cursor: "pointer", fontSize: "12px", borderRadius: "4px" }, onClick: () => openFileInEditor(f.path) },
          f.type === "dir" ? "\uD83D\uDCC1 " : "\uD83D\uDCC4 ", f.name
        )
      )
    ),
    h("div", { style: { flex: 1, display: "flex", flexDirection: "column" } },
      openFile ? h("div", { style: { flex: 1, display: "flex", flexDirection: "column" } },
        h("div", { style: { padding: "8px 12px", borderBottom: "1px solid #444", display: "flex", justifyContent: "space-between", alignItems: "center" } },
          h("span", { style: { fontSize: "12px", color: "#aaa" } }, openFile.path),
          h("button", { className: "btn btn-sm btn-primary", onClick: saveFile }, "Save")
        ),
        h("textarea", {
          value: editorContent,
          onInput: (e: any) => setEditorContent(e.target.value),
          style: { flex: 1, width: "100%", background: "#0d0d0d", color: "#d4d4d4", border: "none", padding: "12px", fontFamily: "monospace", fontSize: "13px", resize: "none", outline: "none" }
        })
      ) : h("div", { className: "applet-placeholder" }, "Select a file to edit")
    )
  );
}
