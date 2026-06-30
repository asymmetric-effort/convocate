/**
 * Code IDE Applet
 *
 * VS Code-style editor with file explorer, tabbed editing, syntax
 * highlighting, and file CRUD operations.  Wired to the /api/v1/ide/
 * API endpoints.
 *
 * Features:
 *   - Project selector (list all projects)
 *   - File explorer sidebar (tree view)
 *   - Tabbed editor with syntax highlighting
 *   - Create / Save / Delete files
 *   - New Project dialog
 *   - Status bar showing file info
 */

import { createElement, useState, useEffect, useCallback, useRef } from "@asymmetric-effort/specifyjs";
import {
  Button,
  Modal,
  TextField,
  Spinner,
  TreeNav,
} from "@asymmetric-effort/specifyjs/components";
import { useMenuBar } from "./use-menu-bar";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Project {
  id: string;
  name: string;
  repoId: string;
}

interface RepoFile {
  name: string;
  type: "file" | "dir";
  size: number;
  path: string;
}

interface FileContent {
  path: string;
  content: string;
  language?: string;
  updatedAt?: string;
}

interface EditorTab {
  path: string;
  content: string;
  modified: boolean;
  language: string;
}

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

async function fetchProjects(): Promise<Project[]> {
  const res = await fetch("/api/v1/ide/project?limit=100", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch projects: ${res.status}`);
  const page = await res.json();
  return page.items || [];
}

async function createProject(name: string): Promise<Project> {
  const res = await fetch("/api/v1/ide/project", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ name }),
  });
  if (!res.ok) throw new Error(`Failed to create project: ${res.status}`);
  return res.json();
}

async function fetchTree(projectId: string, path: string = ""): Promise<RepoFile[]> {
  const query = path ? `?path=${encodeURIComponent(path)}` : "";
  const res = await fetch(`/api/v1/ide/project/${projectId}/tree${query}`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Failed to fetch tree: ${res.status}`);
  return res.json();
}

async function readFile(projectId: string, path: string): Promise<FileContent> {
  const res = await fetch(`/api/v1/ide/project/${projectId}/file/${encodeURIComponent(path)}`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Failed to read file: ${res.status}`);
  return res.json();
}

async function saveFile(projectId: string, path: string, content: string): Promise<FileContent> {
  const res = await fetch(`/api/v1/ide/project/${projectId}/file/${encodeURIComponent(path)}`, {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify({ content }),
  });
  if (!res.ok) throw new Error(`Failed to save file: ${res.status}`);
  return res.json();
}

async function deleteFile(projectId: string, path: string): Promise<void> {
  const res = await fetch(`/api/v1/ide/project/${projectId}/file/${encodeURIComponent(path)}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Failed to delete file: ${res.status}`);
}

// ---------------------------------------------------------------------------
// Syntax highlighting — minimal tokenizer for common languages
// ---------------------------------------------------------------------------

const KEYWORDS = new Set([
  "const", "let", "var", "function", "return", "if", "else", "for", "while",
  "class", "import", "export", "from", "default", "async", "await", "new",
  "this", "try", "catch", "throw", "switch", "case", "break", "continue",
  "type", "interface", "extends", "implements", "package", "func", "struct",
  "true", "false", "null", "undefined", "void", "string", "number", "boolean",
]);

/** Detect language from file extension */
function detectLanguage(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  const map: Record<string, string> = {
    ts: "typescript", tsx: "typescript", js: "javascript", jsx: "javascript",
    go: "go", py: "python", md: "markdown", json: "json", yaml: "yaml",
    yml: "yaml", html: "html", css: "css", sql: "sql", sh: "shell",
    dockerfile: "dockerfile", makefile: "makefile", txt: "text",
  };
  return map[ext] || "text";
}

/** Tokenize a single line for syntax highlighting */
function tokenizeLine(line: string): Array<{ text: string; color: string }> {
  const tokens: Array<{ text: string; color: string }> = [];
  let i = 0;
  while (i < line.length) {
    // Comments
    if (line.substring(i, i + 2) === "//" || line[i] === "#") {
      tokens.push({ text: line.substring(i), color: "#6a9955" });
      break;
    }
    // Strings
    if (line[i] === '"' || line[i] === "'" || line[i] === "`") {
      const quote = line[i];
      let j = i + 1;
      while (j < line.length && line[j] !== quote) {
        if (line[j] === "\\") j++;
        j++;
      }
      tokens.push({ text: line.substring(i, j + 1), color: "#ce9178" });
      i = j + 1;
      continue;
    }
    // Numbers
    if (/\d/.test(line[i]) && (i === 0 || /\W/.test(line[i - 1]))) {
      let j = i;
      while (j < line.length && /[\d.xXa-fA-F]/.test(line[j])) j++;
      tokens.push({ text: line.substring(i, j), color: "#b5cea8" });
      i = j;
      continue;
    }
    // Words (keywords/identifiers)
    if (/[a-zA-Z_]/.test(line[i])) {
      let j = i;
      while (j < line.length && /\w/.test(line[j])) j++;
      const word = line.substring(i, j);
      if (KEYWORDS.has(word)) {
        tokens.push({ text: word, color: "#569cd6" });
      } else {
        tokens.push({ text: word, color: "#cccccc" });
      }
      i = j;
      continue;
    }
    // Operators and punctuation
    tokens.push({ text: line[i], color: "#d4d4d4" });
    i++;
  }
  return tokens;
}

