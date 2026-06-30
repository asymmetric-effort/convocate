/**
 * Support Tool Applet
 *
 * Internal support: ticket list with new ticket composer and
 * documentation browser.
 * Data source: /api/v1/sup/
 */

import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import { Button, Modal, TextField, Spinner, Tag, DataGrid, Tabs } from "@asymmetric-effort/specifyjs/components";
import { useMenuBar } from "./use-menu-bar";

const h = createElement;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Ticket { id: string; subject: string; status: string; priority: string; body: string; updatedAt: string }
interface DocArticle { id: string; title: string; slug: string }

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem("accessToken");
  return { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) };
}

async function fetchTickets(): Promise<Ticket[]> {
  const res = await fetch("/api/v1/sup/ticket?limit=200", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return (await res.json()).items || [];
}

async function createTicket(data: { subject: string; priority: string; body: string }): Promise<Ticket> {
  const res = await fetch("/api/v1/sup/ticket", { method: "POST", headers: authHeaders(), body: JSON.stringify(data) });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return res.json();
}

async function fetchDocs(): Promise<DocArticle[]> {
  const res = await fetch("/api/v1/sup/doc?limit=200", { headers: authHeaders() });
  if (!res.ok) throw new Error(`Failed: ${res.status}`);
  return (await res.json()).items || [];
}

// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------

export function SupportTool({ principal }: { principal?: any } = {}) {
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [docs, setDocs] = useState<DocArticle[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showNewTicket, setShowNewTicket] = useState(false);
  const [selectedTicket, setSelectedTicket] = useState<Ticket | null>(null);

  useMenuBar("sup", [
    { label: "Support", items: [
      { label: "New Ticket", onClick: () => setShowNewTicket(true) },
    ]},
  ]);
  const [newSubject, setNewSubject] = useState("");
  const [newPriority, setNewPriority] = useState("medium");
  const [newBody, setNewBody] = useState("");

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchTickets(), fetchDocs()])
      .then(([t, d]) => { setTickets(t); setDocs(d); })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  const handleCreateTicket = useCallback(async () => {
    if (!newSubject.trim()) { setError("Subject is required."); return; }
    try {
      await createTicket({ subject: newSubject.trim(), priority: newPriority, body: newBody.trim() });
      setNewSubject(""); setNewBody(""); setNewPriority("medium"); setShowNewTicket(false);
      const updated = await fetchTickets();
      setTickets(updated);
    } catch (err: any) { setError(err.message); }
  }, [newSubject, newPriority, newBody]);

  if (loading) {
    return h("div", { style: { display: "flex", alignItems: "center", justifyContent: "center", height: "100%", backgroundColor: "#1e1e1e" }, "data-testid": "support-tool" }, h(Spinner, null));
  }

  const priorityColor = (p: string) => p === "high" ? "red" : p === "medium" ? "#b45309" : "green";
  const statusColor = (s: string) => s === "open" ? "#3b82f6" : s === "in-progress" ? "#b45309" : s === "resolved" ? "green" : "#6b7280";

  // Tickets tab
  const ticketsTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    h("div", { style: { display: "flex", justifyContent: "flex-end", marginBottom: "8px" } },
      h(Button, { variant: "primary" as const, onClick: () => setShowNewTicket(true) }, "New Ticket")
    ),
    tickets.length > 0
      ? h("div", { style: { backgroundColor: "#fff", borderRadius: "4px" } },
          h(DataGrid, {
            columns: [
              { key: "id", header: "ID", width: 100, render: (v: string, row: any) =>
                h("a", { href: "#", style: { color: "#2563eb", textDecoration: "none", cursor: "pointer" },
                  onClick: (e: Event) => { e.preventDefault(); const t = tickets.find((t) => t.id === v); if (t) setSelectedTicket(t); },
                }, v)
              },
              { key: "subject", header: "Subject", width: 250 },
              { key: "priority", header: "Priority", width: 100, render: (v: string) => h(Tag, { label: v, color: priorityColor(v), variant: "solid" as const, size: "sm" as const }) },
              { key: "status", header: "Status", width: 120, render: (v: string) => h(Tag, { label: v, color: statusColor(v), variant: "solid" as const, size: "sm" as const }) },
            ],
            data: tickets.map((t) => ({ id: t.id, subject: t.subject, priority: t.priority, status: t.status })),
            striped: true,
          })
        )
      : h("div", { style: { color: "#aaa", padding: "16px", textAlign: "center" } }, "No tickets")
  );

  // Docs tab
  const docsTab = h("div", { style: { backgroundColor: "#1e1e1e", color: "#e0e0e0", padding: "8px" } },
    docs.length > 0
      ? h("div", { style: { display: "flex", flexDirection: "column", gap: "4px" } },
          ...docs.map((d) =>
            h("div", { key: d.id, style: { padding: "8px 12px", backgroundColor: "#2d2d2d", borderRadius: "4px", cursor: "pointer" } },
              h("div", { style: { fontWeight: "500" } }, d.title),
              h("div", { style: { fontSize: "11px", color: "#aaa" } }, d.slug)
            )
          )
        )
      : h("div", { style: { color: "#aaa", padding: "16px", textAlign: "center" } }, "No documentation articles")
  );

  return h("div", { style: { display: "flex", flexDirection: "column", width: "100%", height: "100%", backgroundColor: "#1e1e1e", color: "#e0e0e0" }, "data-testid": "support-tool" },
    error ? h("div", { style: { padding: "4px 8px", backgroundColor: "#3d1c1c", color: "#ff8888", fontSize: "12px" }, onClick: () => setError("") }, error) : null,
    h(Tabs, { tabs: [
      { id: "tickets", label: "Tickets", content: ticketsTab },
      { id: "docs", label: "Documentation", content: docsTab },
    ] }),
    // New Ticket dialog
    showNewTicket ? h(Modal, { open: true, onClose: () => setShowNewTicket(false), title: "New Ticket", size: "md" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h(TextField, { placeholder: "Subject", value: newSubject, onChange: (v: string) => setNewSubject(v) }),
        h("div", { style: { display: "flex", gap: "8px" } },
          ...["low", "medium", "high"].map((p) =>
            h("button", {
              key: p,
              style: {
                padding: "4px 12px", borderRadius: "4px", border: "none", cursor: "pointer",
                backgroundColor: newPriority === p ? priorityColor(p) : "#333",
                color: "#fff", fontSize: "12px", fontWeight: newPriority === p ? "700" : "400",
              },
              onClick: () => setNewPriority(p),
            }, p)
          )
        ),
        h("textarea", {
          value: newBody,
          onInput: (e: Event) => setNewBody((e.target as HTMLTextAreaElement).value),
          placeholder: "Describe the issue...",
          style: {
            width: "100%", minHeight: "100px", backgroundColor: "#2d2d2d", color: "#e0e0e0",
            border: "1px solid #444", borderRadius: "4px", padding: "8px",
            fontSize: "13px", resize: "vertical",
          },
        }),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setShowNewTicket(false) }, "Cancel"),
          h(Button, { variant: "primary" as const, onClick: handleCreateTicket }, "Submit")
        )
      )
    ) : null,
    // Ticket detail dialog
    selectedTicket ? h(Modal, { open: true, onClose: () => setSelectedTicket(null), title: `Ticket: ${selectedTicket.id}`, size: "md" as const },
      h("div", { style: { display: "flex", flexDirection: "column", gap: "12px", padding: "16px", backgroundColor: "#1e1e1e", color: "#e0e0e0", borderRadius: "0 0 8px 8px" } },
        h("div", { style: { display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px" } },
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "SUBJECT"),
            h("div", null, selectedTicket.subject)
          ),
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "STATUS"),
            h(Tag, { label: selectedTicket.status, color: statusColor(selectedTicket.status), variant: "solid" as const, size: "sm" as const })
          ),
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "PRIORITY"),
            h(Tag, { label: selectedTicket.priority, color: priorityColor(selectedTicket.priority), variant: "solid" as const, size: "sm" as const })
          ),
          h("div", null,
            h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "UPDATED"),
            h("div", null, selectedTicket.updatedAt || "—")
          ),
        ),
        h("div", null,
          h("div", { style: { fontSize: "11px", color: "#aaa", marginBottom: "4px" } }, "BODY"),
          h("div", { style: { padding: "8px", backgroundColor: "#2d2d2d", borderRadius: "4px", fontSize: "13px", whiteSpace: "pre-wrap" } },
            selectedTicket.body || "No description."
          )
        ),
        h("div", { style: { display: "flex", gap: "8px", justifyContent: "flex-end" } },
          h(Button, { variant: "secondary" as const, onClick: () => setSelectedTicket(null) }, "Close")
        )
      )
    ) : null,
  );
}
