/**
 * Project Board Applet
 *
 * DAG-based project planning board with Cards and Containers.
 * Cards have statuses: todo, active, done, fail, note.
 * Cards can be grouped into Containers (mapped to agent-containers).
 *
 * Data source: /api/v1/pb/board, /api/v1/pb/board/{id}/card, etc.
 */

import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import {
  Button,
  Modal,
  TextField,
  Spinner,
  Tag,
  Card as CardComponent,
} from "@asymmetric-effort/specifyjs/components";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface BoardSummary {
  id: string;
  name: string;
  updatedAt?: string;
}

interface Container {
  id: string;
  title: string;
  agentId?: string;
  minimized: boolean;
}

interface BoardCard {
  id: string;
  title: string;
  status: string;
  content: string;
  containerId?: string;
  note?: string;
}

interface Edge {
  id: string;
  type: string;
  from: string;
  to: string;
}

interface Board extends BoardSummary {
  containers: Container[];
  cards: BoardCard[];
  edges: Edge[];
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

async function fetchBoards(): Promise<BoardSummary[]> {
  const res = await fetch("/api/v1/pb/board?limit=100", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch boards: ${res.status}`);
  const page = await res.json();
  return page.items || [];
}

async function fetchBoard(id: string): Promise<Board> {
  const res = await fetch(`/api/v1/pb/board/${id}`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch board: ${res.status}`);
  return res.json();
}

async function createBoard(name: string): Promise<Board> {
  const res = await fetch("/api/v1/pb/board", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ name, repoId: "" }),
  });
  if (!res.ok) throw new Error(`Failed to create board: ${res.status}`);
  return res.json();
}

async function createCard(boardId: string, title: string, status: string = "todo"): Promise<BoardCard> {
  const res = await fetch(`/api/v1/pb/board/${boardId}/card`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ title, status }),
  });
  if (!res.ok) throw new Error(`Failed to create card: ${res.status}`);
  return res.json();
}

