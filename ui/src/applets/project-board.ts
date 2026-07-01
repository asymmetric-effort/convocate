/**
 * Project Board Applet
 *
 * DAG-based project planning board with Cards.
 * Cards have statuses: todo, active, done, fail, note.
 *
 * Data source: /api/v1/pb/board, /api/v1/pb/board/{id}/card, etc.
 */

import { createElement, useState, useEffect, useCallback, useRef } from "@asymmetric-effort/specifyjs";
import {
  Board,
  Button,
  Modal,
  TextField,
  Spinner,
  Tag,
  useBoardReducer,
} from "@asymmetric-effort/specifyjs/components";
import { useMenuBar } from "./use-menu-bar";
import { fetchProjects, createProject, UnifiedProject } from "./shared-projects";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface BoardSummary {
  id: string;
  name: string;
  updatedAt?: string;
}

interface BoardCard {
  id: string;
  title: string;
  status: string;
  content: string;
  note?: string;
}

interface Edge {
  id: string;
  type: string;
  from: string;
  to: string;
}

interface BoardData extends BoardSummary {
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

async function fetchBoard(id: string): Promise<BoardData> {
  const res = await fetch(`/api/v1/pb/board/${id}`, { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch board: ${res.status}`);
  return res.json();
}

async function createBoard(name: string): Promise<BoardData> {
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

async function createEdge(boardId: string, from: string, to: string, type: string = "RelatesTo"): Promise<Edge> {
  const res = await fetch(`/api/v1/pb/board/${boardId}/edge`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ from, to, type }),
  });
  if (!res.ok) throw new Error(`Failed to create edge: ${res.status}`);
  return res.json();
}

async function deleteEdge(boardId: string, edgeId: string): Promise<void> {
  const res = await fetch(`/api/v1/pb/board/${boardId}/edge/${edgeId}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`Failed to delete edge: ${res.status}`);
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
// New Project Dialog
// ---------------------------------------------------------------------------

function NewProjectDialog({
  open, onClose, onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (project: UnifiedProject) => void;
}) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    if (!name.trim()) { setError("Project name is required."); return; }
    setError("");
    setSubmitting(true);
    try {
      const project = await createProject(name.trim());
      setName("");
      onCreated(project);
      onClose();
    } catch (err: any) {
      setError(err.message);
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
  const [cardType, setCardType] = useState<"todo" | "note">("todo");
  const [error, setError] = useState("");

  const handleSubmit = useCallback(() => {
    if (!title.trim()) { setError("Card title is required."); return; }
    // Cards are always created in todo or note status
    onCreated(title.trim(), cardType);
    setTitle("");
    setCardType("todo");
    setError("");
    onClose();
  }, [title, cardType, onCreated, onClose]);

  if (!open) return null;

  return h(Modal, { open: true, onClose, title: "New Card", size: "sm" as const },
    h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, { placeholder: "Card title", value: title, onChange: (v: string) => setTitle(v) }),
      // Card type: Task (todo) or Note
      h("div", { style: { display: "flex", gap: "8px" } },
        h("button", {
          style: {
            padding: "6px 16px", borderRadius: "4px", border: "none", cursor: "pointer",
            backgroundColor: cardType === "todo" ? STATUS_COLORS.todo : "#333",
            color: "#fff", fontSize: "12px", fontWeight: cardType === "todo" ? "700" : "400",
          },
          onClick: () => setCardType("todo"),
        }, "Task"),
        h("button", {
          style: {
            padding: "6px 16px", borderRadius: "4px", border: "none", cursor: "pointer",
            backgroundColor: cardType === "note" ? STATUS_COLORS.note : "#333",
            color: "#fff", fontSize: "12px", fontWeight: cardType === "note" ? "700" : "400",
          },
          onClick: () => setCardType("note"),
        }, "Note"),
      ),
      h("div", { style: { fontSize: "11px", color: "#aaa" } },
        cardType === "todo"
          ? "Task cards track work items through the board workflow."
          : "Note cards are for documentation — they cannot be executed."
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
// Canvas View — free-form board with positioned cards and SVG edges
// ---------------------------------------------------------------------------

// Convert API board data to SpecifyJS Board state format
function apiBoardToBoardState(board: BoardData): any {
  const now = Date.now();
  const collection: any[] = [];

  // Add cards
  board.cards.forEach((card, i) => {
    const color = STATUS_COLORS[card.status] || "#6b7280";
    const col = i % 4;
    const row = Math.floor(i / 4);
    const cardItem: any = {
      type: "card",
      card_id: card.id,
      card_type: "text",
      card_title: card.title,
      card_link: [],
      content: { text: card.content || "" },
      color,
      position: { x: 50 + col * 220, y: 50 + row * 150 },
      size: { width: 180, height: 120 },
      priority: card.status === "fail" ? "critical" : card.status === "active" ? "high" : "medium",
      createdAt: now,
      updatedAt: now,
    };

    // Build links from edges
    board.edges
      .filter((e) => e.from === card.id)
      .forEach((e) => {
        cardItem.card_link.push({
          link_id: e.id,
          link_name: e.type === "DependsOn" ? "depends on" : "relates to",
          target_card_id: e.to,
          color: e.type === "DependsOn" ? "#3b82f6" : "#94a3b8",
          attributes: {},
        });
      });

    collection.push(cardItem);
  });

  return {
    id: board.id,
    name: board.name,
    collection,
    viewport: { panX: 0, panY: 0, zoom: 1 },
  };
}

// Canvas view using the SpecifyJS Board component with drag-and-drop,
// card linking, and management.
function CanvasView({
  board,
  onSelectCard,
  onReloadBoard,
}: {
  board: BoardData;
  onSelectCard: (card: BoardCard) => void;
  onReloadBoard: () => void;
}) {
  // Convert API data to Board state only on initial mount or board ID change
  const boardIdRef = useRef<string>("");
  const initialState = apiBoardToBoardState(board);
  const { state, dispatch } = useBoardReducer(initialState);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<{ cardId: string; x: number; y: number } | null>(null);

  // Only reset Board state when the board ID changes (switching boards),
  // not on every API data refresh — this lets the Board component own its
  // own drag/layout state between reloads.
  useEffect(() => {
    if (board.id !== boardIdRef.current) {
      boardIdRef.current = board.id;
      dispatch({ type: "SET_BOARD", state: apiBoardToBoardState(board) });
    }
  }, [board.id, dispatch]);

  // Persist card position/size changes back to the API after drag-end.
  const prevCollectionRef = useRef<string>("");
  useEffect(() => {
    const serialized = JSON.stringify(state.collection);
    if (prevCollectionRef.current && serialized !== prevCollectionRef.current) {
      const syncCards = (items: any[]) => {
        for (const item of items) {
          if (item.type === "card") {
            updateCard(board.id, item.card_id, {
              position: { x: item.position.x, y: item.position.y },
              size: { w: item.size.width, h: item.size.height },
            } as any).catch(() => { /* position sync is best-effort */ });
          }
        }
      };
      syncCards(state.collection);
    }
    prevCollectionRef.current = serialized;
  }, [state.collection, board.id]);

  // Handle card selection — find the matching API card and open detail
  const handleSelectItem = useCallback((itemId: string | null) => {
    setSelectedId(itemId);
    if (itemId) {
      const apiCard = board.cards.find((c) => c.id === itemId);
      if (apiCard) {
        onSelectCard(apiCard);
      }
    }
  }, [board.cards, onSelectCard]);

  // Right-click context menu on cards
  const handleCardContextMenu = useCallback((cardId: string, x: number, y: number) => {
    setContextMenu({ cardId, x, y });
  }, []);

  // Save card title/content edits from the Board's inline editing
  const handleUpdateItem = useCallback((itemId: string, updates: any) => {
    const data: Partial<BoardCard> = {};
    if (updates.card_title !== undefined) data.title = updates.card_title;
    if (updates.content?.text !== undefined) data.content = updates.content.text;
    if (Object.keys(data).length === 0) return;
    updateCard(board.id, itemId, data)
      .then(() => onReloadBoard())
      .catch(() => { /* silent */ });
  }, [board.id, onReloadBoard]);

  // Context menu actions
  const handleDeleteCard = useCallback(async () => {
    if (!contextMenu) return;
    try {
      await deleteCard(board.id, contextMenu.cardId);
      setContextMenu(null);
      onReloadBoard();
    } catch { /* silent */ }
  }, [contextMenu, board.id, onReloadBoard]);

  const handleChangeStatus = useCallback(async (newStatus: string) => {
    if (!contextMenu) return;
    try {
      await updateCard(board.id, contextMenu.cardId, { status: newStatus });
      setContextMenu(null);
      onReloadBoard();
    } catch { /* silent */ }
  }, [contextMenu, board.id, onReloadBoard]);

  return h("div", { style: { flex: 1, overflow: "hidden", position: "relative" } },
    h(Board, {
      state,
      dispatch,
      selectedId,
      onSelectItem: handleSelectItem,
      gridEnabled: true,
      onCardContextMenu: handleCardContextMenu,
      onUpdateItem: handleUpdateItem,
    }),
    // Context menu overlay
    contextMenu ? h("div", {
      style: { position: "fixed", inset: 0, zIndex: 1000 },
      onClick: () => setContextMenu(null),
    },
      h("div", {
        style: {
          position: "absolute", left: `${contextMenu.x}px`, top: `${contextMenu.y}px`,
          backgroundColor: "#2d2d2d", border: "1px solid #555", borderRadius: "4px",
          boxShadow: "0 4px 12px rgba(0,0,0,0.4)", minWidth: "160px", zIndex: 1001,
        },
        onClick: (e: Event) => e.stopPropagation(),
      },
        h("div", { style: { padding: "6px 12px", fontSize: "11px", color: "#888", borderBottom: "1px solid #444" } },
          "Card Actions"
        ),
        ...STATUS_ORDER.map((s) =>
          h("div", {
            key: s,
            style: {
              padding: "6px 12px", fontSize: "12px", color: "#e0e0e0",
              cursor: "pointer", display: "flex", alignItems: "center", gap: "8px",
            },
            onMouseEnter: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "#3c3c3c"; },
            onMouseLeave: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "transparent"; },
            onClick: () => handleChangeStatus(s),
          },
            h("span", { style: { width: "8px", height: "8px", borderRadius: "50%", backgroundColor: STATUS_COLORS[s], display: "inline-block" } }),
            h("span", null, `Set ${s}`)
          )
        ),
        h("div", { style: { borderTop: "1px solid #444" } }),
        h("div", {
          style: {
            padding: "6px 12px", fontSize: "12px", color: "#ff8888", cursor: "pointer",
          },
          onMouseEnter: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "#3c3c3c"; },
          onMouseLeave: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "transparent"; },
          onClick: handleDeleteCard,
        }, "Delete Card")
      )
    ) : null
  );
}

// ---------------------------------------------------------------------------
// Kanban Status View — flat columns grouped by card status
// ---------------------------------------------------------------------------

/** Render a single draggable card in a kanban cell */
function renderCard(
  card: BoardCard,
  onSelectCard: (c: BoardCard) => void,
  onDragStart?: (cardId: string) => void,
) {
  return h("div", {
    key: card.id,
    draggable: true,
    onDragStart: (e: DragEvent) => {
      e.dataTransfer?.setData("text/plain", card.id);
      if (onDragStart) onDragStart(card.id);
    },
    style: {
      padding: "8px 10px", backgroundColor: "#2d2d2d", borderRadius: "4px",
      borderLeft: `3px solid ${STATUS_COLORS[card.status]}`,
      cursor: "grab", fontSize: "13px",
    },
    onClick: () => onSelectCard(card),
  },
    h("div", { style: { fontWeight: "500", marginBottom: "4px" } }, card.title),
    card.content
      ? h("div", { style: { fontSize: "11px", color: "#aaa", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" } },
          card.content.substring(0, 60)
        )
      : null,
    h("div", { style: { fontSize: "10px", color: "#666", marginTop: "4px" } }, card.id)
  );
}

function StatusView({
  board,
  onSelectCard,
  onMoveCard,
}: {
  board: BoardData;
  onSelectCard: (card: BoardCard) => void;
  onMoveCard: (cardId: string, newStatus: string) => void;
}) {
  const [dragOverCol, setDragOverCol] = useState<string | null>(null);

  return h("div", {
    style: { display: "flex", flex: 1, overflow: "auto", gap: "1px" },
  },
    ...STATUS_ORDER.map((status) => {
      const colCards = board.cards.filter((c) => c.status === status);
      const isOver = dragOverCol === status;

      return h("div", {
        key: status,
        style: {
          flex: 1, minWidth: "180px", display: "flex", flexDirection: "column",
          backgroundColor: "#252526",
        },
        onDragOver: (e: DragEvent) => {
          e.preventDefault();
          setDragOverCol(status);
        },
        onDragLeave: () => setDragOverCol(null),
        onDrop: (e: DragEvent) => {
          e.preventDefault();
          setDragOverCol(null);
          const cardId = e.dataTransfer?.getData("text/plain");
          if (cardId) onMoveCard(cardId, status);
        },
      },
        // Column header
        h("div", {
          style: {
            padding: "8px 12px", fontWeight: "600", fontSize: "13px",
            borderBottom: `2px solid ${STATUS_COLORS[status]}`,
            textAlign: "center", flexShrink: 0,
            display: "flex", justifyContent: "center", alignItems: "center", gap: "6px",
          },
        },
          h("span", null, status.toUpperCase()),
          h("span", { style: { fontSize: "11px", color: "#888", fontWeight: "400" } }, `(${colCards.length})`)
        ),
        // Scrollable card list
        h("div", {
          style: {
            flex: 1, padding: "6px", display: "flex", flexDirection: "column", gap: "4px",
            overflowY: "auto",
            backgroundColor: isOver ? "rgba(59, 130, 246, 0.15)" : "transparent",
            transition: "background-color 150ms",
          },
        },
          ...colCards.map((card) => renderCard(card, onSelectCard))
        )
      );
    })
  );
}

// ---------------------------------------------------------------------------
// Main Project Board Component
// ---------------------------------------------------------------------------

export function ProjectBoard({ principal }: { principal?: any } = {}) {
  const [ideProjects, setIdeProjects] = useState<UnifiedProject[]>([]);
  const [boards, setBoards] = useState<BoardSummary[]>([]);
  const [activeBoard, setActiveBoard] = useState<BoardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showNewProject, setShowNewProject] = useState(false);
  const [showNewCard, setShowNewCard] = useState(false);
  const [selectedCard, setSelectedCard] = useState<BoardCard | null>(null);
  const [viewMode, setViewMode] = useState<"status" | "canvas">("status");

  // Register applet menu bar
  useMenuBar("pb", [
    { label: "Graph", items: [
      { label: "New Card", onClick: () => setShowNewCard(true) },
      { label: "", divider: true },
      { label: "New Project", onClick: () => setShowNewProject(true) },
    ]},
    { label: "View", items: [
      { label: "Status View", onClick: () => setViewMode("status") },
      { label: "Canvas View", onClick: () => setViewMode("canvas") },
    ]},
  ]);

  // Load IDE projects on mount — projects are the canonical list
  useEffect(() => {
    setLoading(true);
    fetchProjects()
      .then((projects) => {
        setIdeProjects(projects);
        const b: BoardSummary[] = projects.map((p) => ({
          id: p.boardId || `__project__${p.id}`,
          name: p.name,
          updatedAt: undefined,
        }));
        setBoards(b);
        // Auto-select the first project that has a board
        const firstWithBoard = projects.find((p) => p.boardId);
        if (firstWithBoard) {
          return fetchBoard(firstWithBoard.boardId);
        }
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

  /** Move a card to a different status (drag-and-drop).
   *  Cards can move between status columns freely.
   */
  const handleMoveCard = useCallback(async (cardId: string, newStatus: string) => {
    if (!activeBoard) return;
    const card = activeBoard.cards.find((c) => c.id === cardId);
    if (!card) return;
    if (newStatus === card.status) return;

    try {
      await updateCard(activeBoard.id, cardId, { status: newStatus });
      await reloadBoard();
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeBoard, reloadBoard]);

  /** Handle project created — load its board. */
  const handleProjectCreated = useCallback(async (project: UnifiedProject) => {
    setIdeProjects((prev) => [...prev, project]);
    setBoards((prev) => [...prev, { id: project.boardId || `__project__${project.id}`, name: project.name }]);
    if (project.boardId) {
      try {
        const board = await fetchBoard(project.boardId);
        setActiveBoard(board);
      } catch (err: any) {
        setError(err.message);
      }
    }
  }, []);

  /** Switch board — if the selected entry is a project without a
   *  board yet, create a board on demand and link it. */
  const switchBoard = useCallback(async (id: string) => {
    try {
      // Check if this is a placeholder for a project with no board
      if (id.startsWith("__project__")) {
        const projectId = id.replace("__project__", "");
        const proj = ideProjects.find((p) => p.id === projectId);
        if (!proj) return;
        // Create a board for this project
        const board = await createBoard(proj.name);
        // Update local state
        setIdeProjects((prev) => prev.map((p) => p.id === proj.id ? { ...p, boardId: board.id } : p));
        setBoards((prev) => prev.map((b) => b.id === id ? { ...b, id: board.id } : b));
        setActiveBoard(board);
        return;
      }
      const board = await fetchBoard(id);
      setActiveBoard(board);
    } catch (err: any) {
      setError(err.message);
    }
  }, [ideProjects]);

  if (loading) {
    return h("div", {
      style: { display: "flex", alignItems: "center", justifyContent: "center", height: "100%", backgroundColor: "#1e1e1e" },
      "data-testid": "project-board",
    }, h(Spinner, null));
  }

  return h("div", {
    style: {
      display: "flex", flexDirection: "column", width: "100%", height: "100%",
      backgroundColor: "#1e1e1e", color: "#e0e0e0",
      overflow: "hidden",
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
        // Project selector
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
      h("div", { style: { display: "flex", gap: "8px", alignItems: "center" } },
        // View toggle
        h("div", { style: { display: "flex", borderRadius: "4px", overflow: "hidden", border: "1px solid #555" } },
          h("button", {
            style: {
              padding: "4px 10px", border: "none", cursor: "pointer", fontSize: "12px",
              backgroundColor: viewMode === "status" ? "#007acc" : "#333", color: "#fff",
            },
            onClick: () => setViewMode("status"),
          }, "Status"),
          h("button", {
            style: {
              padding: "4px 10px", border: "none", cursor: "pointer", fontSize: "12px",
              backgroundColor: viewMode === "canvas" ? "#007acc" : "#333", color: "#fff",
            },
            onClick: () => setViewMode("canvas"),
          }, "Canvas"),
        ),
        h(Button, { variant: "primary" as const, onClick: () => setShowNewCard(true) }, "New Card"),
        h(Button, { variant: "secondary" as const, onClick: () => setShowNewProject(true) }, "New Project"),
        h(Button, { variant: "secondary" as const, onClick: reloadBoard }, "Refresh"),
      )
    ),
    // Error banner
    error ? h("div", {
      style: { padding: "4px 8px", backgroundColor: "#3d1c1c", color: "#ff8888", fontSize: "12px", flexShrink: 0 },
      onClick: () => setError(""),
    }, error) : null,
    // View — status kanban or canvas
    activeBoard
      ? viewMode === "status"
        ? h(StatusView, { board: activeBoard, onSelectCard: setSelectedCard, onMoveCard: handleMoveCard })
        : h(CanvasView, { board: activeBoard, onSelectCard: setSelectedCard, onReloadBoard: reloadBoard })
      : h("div", { style: { flex: 1, display: "flex", alignItems: "center", justifyContent: "center", color: "#666" } }, "No board selected"),
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
    h(NewProjectDialog, { open: showNewProject, onClose: () => setShowNewProject(false), onCreated: handleProjectCreated }),
    h(NewCardDialog, { open: showNewCard, onClose: () => setShowNewCard(false), onCreated: handleCreateCard }),
    h(CardDetailDialog, {
      card: selectedCard,
      boardId: activeBoard?.id || "",
      onClose: () => setSelectedCard(null),
      onUpdated: reloadBoard,
    }),
  );
}
