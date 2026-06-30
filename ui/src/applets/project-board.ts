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
  // Fetch boards from the Project Board API
  const res = await fetch("/api/v1/pb/board?limit=100", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed to fetch boards: ${res.status}`);
  const page = await res.json();
  const boards: BoardSummary[] = page.items || [];

  // Also fetch IDE projects and create board references for any that
  // have an associated board — ensures both applets share the same
  // project list
  try {
    const ideRes = await fetch("/api/v1/ide/project?limit=100", { headers: authHeaders() });
    if (ideRes.ok) {
      const idePage = await ideRes.json();
      const existingIds = new Set(boards.map((b) => b.id));
      for (const proj of (idePage.items || [])) {
        if (proj.boardId && !existingIds.has(proj.boardId)) {
          boards.push({ id: proj.boardId, name: proj.name });
        }
      }
    }
  } catch {
    // IDE projects are supplementary — don't fail if unavailable
  }

  return boards;
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

async function createContainer(boardId: string, title: string): Promise<Container> {
  const res = await fetch(`/api/v1/pb/board/${boardId}/container`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ title }),
  });
  if (!res.ok) throw new Error(`Failed to create container: ${res.status}`);
  return res.json();
}

async function fetchAgents(): Promise<Array<{ id: string; project: string; nodeId: string; status: string }>> {
  const res = await fetch("/api/v1/amgr/agent?limit=200", { headers: authHeaders() });
  if (!res.ok) return [];
  const page = await res.json();
  return page.items || [];
}

async function updateContainer(boardId: string, containerId: string, data: Partial<Container>): Promise<Container> {
  const res = await fetch(`/api/v1/pb/board/${boardId}/container/${containerId}`, {
    method: "PATCH",
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(`Failed to update container: ${res.status}`);
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
          ? "Task cards can be attached to containers and dispatched to agents."
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

function CanvasView({
  board,
  onSelectCard,
}: {
  board: Board;
  onSelectCard: (card: BoardCard) => void;
}) {
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });

  // Pan the canvas by dragging on the background
  const handleMouseDown = useCallback((e: MouseEvent) => {
    if ((e.target as HTMLElement).dataset.canvas) {
      setDragging(true);
      setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
    }
  }, [pan]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (dragging) {
      setPan({ x: e.clientX - dragStart.x, y: e.clientY - dragStart.y });
    }
  }, [dragging, dragStart]);

  const handleMouseUp = useCallback(() => {
    setDragging(false);
  }, []);

  // Default card positions if not set — arrange in a grid
  const cardPositions: Record<string, { x: number; y: number }> = {};
  board.cards.forEach((card, i) => {
    if (card.containerId) {
      // Position inside container area
      const contIdx = board.containers.findIndex((c) => c.id === card.containerId);
      cardPositions[card.id] = { x: 50 + (contIdx * 350), y: 80 + (i % 5) * 140 };
    } else {
      const col = i % 4;
      const row = Math.floor(i / 4);
      cardPositions[card.id] = { x: 50 + col * 250, y: 50 + row * 140 };
    }
  });

  // Build SVG edges
  const edgeLines = board.edges.map((edge) => {
    const from = cardPositions[edge.from];
    const to = cardPositions[edge.to];
    if (!from || !to) return null;
    const isDep = edge.type === "DependsOn";
    return h("line", {
      key: edge.id,
      x1: from.x + 90, y1: from.y + 50,
      x2: to.x + 90, y2: to.y + 50,
      stroke: isDep ? "#3b82f6" : "#6b7280",
      strokeWidth: isDep ? 2 : 1,
      strokeDasharray: isDep ? "none" : "4,4",
      markerEnd: isDep ? "url(#arrowhead)" : undefined,
    });
  }).filter(Boolean);

  return h("div", {
    style: {
      flex: 1, position: "relative", overflow: "hidden",
      backgroundColor: "#1a1a2e", cursor: dragging ? "grabbing" : "grab",
    },
    onMouseDown: handleMouseDown,
    onMouseMove: handleMouseMove,
    onMouseUp: handleMouseUp,
    onMouseLeave: handleMouseUp,
    "data-canvas": "true",
  },
    // SVG layer for edges
    h("svg", {
      style: { position: "absolute", top: 0, left: 0, width: "100%", height: "100%", pointerEvents: "none" },
    },
      h("defs", null,
        h("marker", {
          id: "arrowhead", markerWidth: 10, markerHeight: 7, refX: 10, refY: 3.5, orient: "auto",
        },
          h("polygon", { points: "0 0, 10 3.5, 0 7", fill: "#3b82f6" })
        )
      ),
      h("g", { transform: `translate(${pan.x},${pan.y})` }, ...edgeLines)
    ),
    // Cards layer
    h("div", {
      style: { position: "absolute", top: 0, left: 0, transform: `translate(${pan.x}px,${pan.y}px)` },
    },
      // Containers (background rectangles)
      ...board.containers.map((cont, ci) =>
        h("div", {
          key: cont.id,
          style: {
            position: "absolute", left: `${30 + ci * 350}px`, top: "20px",
            width: "300px", minHeight: "400px",
            backgroundColor: "rgba(255,255,255,0.05)", border: "1px dashed #555",
            borderRadius: "8px", padding: "8px",
          },
        },
          h("div", { style: { fontSize: "12px", color: "#aaa", marginBottom: "4px", fontWeight: "600" } },
            cont.title
          )
        )
      ),
      // Cards (positioned absolutely)
      ...board.cards.map((card) => {
        const pos = cardPositions[card.id] || { x: 0, y: 0 };
        const borderColor = STATUS_COLORS[card.status] || "#666";
        return h("div", {
          key: card.id,
          style: {
            position: "absolute", left: `${pos.x}px`, top: `${pos.y}px`,
            width: "180px", minHeight: "80px",
            backgroundColor: "#2d2d2d", borderRadius: "6px",
            border: `2px solid ${borderColor}`,
            padding: "8px 10px", cursor: "pointer",
            boxShadow: "0 2px 8px rgba(0,0,0,0.3)",
          },
          onClick: (e: Event) => { e.stopPropagation(); onSelectCard(card); },
        },
          h("div", { style: { fontSize: "12px", fontWeight: "600", color: "#e0e0e0", marginBottom: "4px" } }, card.title),
          card.content
            ? h("div", { style: { fontSize: "11px", color: "#aaa", overflow: "hidden", maxHeight: "40px" } },
                card.content.substring(0, 80)
              )
            : null,
          h("div", { style: { display: "flex", justifyContent: "space-between", marginTop: "6px" } },
            statusTag(card.status),
            h("span", { style: { fontSize: "9px", color: "#666" } }, card.id)
          )
        );
      })
    )
  );
}

// ---------------------------------------------------------------------------
// Kanban Status View — columns grouped by card status
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
  onAttachAgent,
}: {
  board: Board;
  onSelectCard: (card: BoardCard) => void;
  onMoveCard: (cardId: string, containerId: string | null, newStatus?: string) => void;
  onAttachAgent: (containerId: string, agentId: string) => void;
}) {
  const [dragOverCell, setDragOverCell] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<{ containerId: string; x: number; y: number } | null>(null);
  const [agents, setAgents] = useState<Array<{ id: string; project: string; status: string }>>([]);

  // Fetch agents when context menu opens
  useEffect(() => {
    if (contextMenu) {
      fetchAgents().then(setAgents);
    }
  }, [contextMenu]);
  // Build swim lanes: one per container + one for unassigned cards
  const lanes: Array<{ id: string; label: string }> = [];
  for (const cont of board.containers) {
    lanes.push({ id: cont.id, label: cont.title });
  }
  lanes.push({ id: "__unassigned__", label: "Unassigned" });

  return h("div", {
    style: { display: "flex", flexDirection: "column", flex: 1, overflow: "auto" },
  },
    // Column headers
    h("div", {
      style: { display: "flex", gap: "1px", flexShrink: 0, position: "sticky", top: 0, zIndex: 1 },
    },
      h("div", { style: { width: "120px", flexShrink: 0, padding: "8px", backgroundColor: "#1e1e1e" } }), // spacer for lane labels
      ...STATUS_ORDER.map((status) =>
        h("div", {
          key: status,
          style: {
            flex: 1, minWidth: "150px", padding: "8px 12px",
            fontWeight: "600", fontSize: "13px", backgroundColor: "#252526",
            borderBottom: `2px solid ${STATUS_COLORS[status]}`,
            textAlign: "center",
          },
        }, status.toUpperCase())
      )
    ),
    // Swim lanes
    ...lanes.map((lane) => {
      // Cards in this lane, grouped by status
      const laneCards = board.cards.filter((c) =>
        lane.id === "__unassigned__" ? !c.containerId : c.containerId === lane.id
      );
      // Skip empty unassigned lane if all cards are in containers
      if (lane.id === "__unassigned__" && laneCards.length === 0 && board.containers.length > 0) return null;

      const containerId = lane.id === "__unassigned__" ? null : lane.id;
      const isUnassigned = lane.id === "__unassigned__";

      return h("div", {
        key: lane.id,
        style: {
          display: "flex", gap: "1px",
          borderBottom: "1px solid #333", minHeight: "80px",
        },
      },
        // Lane label — right-click for container context menu
        h("div", {
          style: {
            width: "120px", flexShrink: 0, padding: "8px",
            fontSize: "12px", fontWeight: "600", color: "#aaa",
            backgroundColor: "#1e1e1e", display: "flex", flexDirection: "column",
            alignItems: "flex-start", borderRight: "1px solid #333", cursor: isUnassigned ? "default" : "context-menu",
          },
          onContextMenu: isUnassigned ? undefined : (e: MouseEvent) => {
            e.preventDefault();
            setContextMenu({ containerId: lane.id, x: e.clientX, y: e.clientY });
          },
        },
          h("span", null, lane.label),
          // Show attached agent if any
          (() => {
            const cont = board.containers.find((c) => c.id === lane.id);
            if (cont?.agentId) {
              return h("span", { style: { fontSize: "10px", color: "#6b7280", marginTop: "2px" } },
                `→ ${cont.agentId}`
              );
            }
            return null;
          })()
        ),
        // Cells per status column — each cell is a drop target
        ...STATUS_ORDER.map((status) => {
          const cellCards = laneCards.filter((c) => c.status === status);
          const cellKey = `${lane.id}:${status}`;
          const isOver = dragOverCell === cellKey;

          // Drop rules are validated on actual drop (see onDrop handler).
          // Visual feedback: dim columns that never accept drops.
          // Unassigned lane: only todo and note columns accept drops.
          const canDropHere = !isUnassigned || status === "todo" || status === "note";

          return h("div", {
            key: status,
            style: {
              flex: 1, minWidth: "150px", padding: "6px",
              backgroundColor: isOver && canDropHere ? "rgba(59, 130, 246, 0.15)" : "#252526",
              display: "flex", flexDirection: "column", gap: "4px",
              transition: "background-color 150ms",
              opacity: canDropHere ? "1" : "0.6",
            },
            onDragOver: (e: DragEvent) => {
              if (canDropHere) { e.preventDefault(); setDragOverCell(cellKey); }
            },
            onDragLeave: () => setDragOverCell(null),
            onDrop: (e: DragEvent) => {
              e.preventDefault();
              setDragOverCell(null);
              if (!canDropHere) return;
              const cardId = e.dataTransfer?.getData("text/plain");
              if (cardId) onMoveCard(cardId, containerId, status);
            },
          },
            ...cellCards.map((card) => renderCard(card, onSelectCard))
          );
        })
      );
    }).filter(Boolean),
    // Context menu overlay for "Attach to" agent
    contextMenu ? h("div", {
      style: { position: "fixed", inset: 0, zIndex: 1000 },
      onClick: () => setContextMenu(null),
    },
      h("div", {
        style: {
          position: "absolute", left: `${contextMenu.x}px`, top: `${contextMenu.y}px`,
          backgroundColor: "#2d2d2d", border: "1px solid #555", borderRadius: "4px",
          boxShadow: "0 4px 12px rgba(0,0,0,0.4)", minWidth: "180px", zIndex: 1001,
        },
        onClick: (e: Event) => e.stopPropagation(),
      },
        h("div", { style: { padding: "6px 12px", fontSize: "11px", color: "#888", borderBottom: "1px solid #444" } },
          "Attach to Agent"
        ),
        agents.length === 0
          ? h("div", { style: { padding: "8px 12px", fontSize: "12px", color: "#666" } }, "No agents available")
          : null,
        ...agents.map((agent) =>
          h("div", {
            key: agent.id,
            style: {
              padding: "6px 12px", fontSize: "12px", color: "#e0e0e0",
              cursor: "pointer", display: "flex", justifyContent: "space-between",
            },
            onMouseEnter: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "#3c3c3c"; },
            onMouseLeave: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "transparent"; },
            onClick: () => {
              onAttachAgent(contextMenu.containerId, agent.id);
              setContextMenu(null);
            },
          },
            h("span", null, agent.id),
            h("span", { style: { color: "#888", fontSize: "10px" } }, agent.status)
          )
        ),
        h("div", {
          style: { padding: "6px 12px", fontSize: "12px", color: "#888", borderTop: "1px solid #444", cursor: "pointer" },
          onMouseEnter: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "#3c3c3c"; },
          onMouseLeave: (e: Event) => { (e.target as HTMLElement).style.backgroundColor = "transparent"; },
          onClick: () => {
            onAttachAgent(contextMenu.containerId, "");
            setContextMenu(null);
          },
        }, "Detach")
      )
    ) : null
  );
}

// ---------------------------------------------------------------------------
// New Container Dialog
// ---------------------------------------------------------------------------

function NewContainerDialog({
  open, onClose, onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (title: string) => void;
}) {
  const [title, setTitle] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = useCallback(() => {
    if (!title.trim()) { setError("Container title is required."); return; }
    onCreated(title.trim());
    setTitle("");
    setError("");
    onClose();
  }, [title, onCreated, onClose]);

  if (!open) return null;

  return h(Modal, { open: true, onClose, title: "New Container", size: "sm" as const },
    h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
      h(TextField, { placeholder: "Container title", value: title, onChange: (v: string) => setTitle(v) }),
      error ? h("div", { style: { color: "#ff8888", fontSize: "13px" } }, error) : null,
      h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
        h(Button, { variant: "secondary" as const, onClick: onClose }, "Cancel"),
        h(Button, { variant: "primary" as const, onClick: handleSubmit }, "Create")
      )
    )
  );
}

// ---------------------------------------------------------------------------
// Main Project Board Component
// ---------------------------------------------------------------------------

export function ProjectBoard() {
  const [boards, setBoards] = useState<BoardSummary[]>([]);
  const [activeBoard, setActiveBoard] = useState<Board | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showNewBoard, setShowNewBoard] = useState(false);
  const [showNewCard, setShowNewCard] = useState(false);
  const [showNewContainer, setShowNewContainer] = useState(false);
  const [selectedCard, setSelectedCard] = useState<BoardCard | null>(null);
  const [viewMode, setViewMode] = useState<"status" | "canvas">("status");

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

  /** Handle new container creation */
  const handleCreateContainer = useCallback(async (title: string) => {
    if (!activeBoard) return;
    try {
      await createContainer(activeBoard.id, title);
      await reloadBoard();
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeBoard, reloadBoard]);

  /** Move a card to a different container and/or status (drag-and-drop).
   *  Enforces transition rules:
   *   - Note cards: cannot change status (only container assignment)
   *   - Task cards in active/done/fail: cannot change status via drag
   *   - Task cards in todo: can go to active ONLY if attached to a container
   *   - No card can be dragged TO the note column
   */
  const handleMoveCard = useCallback(async (cardId: string, containerId: string | null, newStatus?: string) => {
    if (!activeBoard) return;
    const card = activeBoard.cards.find((c) => c.id === cardId);
    if (!card) return;

    // Validate status transitions
    let finalStatus = card.status;
    if (newStatus && newStatus !== card.status) {
      // Note cards never change status
      if (card.status === "note") {
        finalStatus = "note";
      }
      // No card can be dragged to note
      else if (newStatus === "note") {
        finalStatus = card.status;
      }
      // Cards in active/done/fail cannot change status via drag
      else if (card.status === "active" || card.status === "done" || card.status === "fail") {
        finalStatus = card.status;
      }
      // Todo → active requires a container
      else if (card.status === "todo" && newStatus === "active") {
        const targetContainer = containerId ?? card.containerId;
        if (targetContainer) {
          finalStatus = "active";
        } else {
          setError("Card must be attached to a container before activation.");
          return;
        }
      }
      // Todo can only go to active (not done/fail directly)
      else if (card.status === "todo" && (newStatus === "done" || newStatus === "fail")) {
        setError("Cards must go through Active status before Done or Fail.");
        return;
      }
      else {
        finalStatus = newStatus;
      }
    }

    try {
      const updates: Partial<BoardCard> = { ...card, status: finalStatus };
      if (containerId !== undefined) {
        updates.containerId = containerId || undefined;
      }
      await updateCard(activeBoard.id, cardId, updates);
      await reloadBoard();
    } catch (err: any) {
      setError(err.message);
    }
  }, [activeBoard, reloadBoard]);

  /** Attach or detach an agent to/from a container */
  const handleAttachAgent = useCallback(async (containerId: string, agentId: string) => {
    if (!activeBoard) return;
    try {
      await updateContainer(activeBoard.id, containerId, { agentId: agentId || undefined });
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
        h(Button, { variant: "secondary" as const, onClick: () => setShowNewContainer(true) }, "New Container"),
        h(Button, { variant: "secondary" as const, onClick: () => setShowNewBoard(true) }, "New Board"),
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
        ? h(StatusView, { board: activeBoard, onSelectCard: setSelectedCard, onMoveCard: handleMoveCard, onAttachAgent: handleAttachAgent })
        : h(CanvasView, { board: activeBoard, onSelectCard: setSelectedCard })
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
    h(NewBoardDialog, { open: showNewBoard, onClose: () => setShowNewBoard(false), onCreated: handleBoardCreated }),
    h(NewCardDialog, { open: showNewCard, onClose: () => setShowNewCard(false), onCreated: handleCreateCard }),
    h(NewContainerDialog, { open: showNewContainer, onClose: () => setShowNewContainer(false), onCreated: handleCreateContainer }),
    h(CardDetailDialog, {
      card: selectedCard,
      boardId: activeBoard?.id || "",
      onClose: () => setSelectedCard(null),
      onUpdated: reloadBoard,
    }),
  );
}