async function updateCard(boardId: string, cardId: string, data: Partial<BoardCard>): Promise<BoardCard> {
  const res = await fetch(`/api/v1/pb/board/${boardId}/card/${cardId}`, {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(`Failed to update card: ${res.status}`);
  return res.json();
}

async function deleteCard(boardId: string, cardId: string): Promise<void> {
  const res = await fetch(`/api/v1/pb/board/${boardId}/card/${cardId}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Failed to delete card: ${res.status}`);
}

// ---------------------------------------------------------------------------
// Status colors and helpers
// ---------------------------------------------------------------------------

const STATUS_COLORS: Record<string, string> = {
  todo: "#eab308",   // yellow
  active: "#3b82f6", // blue
  done: "#22c55e",   // green
  fail: "#ef4444",   // red
  note: "#6b7280",   // grey
};

const STATUS_ORDER = ["todo", "active", "done", "fail", "note"];

function statusTag(status: string) {
  const color = STATUS_COLORS[status] || "#6b7280";
  return h(Tag, { label: status, color, variant: "solid" as const, size: "sm" as const });
}

// ---------------------------------------------------------------------------
// New Board Dialog
// ---------------------------------------------------------------------------

function NewBoardDialog({
  open, onClose, onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (board: Board) => void;
}) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    if (!name.trim()) { setError("Board name is required."); return; }
    setError("");
    setSubmitting(true);
    try {
      const board = await createBoard(name.trim());
      setName("");
      onCreated(board);
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }, [name, onCreated, onClose]);

  if (!open) return null;

  return h(Modal, { open: true, onClose, title: "New Board", size: "sm" as const },
    h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, { placeholder: "Board name", value: name, onChange: (v: string) => setName(v) }),
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
// New Card Dialog
// ---------------------------------------------------------------------------

function NewCardDialog({
  open, onClose, onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (title: string, status: string) => void;
}) {
  const [title, setTitle] = useState("");
  const [status, setStatus] = useState("todo");
  const [error, setError] = useState("");

  const handleSubmit = useCallback(() => {
    if (!title.trim()) { setError("Card title is required."); return; }
    onCreated(title.trim(), status);
    setTitle("");
    setStatus("todo");
    setError("");
    onClose();
  }, [title, status, onCreated, onClose]);

  if (!open) return null;

  return h(Modal, { open: true, onClose, title: "New Card", size: "sm" as const },
    h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, { placeholder: "Card title", value: title, onChange: (v: string) => setTitle(v) }),
      h("div", { style: { display: "flex", gap: "8px" } },
        ...STATUS_ORDER.map((s) =>
          h("button", {
            key: s,
            style: {
              padding: "4px 12px", borderRadius: "4px", border: "none", cursor: "pointer",
              backgroundColor: status === s ? STATUS_COLORS[s] : "#333",
              color: "#fff", fontSize: "12px", fontWeight: status === s ? "700" : "400",
            },
            onClick: () => setStatus(s),
          }, s)
        )
      ),
      error ? h("div", { style: { color: "#ff8888", fontSize: "13px" } }, error) : null,
      h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
        h(Button, { variant: "secondary" as const, onClick: onClose }, "Cancel"),
        h(Button, { variant: "primary" as const, onClick: handleSubmit }, "Add Card")
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Card Detail Dialog
// ---------------------------------------------------------------------------

function CardDetailDialog({
  card, boardId, onClose, onUpdated,
}: {
  card: BoardCard | null;
  boardId: string;
  onClose: () => void;
  onUpdated: () => void;
}) {
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [status, setStatus] = useState("todo");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (card) {
      setTitle(card.title);
      setContent(card.content || "");
      setStatus(card.status);
    }
  }, [card]);

  const handleSave = useCallback(async () => {
    if (!card) return;
    setSaving(true);
    try {
      await updateCard(boardId, card.id, { title, content, status });
      onUpdated();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }, [card, boardId, title, content, status, onUpdated, onClose]);

  const handleDelete = useCallback(async () => {
    if (!card) return;
    try {
      await deleteCard(boardId, card.id);
      onUpdated();
      onClose();
    } catch (err: any) {
      setError(err.message);
    }
  }, [card, boardId, onUpdated, onClose]);

  if (!card) return null;

  return h(Modal, { open: true, onClose, title: `Card: ${card.id}`, size: "md" as const },
    h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, { placeholder: "Title", value: title, onChange: (v: string) => setTitle(v) }),
      h("textarea", {
        value: content,
        onInput: (e: Event) => setContent((e.target as HTMLTextAreaElement).value),
        placeholder: "Content / description",
        style: {
          width: "100%", minHeight: "80px", backgroundColor: "#2d2d2d", color: "#e0e0e0",
          border: "1px solid #444", borderRadius: "4px", padding: "8px",
          fontSize: "13px", fontFamily: "monospace", resize: "vertical",
        },
      }),
      h("div", { style: { display: "flex", gap: "8px" } },
        ...STATUS_ORDER.map((s) =>
          h("button", {
            key: s,
            style: {
              padding: "4px 12px", borderRadius: "4px", border: "none", cursor: "pointer",
              backgroundColor: status === s ? STATUS_COLORS[s] : "#333",
              color: "#fff", fontSize: "12px", fontWeight: status === s ? "700" : "400",
            },
            onClick: () => setStatus(s),
          }, s)
        )
      ),
      error ? h("div", { style: { color: "#ff8888", fontSize: "13px" } }, error) : null,
      h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
        h(Button, { variant: "danger" as const, onClick: handleDelete }, "Delete"),
        h("div", { style: { flex: 1 } }),
        h(Button, { variant: "secondary" as const, onClick: onClose }, "Cancel"),
        h(Button, { variant: "primary" as const, onClick: handleSave, disabled: saving },
          saving ? "Saving..." : "Save"
        )
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Main Project Board Component — Kanban-style columns by status
// ---------------------------------------------------------------------------

export function ProjectBoard() {
  const [boards, setBoards] = useState<BoardSummary[]>([]);
  const [activeBoard, setActiveBoard] = useState<Board | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showNewBoard, setShowNewBoard] = useState(false);
  const [showNewCard, setShowNewCard] = useState(false);
  const [selectedCard, setSelectedCard] = useState<BoardCard | null>(null);

  // Load boards on mount
  useEffect(() => {
    setLoading(true);
    fetchBoards()
      .then((b) => {
        setBoards(b);
        if (b.length > 0) return fetchBoard(b[0].id);
        return null;
      })
      .then((board) => { if (board) setActiveBoard(board); })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  /** Reload the active board */
  const reloadBoard = useCallback(async () => {
    if (!activeBoard) return;
    try {
      const board = await fetchBoard(activeBoard.id);
      setActiveBoard(board);
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeBoard]);

  /** Handle new card creation */
  const handleCreateCard = useCallback(async (title: string, status: string) => {
    if (!activeBoard) return;
    try {
      await createCard(activeBoard.id, title, status);
      await reloadBoard();
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeBoard, reloadBoard]);

  /** Handle board created */
  const handleBoardCreated = useCallback(async (board: Board) => {
    setBoards((prev) => [...prev, { id: board.id, name: board.name }]);
    setActiveBoard(board);
  }, []);

  /** Switch board */
  const switchBoard = useCallback(async (id: string) => {
    try {
      const board = await fetchBoard(id);
      setActiveBoard(board);
    } catch (err: any) {
      setError(err.message);
    }
  }, []);

  // Group cards by status for kanban columns
  const cardsByStatus: Record<string, BoardCard[]> = {};
  for (const s of STATUS_ORDER) cardsByStatus[s] = [];
  if (activeBoard) {
    for (const card of activeBoard.cards) {
      const col = cardsByStatus[card.status] || (cardsByStatus[card.status] = []);
      col.push(card);
    }
  }

  if (loading) {
    return h("div", {
      style: { display: "flex", alignItems: "center", justifyContent: "center", height: "100%", backgroundColor: "#1e1e1e" },
      "data-testid": "project-board",
    }, h(Spinner, null));
  }

  return h("div", {
    style: {
      display: "flex", flexDirection: "column", height: "100%",
      backgroundColor: "#1e1e1e", color: "#e0e0e0",
    },
    "data-testid": "project-board",
  },
    // Toolbar
    h("div", {
      style: {
        display: "flex", alignItems: "center", justifyContent: "space-between",
        padding: "8px 12px", borderBottom: "1px solid #333", flexShrink: 0,
      },
    },
      h("div", { style: { display: "flex", alignItems: "center", gap: "8px" } },
        // Board selector
        h("select", {
          style: {
            backgroundColor: "#333", color: "#e0e0e0", border: "1px solid #555",
            borderRadius: "4px", padding: "4px 8px", fontSize: "13px",
          },
          value: activeBoard?.id || "",
          onChange: (e: Event) => switchBoard((e.target as HTMLSelectElement).value),
        }, ...boards.map((b) =>
          h("option", { key: b.id, value: b.id }, b.name)
        )),
      ),
      h("div", { style: { display: "flex", gap: "8px" } },
        h(Button, { variant: "primary" as const, onClick: () => setShowNewCard(true) }, "New Card"),
        h(Button, { variant: "secondary" as const, onClick: () => setShowNewBoard(true) }, "New Board"),
        h(Button, { variant: "secondary" as const, onClick: reloadBoard }, "Refresh"),
      )
    ),
    // Error banner
    error ? h("div", {
      style: { padding: "4px 8px", backgroundColor: "#3d1c1c", color: "#ff8888", fontSize: "12px", flexShrink: 0 },
      onClick: () => setError(""),
    }, error) : null,
    // Kanban columns
    h("div", {
      style: {
        display: "flex", flex: 1, gap: "8px", padding: "8px",
        overflowX: "auto", overflowY: "hidden",
      },
    },
      ...STATUS_ORDER.map((status) =>
        h("div", {
          key: status,
          style: {
            flex: "1", minWidth: "180px", display: "flex", flexDirection: "column",
            backgroundColor: "#252526", borderRadius: "6px", overflow: "hidden",
          },
        },
          // Column header
          h("div", {
            style: {
              padding: "8px 12px", fontWeight: "600", fontSize: "13px",
              borderBottom: `2px solid ${STATUS_COLORS[status]}`,
              display: "flex", alignItems: "center", justifyContent: "space-between",
            },
          },
            h("span", null, status.toUpperCase()),
            h("span", { style: { fontSize: "11px", color: "#aaa" } },
              String(cardsByStatus[status]?.length || 0)
            )
          ),
          // Cards in column
          h("div", {
            style: { flex: 1, overflowY: "auto", padding: "8px", display: "flex", flexDirection: "column", gap: "6px" },
          },
            ...(cardsByStatus[status] || []).map((card) =>
              h("div", {
                key: card.id,
                style: {
                  padding: "8px 10px", backgroundColor: "#2d2d2d", borderRadius: "4px",
                  borderLeft: `3px solid ${STATUS_COLORS[card.status]}`,
                  cursor: "pointer", fontSize: "13px",
                },
                onClick: () => setSelectedCard(card),
              },
                h("div", { style: { fontWeight: "500", marginBottom: "4px" } }, card.title),
                card.content
                  ? h("div", { style: { fontSize: "11px", color: "#aaa", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" } },
                      card.content.substring(0, 60)
                    )
                  : null,
                h("div", { style: { fontSize: "10px", color: "#666", marginTop: "4px" } }, card.id)
              )
            )
          )
        )
      )
    ),
    // Footer
    h("div", {
      style: {
        padding: "4px 12px", borderTop: "1px solid #333", fontSize: "12px",
        color: "#aaa", flexShrink: 0,
      },
    },
      activeBoard
        ? `${activeBoard.cards.length} card${activeBoard.cards.length !== 1 ? "s" : ""} · ${activeBoard.edges.length} edge${activeBoard.edges.length !== 1 ? "s" : ""}`
        : "No board selected"
    ),
    // Dialogs
    h(NewBoardDialog, { open: showNewBoard, onClose: () => setShowNewBoard(false), onCreated: handleBoardCreated }),
    h(NewCardDialog, { open: showNewCard, onClose: () => setShowNewCard(false), onCreated: handleCreateCard }),
    h(CardDetailDialog, {
      card: selectedCard,
      boardId: activeBoard?.id || "",
      onClose: () => setSelectedCard(null),
      onUpdated: reloadBoard,
    }),
  );
}
