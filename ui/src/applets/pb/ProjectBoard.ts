import { createElement, useState, useEffect, useRef } from "@asymmetric-effort/specifyjs";
import type { Board, BoardSummary, Card, Container, Edge, Page } from "../../types/api";
import { apiGet, apiPost, apiPatch, apiDelete } from "../../lib/api";

const h = createElement;

export function ProjectBoard() {
  const [boards, setBoards] = useState<BoardSummary[]>([]);
  const [activeBoard, setActiveBoard] = useState<Board | null>(null);
  const [loading, setLoading] = useState(true);
  const [dragging, setDragging] = useState<{ type: "container" | "card"; id: string; startX: number; startY: number; origX: number; origY: number } | null>(null);
  const [minimizedContainers, setMinimizedContainers] = useState<Set<string>>(new Set());
  const [minimizedCards, setMinimizedCards] = useState<Set<string>>(new Set());

  useEffect(() => { loadBoards(); }, []);

  async function loadBoards() {
    setLoading(true);
    const page = await apiGet<Page<BoardSummary>>("/pb/board?limit=200");
    setBoards(page.items);
    setLoading(false);
  }

  async function openBoard(id: string) {
    const board = await apiGet<Board>(`/pb/board/${id}`);
    setActiveBoard(board);
    setMinimizedContainers(new Set());
    setMinimizedCards(new Set());
  }

  async function createBoard() {
    const name = prompt("Board name:");
    if (!name) return;
    const board = await apiPost<Board>("/pb/board", { name });
    setActiveBoard(board);
    loadBoards();
  }

  function handleMouseDown(type: "container" | "card", id: string, e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    const items = type === "container" ? activeBoard?.containers : activeBoard?.cards;
    const item: any = (items || []).find((i: any) => i.id === id);
    if (!item) return;
    const origX = type === "container" ? (item.geometry?.x || 50) : (item.position?.x || 100);
    const origY = type === "container" ? (item.geometry?.y || 50) : (item.position?.y || 100);
    setDragging({ type, id, startX: e.clientX, startY: e.clientY, origX, origY });
  }

  function handleMouseMove(e: MouseEvent) {
    if (!dragging || !activeBoard) return;
    const dx = e.clientX - dragging.startX;
    const dy = e.clientY - dragging.startY;
    const newX = Math.max(0, dragging.origX + dx);
    const newY = Math.max(0, dragging.origY + dy);

    if (dragging.type === "container") {
      setActiveBoard({
        ...activeBoard,
        containers: activeBoard.containers.map((c: Container) =>
          c.id === dragging.id ? { ...c, geometry: { ...(c.geometry || { w: 300, h: 200 }), x: newX, y: newY } } : c
        ),
      });
    } else {
      setActiveBoard({
        ...activeBoard,
        cards: activeBoard.cards.map((c: Card) =>
          c.id === dragging.id ? { ...c, position: { x: newX, y: newY } } : c
        ),
      });
    }
  }

  function handleMouseUp() {
    if (!dragging || !activeBoard) { setDragging(null); return; }
    const { type, id } = dragging;
    setDragging(null);

    // Persist position to API
    if (type === "container") {
      const container = activeBoard.containers.find((c: Container) => c.id === id);
      if (container?.geometry) {
        apiPatch(`/pb/board/${activeBoard.id}/container/${id}`, { geometry: container.geometry });
      }
    } else {
      const card = activeBoard.cards.find((c: Card) => c.id === id);
      if (card?.position) {
        apiPatch(`/pb/board/${activeBoard.id}/card/${id}`, { position: card.position });
      }
    }
  }

  function toggleContainerMinimize(id: string) {
    setMinimizedContainers((prev: Set<string>) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  function toggleCardMinimize(id: string) {
    setMinimizedCards((prev: Set<string>) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  if (loading) return h("div", { className: "applet-loading" }, "Loading boards...");

  if (!activeBoard) {
    return h("div", { className: "pb" },
      h("div", { className: "applet-toolbar" },
        h("button", { className: "btn btn-primary", onClick: createBoard }, "New Board"),
        h("span", { className: "applet-count" }, `${boards.length} board${boards.length !== 1 ? "s" : ""}`)
      ),
      h("div", { className: "grid-list" },
        h("div", { className: "grid-header", style: { gridTemplateColumns: "1fr 1fr 1fr" } },
          h("span", null, "Name"), h("span", null, "Repository"), h("span", null, "Updated")
        ),
        boards.map((b: BoardSummary, i: number) =>
          h("div", { key: b.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "1fr 1fr 1fr" }, onDblClick: () => openBoard(b.id) },
            h("span", null, b.name), h("span", { className: "cell-id" }, b.repoId || "\u2014"), h("span", null, b.updatedAt?.split("T")[0] || "")
          )
        )
      )
    );
  }

  const cardColors: Record<string, string> = { todo: "#ffd54f", active: "#42a5f5", done: "#66bb6a", fail: "#ef5350", note: "#bdbdbd" };

  return h("div", {
    className: "pb-canvas",
    style: { position: "relative", width: "100%", height: "100%", overflow: "auto", background: "#1a1a1a" },
    onMouseMove: handleMouseMove,
    onMouseUp: handleMouseUp,
    onMouseLeave: handleMouseUp,
  },
    h("div", { className: "applet-toolbar", style: { position: "sticky", top: 0, zIndex: 100, background: "#1e1e1e", borderBottom: "1px solid #444" } },
      h("button", { className: "btn btn-sm", onClick: () => setActiveBoard(null) }, "\u2190 Boards"),
      h("span", { style: { fontWeight: 600, marginLeft: "12px" } }, `Project Board: ${activeBoard.name}`),
      h("button", { className: "btn btn-sm", style: { marginLeft: "auto" }, onClick: async () => {
        const title = prompt("Card title:");
        if (!title) return;
        await apiPost(`/pb/board/${activeBoard.id}/card`, { title, status: "todo", content: "" });
        openBoard(activeBoard.id);
      }}, "Add Card"),
      h("button", { className: "btn btn-sm", style: { marginLeft: "8px" }, onClick: async () => {
        const title = prompt("Container title:");
        if (!title) return;
        await apiPost(`/pb/board/${activeBoard.id}/container`, { title });
        openBoard(activeBoard.id);
      }}, "Add Container")
    ),
    // Render containers
    (activeBoard.containers || []).map((container: Container) => {
      const isMinimized = minimizedContainers.has(container.id);
      return h("div", {
        key: container.id,
        className: "board-container",
        style: {
          position: "absolute",
          left: `${container.geometry?.x || 50}px`,
          top: `${(container.geometry?.y || 50) + 40}px`,
          width: isMinimized ? "auto" : `${container.geometry?.w || 300}px`,
          minHeight: isMinimized ? "auto" : `${container.geometry?.h || 200}px`,
          border: "2px solid #555", borderRadius: "8px",
          background: "rgba(40,40,40,0.9)", padding: isMinimized ? "0" : "8px",
          cursor: dragging?.id === container.id ? "grabbing" : "default",
        },
      },
        h("div", {
          style: {
            fontWeight: 600, padding: "4px 8px", borderBottom: isMinimized ? "none" : "1px solid #444",
            marginBottom: isMinimized ? "0" : "8px", color: "#7eb8da", cursor: "grab", userSelect: "none",
          },
          onMouseDown: (e: MouseEvent) => handleMouseDown("container", container.id, e),
          onDblClick: () => toggleContainerMinimize(container.id),
        },
          container.title,
          container.agentId ? h("span", { style: { fontSize: "10px", color: "#888", marginLeft: "8px" } }, `\u2192 ${container.agentId}`) : null,
          isMinimized ? h("span", { style: { fontSize: "10px", color: "#666", marginLeft: "8px" } }, "[minimized]") : null
        )
      );
    }),
    // Render cards
    (activeBoard.cards || []).map((card: Card) => {
      const isMinimized = minimizedCards.has(card.id);
      return h("div", {
        key: card.id,
        className: "board-card",
        style: {
          position: "absolute",
          left: `${card.position?.x || 100}px`,
          top: `${(card.position?.y || 100) + 40}px`,
          width: `${card.size?.w || 200}px`,
          minHeight: isMinimized ? "auto" : `${card.size?.h || 80}px`,
          background: cardColors[card.status] || "#444",
          color: "#000", borderRadius: "6px", padding: isMinimized ? "4px 8px" : "8px",
          cursor: dragging?.id === card.id ? "grabbing" : "grab",
          boxShadow: "0 2px 8px rgba(0,0,0,0.3)", userSelect: "none",
        },
        onMouseDown: (e: MouseEvent) => handleMouseDown("card", card.id, e),
      },
        h("div", {
          style: { fontWeight: 600, fontSize: "13px", marginBottom: isMinimized ? "0" : "4px", cursor: "pointer" },
          onDblClick: (e: Event) => { e.stopPropagation(); toggleCardMinimize(card.id); },
        }, card.title),
        isMinimized ? null : h("div", { style: { fontSize: "11px", opacity: 0.7 } }, card.content?.substring(0, 80) || ""),
        isMinimized ? null : h("div", { style: { fontSize: "10px", marginTop: "4px", opacity: 0.5 } }, card.status)
      );
    }),
    // Render edges as SVG
    (activeBoard.edges || []).length > 0
      ? h("svg", { style: { position: "absolute", top: 0, left: 0, width: "100%", height: "100%", pointerEvents: "none" } },
          (activeBoard.edges || []).map((edge: Edge) => {
            const fromCard = (activeBoard.cards || []).find((c: Card) => c.id === edge.from);
            const toCard = (activeBoard.cards || []).find((c: Card) => c.id === edge.to);
            if (!fromCard?.position || !toCard?.position) return null;
            const x1 = (fromCard.position.x || 0) + (fromCard.size?.w || 200) / 2;
            const y1 = (fromCard.position.y || 0) + (fromCard.size?.h || 80) / 2 + 40;
            const x2 = (toCard.position.x || 0) + (toCard.size?.w || 200) / 2;
            const y2 = (toCard.position.y || 0) + (toCard.size?.h || 80) / 2 + 40;
            return h("line", { key: edge.id, x1, y1, x2, y2, stroke: edge.type === "DependsOn" ? "#ff9800" : "#666", strokeWidth: 2 });
          })
        )
      : null
  );
}
