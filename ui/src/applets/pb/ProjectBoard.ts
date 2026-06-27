import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import type { Board, BoardSummary, Card, Container, Edge, Page } from "../../types/api";
import { apiGet, apiPost, apiPatch, apiDelete } from "../../lib/api";

const h = createElement;

export function ProjectBoard() {
  const [boards, setBoards] = useState<BoardSummary[]>([]);
  const [activeBoard, setActiveBoard] = useState<Board | null>(null);
  const [loading, setLoading] = useState(true);

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
  }

  async function createBoard() {
    const name = prompt("Board name:");
    if (!name) return;
    const board = await apiPost<Board>("/pb/board", { name });
    setActiveBoard(board);
    loadBoards();
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
            h("span", null, b.name), h("span", { className: "cell-id" }, b.repoId || "—"), h("span", null, b.updatedAt?.split("T")[0] || "")
          )
        )
      )
    );
  }

  const cardColors: Record<string, string> = { todo: "#ffd54f", active: "#42a5f5", done: "#66bb6a", fail: "#ef5350", note: "#bdbdbd" };

  return h("div", { className: "pb-canvas", style: { position: "relative", width: "100%", height: "100%", overflow: "auto", background: "#1a1a1a" } },
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
    (activeBoard.containers || []).map((container: Container) =>
      h("div", {
        key: container.id,
        className: "board-container",
        style: {
          position: "absolute",
          left: `${container.geometry?.x || 50}px`,
          top: `${(container.geometry?.y || 50) + 40}px`,
          width: `${container.geometry?.w || 300}px`,
          minHeight: `${container.geometry?.h || 200}px`,
          border: "2px solid #555", borderRadius: "8px", background: "rgba(40,40,40,0.9)", padding: "8px"
        }
      },
        h("div", { style: { fontWeight: 600, padding: "4px 8px", borderBottom: "1px solid #444", marginBottom: "8px", color: "#7eb8da" } },
          container.title, container.agentId ? h("span", { style: { fontSize: "10px", color: "#888", marginLeft: "8px" } }, `\u2192 ${container.agentId}`) : null
        )
      )
    ),
    // Render cards
    (activeBoard.cards || []).map((card: Card) =>
      h("div", {
        key: card.id,
        className: "board-card",
        style: {
          position: "absolute",
          left: `${card.position?.x || 100}px`,
          top: `${(card.position?.y || 100) + 40}px`,
          width: `${card.size?.w || 200}px`,
          minHeight: `${card.size?.h || 80}px`,
          background: cardColors[card.status] || "#444",
          color: "#000", borderRadius: "6px", padding: "8px", cursor: "pointer",
          boxShadow: "0 2px 8px rgba(0,0,0,0.3)"
        }
      },
        h("div", { style: { fontWeight: 600, fontSize: "13px", marginBottom: "4px" } }, card.title),
        h("div", { style: { fontSize: "11px", opacity: 0.7 } }, card.content?.substring(0, 80) || ""),
        h("div", { style: { fontSize: "10px", marginTop: "4px", opacity: 0.5 } }, card.status)
      )
    ),
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
            return h("line", { key: edge.id, x1, y1, x2, y2, stroke: edge.type === "DependsOn" ? "#ff9800" : "#666", strokeWidth: 2, markerEnd: "url(#arrow)" });
          })
        )
      : null
  );
}
