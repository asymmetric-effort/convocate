import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import type { Ticket, Page } from "../../types/api";
import { apiGet, apiPost } from "../../lib/api";

const h = createElement;

export function SupportTool() {
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => { loadTickets(); }, []);

  async function loadTickets() {
    setLoading(true);
    const page = await apiGet<Page<Ticket>>("/sup/ticket?limit=200");
    setTickets(page.items);
    setLoading(false);
  }

  function priorityColor(p: string) {
    return p === "high" ? "#f44336" : p === "medium" ? "#ff9800" : "#4caf50";
  }

  function statusColor(s: string) {
    return s === "open" ? "#2196f3" : s === "in-progress" ? "#ff9800" : s === "resolved" ? "#4caf50" : "#999";
  }

  if (loading) return h("div", { className: "applet-loading" }, "Loading tickets...");

  return h("div", { className: "sup" },
    h("div", { className: "applet-toolbar" },
      h("button", { className: "btn btn-primary", onClick: () => setShowCreate(true) }, "New Ticket"),
      h("span", { className: "applet-count" }, `${tickets.length} ticket${tickets.length !== 1 ? "s" : ""}`)
    ),
    h("div", { className: "grid-list" },
      h("div", { className: "grid-header", style: { gridTemplateColumns: "80px 2fr 100px 100px 1fr" } },
        h("span", null, "ID"), h("span", null, "Subject"), h("span", null, "Priority"), h("span", null, "Status"), h("span", null, "Updated")
      ),
      tickets.map((t: Ticket, i: number) =>
        h("div", { key: t.id, className: `grid-row ${i % 2 === 0 ? "even" : "odd"}`, style: { gridTemplateColumns: "80px 2fr 100px 100px 1fr" } },
          h("span", { className: "cell-id" }, t.id),
          h("span", null, t.subject),
          h("span", null, h("span", { className: "status-dot", style: { backgroundColor: priorityColor(t.priority) } }), " ", t.priority),
          h("span", null, h("span", { className: "status-dot", style: { backgroundColor: statusColor(t.status) } }), " ", t.status),
          h("span", null, t.updatedAt?.split("T")[0] || "")
        )
      )
    ),
    showCreate ? h(NewTicketDialog, { onClose: () => setShowCreate(false), onCreated: () => { setShowCreate(false); loadTickets(); } }) : null
  );
}

function NewTicketDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [subject, setSubject] = useState("");
  const [priority, setPriority] = useState("medium");
  const [body, setBody] = useState("");

  async function handleSubmit(e: Event) {
    e.preventDefault();
    await apiPost("/sup/ticket", { subject, priority, body });
    onCreated();
  }

  return h("div", { className: "modal-overlay", onClick: onClose },
    h("div", { className: "modal", onClick: (e: Event) => e.stopPropagation() },
      h("div", { className: "modal-header" }, h("h2", null, "New Ticket"), h("button", { className: "modal-close", onClick: onClose }, "\u00D7")),
      h("form", { className: "modal-body", onSubmit: handleSubmit },
        h("label", null, "Subject"),
        h("input", { type: "text", value: subject, onInput: (e: any) => setSubject(e.target.value), required: true }),
        h("label", null, "Priority"),
        h("select", { value: priority, onInput: (e: any) => setPriority(e.target.value) },
          h("option", { value: "low" }, "Low"), h("option", { value: "medium" }, "Medium"), h("option", { value: "high" }, "High")
        ),
        h("label", null, "Description"),
        h("textarea", { value: body, onInput: (e: any) => setBody(e.target.value), rows: 5, style: { width: "100%", background: "#1a1a1a", color: "#fff", border: "1px solid #555", borderRadius: "4px", padding: "8px", resize: "vertical" } }),
        h("div", { className: "modal-actions" },
          h("button", { type: "button", className: "btn", onClick: onClose }, "Cancel"),
          h("button", { type: "submit", className: "btn btn-primary" }, "Create Ticket")
        )
      )
    )
  );
}