// ---------------------------------------------------------------------------
// File tree item converter for TreeNav
// ---------------------------------------------------------------------------

function filesToTreeItems(files: RepoFile[]): any[] {
  return files.map((f) => ({
    id: f.path,
    label: f.name,
    icon: f.type === "dir" ? "📁" : "📄",
    children: f.type === "dir" ? [] : undefined,
  }));
}

// ---------------------------------------------------------------------------
// New Project Dialog
// ---------------------------------------------------------------------------

function NewProjectDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (project: Project) => void;
}) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    if (!name.trim()) {
      setError("Project name is required.");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      const project = await createProject(name.trim());
      setName("");
      onCreated(project);
      onClose();
    } catch (err: any) {
      setError(err.message || "Create failed.");
    } finally {
      setSubmitting(false);
    }
  }, [name, onCreated, onClose]);

  if (!open) return null;

  return h(Modal, { open: true, onClose, title: "New Project", size: "sm" as const },
    h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, { placeholder: "Project name", value: name, onChange: (v: string) => setName(v) }),
      error ? h("div", { style: { color: "#ff8888", fontSize: "13px" } }, error) : null,
      h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
        h(Button, { variant: "secondary" as const, onClick: onClose, disabled: submitting }, "Cancel"),
        h(Button, { variant: "primary" as const, onClick: handleSubmit, disabled: submitting },
          submitting ? "Creating..." : "Create"
        )
      )
    )
  );
}

// ---------------------------------------------------------------------------
// New File Dialog
// ---------------------------------------------------------------------------

function NewFileDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (path: string) => void;
}) {
  const [path, setPath] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = useCallback(() => {
    if (!path.trim()) {
      setError("File path is required.");
      return;
    }
    onCreated(path.trim());
    setPath("");
    setError("");
    onClose();
  }, [path, onCreated, onClose]);

  if (!open) return null;

  return h(Modal, { open: true, onClose, title: "New File", size: "sm" as const },
    h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, { placeholder: "File path (e.g. src/main.ts)", value: path, onChange: (v: string) => setPath(v) }),
      error ? h("div", { style: { color: "#ff8888", fontSize: "13px" } }, error) : null,
      h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
        h(Button, { variant: "secondary" as const, onClick: onClose }, "Cancel"),
        h(Button, { variant: "primary" as const, onClick: handleSubmit }, "Create")
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Editor Component — syntax-highlighted code editor
// ---------------------------------------------------------------------------

function Editor({
  tab,
  onContentChange,
}: {
  tab: EditorTab | null;
  onContentChange: (content: string) => void;
}) {
  const editorRef = useRef<HTMLTextAreaElement | null>(null);

  if (!tab) {
    return h("div", {
      style: {
        flex: 1, display: "flex", alignItems: "center", justifyContent: "center",
        color: "#666", fontSize: "14px", backgroundColor: "#1e1e1e",
      },
    }, "Select a file to edit");
  }

  const lines = tab.content.split("\n");

  return h("div", { style: { display: "flex", flex: 1, overflow: "hidden", backgroundColor: "#1e1e1e" } },
    // Line numbers
    h("div", {
      style: {
        padding: "8px 12px 8px 8px", textAlign: "right", color: "#858585",
        fontSize: "13px", fontFamily: "monospace", lineHeight: "20px",
        borderRight: "1px solid #333", userSelect: "none", flexShrink: 0,
        overflowY: "hidden", minWidth: "40px",
      },
    }, ...lines.map((_, i) => h("div", { key: i }, String(i + 1)))),
    // Code area — using textarea for editing + overlay for syntax highlighting
    h("div", { style: { flex: 1, position: "relative", overflow: "auto" } },
      // Syntax-highlighted overlay (non-interactive)
      h("pre", {
        style: {
          position: "absolute", top: 0, left: 0, right: 0,
          padding: "8px", margin: 0, fontSize: "13px", fontFamily: "monospace",
          lineHeight: "20px", color: "#cccccc", pointerEvents: "none",
          whiteSpace: "pre", overflow: "hidden",
        },
        "aria-hidden": "true",
      }, ...lines.map((line, i) =>
        h("div", { key: i },
          ...tokenizeLine(line).map((tok, j) =>
            h("span", { key: j, style: { color: tok.color } }, tok.text)
          ),
          line === "" ? "\u200b" : null // zero-width space for empty lines
        )
      )),
      // Transparent textarea for actual editing
      h("textarea", {
        ref: editorRef,
        value: tab.content,
        onInput: (e: Event) => {
          const target = e.target as HTMLTextAreaElement;
          onContentChange(target.value);
        },
        spellcheck: false,
        style: {
          position: "relative", width: "100%", height: "100%",
          padding: "8px", margin: 0, border: "none", outline: "none",
          fontSize: "13px", fontFamily: "monospace", lineHeight: "20px",
          color: "transparent", caretColor: "#ffffff",
          backgroundColor: "transparent", resize: "none",
          whiteSpace: "pre", overflow: "auto",
        },
      })
    )
  );
}

// ---------------------------------------------------------------------------
// Main Code IDE Component
// ---------------------------------------------------------------------------

export function CodeIDE() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProject, setActiveProject] = useState<Project | null>(null);
  const [tree, setTree] = useState<RepoFile[]>([]);
  const [tabs, setTabs] = useState<EditorTab[]>([]);
  const [activeTabPath, setActiveTabPath] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showNewProject, setShowNewProject] = useState(false);
  const [showNewFile, setShowNewFile] = useState(false);
  const [sidebarWidth] = useState(220);

  // Register applet menu bar
  useMenuBar("ide", [
    { label: "File", items: [
      { label: "New File", onClick: () => setShowNewFile(true) },
      { label: "Save", onClick: () => handleSave() },
    ]},
    { label: "Project", items: [
      { label: "New Project", onClick: () => setShowNewProject(true) },
    ]},
  ]);

  // Load projects on mount
  useEffect(() => {
    setLoading(true);
    fetchProjects()
      .then((projs) => {
        setProjects(projs);
        if (projs.length > 0) setActiveProject(projs[0]);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  // Load file tree when active project changes
  useEffect(() => {
    if (!activeProject) return;
    fetchTree(activeProject.id)
      .then(setTree)
      .catch((err) => setError(err.message));
  }, [activeProject]);

  /** Open a file in a tab */
  const openFile = useCallback(async (path: string) => {
    if (!activeProject) return;
    // If already open, just switch to it
    const existing = tabs.find((t) => t.path === path);
    if (existing) {
      setActiveTabPath(path);
      return;
    }
    try {
      const file = await readFile(activeProject.id, path);
      const newTab: EditorTab = {
        path: file.path,
        content: file.content,
        modified: false,
        language: detectLanguage(file.path),
      };
      setTabs((prev) => [...prev, newTab]);
      setActiveTabPath(path);
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeProject, tabs]);

  /** Close a tab */
  const closeTab = useCallback((path: string) => {
    setTabs((prev) => prev.filter((t) => t.path !== path));
    if (activeTabPath === path) {
      setActiveTabPath((prev) => {
        const remaining = tabs.filter((t) => t.path !== path);
        return remaining.length > 0 ? remaining[remaining.length - 1].path : null;
      });
    }
  }, [activeTabPath, tabs]);

  /** Update content in the active tab */
  const handleContentChange = useCallback((content: string) => {
    setTabs((prev) => prev.map((t) =>
      t.path === activeTabPath ? { ...t, content, modified: true } : t
    ));
  }, [activeTabPath]);

  /** Save the active tab */
  const handleSave = useCallback(async () => {
    if (!activeProject || !activeTabPath) return;
    const tab = tabs.find((t) => t.path === activeTabPath);
    if (!tab) return;
    try {
      await saveFile(activeProject.id, tab.path, tab.content);
      setTabs((prev) => prev.map((t) =>
        t.path === activeTabPath ? { ...t, modified: false } : t
      ));
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeProject, activeTabPath, tabs]);

  /** Create a new file */
  const handleNewFile = useCallback(async (path: string) => {
    if (!activeProject) return;
    try {
      await saveFile(activeProject.id, path, "");
      // Refresh tree
      const newTree = await fetchTree(activeProject.id);
      setTree(newTree);
      // Open the new file
      await openFile(path);
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeProject, openFile]);

  /** Handle project creation */
  const handleProjectCreated = useCallback((project: Project) => {
    setProjects((prev) => [...prev, project]);
    setActiveProject(project);
    setTabs([]);
    setActiveTabPath(null);
  }, []);

  /** Handle file tree click */
  const handleTreeSelect = useCallback((itemId: string) => {
    const file = tree.find((f) => f.path === itemId);
    if (file && file.type === "file") {
      openFile(file.path);
    }
  }, [tree, openFile]);

  const activeTab = tabs.find((t) => t.path === activeTabPath) || null;

  if (loading) {
    return h("div", {
      style: { display: "flex", alignItems: "center", justifyContent: "center", height: "100%", backgroundColor: "#1e1e1e" },
      "data-testid": "code-ide",
    }, h(Spinner, null));
  }

  return h("div", {
    style: {
      display: "flex", flexDirection: "column", width: "100%", height: "100%",
      backgroundColor: "#1e1e1e", color: "#cccccc", fontFamily: "monospace",
      overflow: "hidden",
    },
    "data-testid": "code-ide",
  },
    // Menu bar — spans full width
    h("div", {
      style: {
        display: "flex", alignItems: "center", gap: "4px",
        padding: "0 8px", height: "28px", backgroundColor: "#3c3c3c",
        fontSize: "13px", flexShrink: 0, width: "100%", boxSizing: "border-box",
      },
    },
      // Project selector
      h("select", {
        style: {
          backgroundColor: "#3c3c3c", color: "#cccccc", border: "none",
          fontSize: "13px", outline: "none", cursor: "pointer", padding: "2px 4px",
        },
        value: activeProject?.id || "",
        onChange: (e: Event) => {
          const id = (e.target as HTMLSelectElement).value;
          const proj = projects.find((p) => p.id === id);
          if (proj) {
            setActiveProject(proj);
            setTabs([]);
            setActiveTabPath(null);
          }
        },
      }, ...projects.map((p) =>
        h("option", { key: p.id, value: p.id }, p.name)
      )),
      h("div", { style: { flex: 1 } }), // spacer
      h("button", {
        style: { backgroundColor: "transparent", color: "#cccccc", border: "none", cursor: "pointer", fontSize: "12px", padding: "2px 8px" },
        onClick: () => setShowNewProject(true),
      }, "New Project"),
      h("button", {
        style: { backgroundColor: "transparent", color: "#cccccc", border: "none", cursor: "pointer", fontSize: "12px", padding: "2px 8px" },
        onClick: () => setShowNewFile(true),
      }, "New File"),
      h("button", {
        style: { backgroundColor: "transparent", color: "#cccccc", border: "none", cursor: "pointer", fontSize: "12px", padding: "2px 8px" },
        onClick: handleSave,
        disabled: !activeTab?.modified,
      }, "Save"),
    ),
    // Error banner
    error ? h("div", {
      style: { padding: "4px 8px", backgroundColor: "#3d1c1c", color: "#ff8888", fontSize: "12px", flexShrink: 0 },
      onClick: () => setError(""),
    }, error) : null,
    // Main content area — sidebar pinned left, editor fills remaining space
    h("div", { style: { display: "flex", flex: 1, overflow: "hidden", width: "100%" } },
      // Sidebar — file explorer, always pinned to the left
      h("div", {
        style: {
          width: `${sidebarWidth}px`, minWidth: `${sidebarWidth}px`,
          backgroundColor: "#252526",
          borderRight: "1px solid #333", overflowY: "auto", overflowX: "hidden",
          flexShrink: 0, flexGrow: 0,
        },
      },
        h("div", {
          style: { padding: "8px", fontSize: "11px", color: "#aaa", textTransform: "uppercase", letterSpacing: "1px" },
        }, "Explorer"),
        tree.length > 0
          ? h("div", { style: { padding: "0 4px" } },
              ...tree.map((file) =>
                h("div", {
                  key: file.path,
                  style: {
                    padding: "3px 8px", cursor: "pointer", fontSize: "13px",
                    backgroundColor: activeTabPath === file.path ? "#37373d" : "transparent",
                    color: file.type === "dir" ? "#e8e8e8" : "#cccccc",
                    display: "flex", alignItems: "center", gap: "4px",
                    borderRadius: "3px",
                  },
                  onClick: () => {
                    if (file.type === "file") openFile(file.path);
                  },
                },
                  h("span", { style: { fontSize: "12px" } }, file.type === "dir" ? "📁" : "📄"),
                  file.name
                )
              )
            )
          : h("div", { style: { padding: "8px", color: "#666", fontSize: "12px" } }, "No files")
      ),
      // Editor area
      h("div", { style: { display: "flex", flexDirection: "column", flex: 1, overflow: "hidden" } },
        // Tab bar
        h("div", {
          style: {
            display: "flex", height: "35px", backgroundColor: "#252526",
            borderBottom: "1px solid #333", overflow: "auto", flexShrink: 0,
          },
        },
          ...tabs.map((tab) =>
            h("div", {
              key: tab.path,
              style: {
                display: "flex", alignItems: "center", gap: "4px",
                padding: "0 12px", cursor: "pointer", fontSize: "13px",
                backgroundColor: tab.path === activeTabPath ? "#1e1e1e" : "#2d2d2d",
                borderRight: "1px solid #333",
                borderBottom: tab.path === activeTabPath ? "2px solid #007acc" : "none",
                color: tab.path === activeTabPath ? "#ffffff" : "#969696",
                whiteSpace: "nowrap",
              },
              onClick: () => setActiveTabPath(tab.path),
            },
              h("span", null, tab.path.split("/").pop()),
              tab.modified ? h("span", { style: { color: "#e8e8e8", marginLeft: "4px" } }, "●") : null,
              h("span", {
                style: { marginLeft: "8px", fontSize: "11px", color: "#969696", cursor: "pointer" },
                onClick: (e: Event) => { e.stopPropagation(); closeTab(tab.path); },
              }, "✕")
            )
          )
        ),
        // Editor
        h(Editor, { tab: activeTab, onContentChange: handleContentChange })
      )
    ),
    // Status bar
    h("div", {
      style: {
        height: "22px", backgroundColor: "#007acc", color: "#ffffff",
        display: "flex", alignItems: "center", padding: "0 12px",
        fontSize: "12px", flexShrink: 0, gap: "16px",
      },
    },
      h("span", null, activeProject ? activeProject.name : "No Project"),
      activeTab ? h("span", null, activeTab.language) : null,
      activeTab ? h("span", null, `${activeTab.content.split("\n").length} lines`) : null,
      activeTab?.modified ? h("span", null, "Modified") : null,
    ),
    // Dialogs
    h(NewProjectDialog, { open: showNewProject, onClose: () => setShowNewProject(false), onCreated: handleProjectCreated }),
    h(NewFileDialog, { open: showNewFile, onClose: () => setShowNewFile(false), onCreated: handleNewFile }),
  );
}
