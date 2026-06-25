package sup

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type Handler struct{ store *Store }

func Register(mux *http.ServeMux) {
	h := &Handler{store: NewStore()}
	auth := middleware.Auth
	view := middleware.RBAC("support-view")

	mux.Handle("GET /api/v1/sup/ticket", middleware.Chain(http.HandlerFunc(h.listTickets), auth, view))
	mux.Handle("POST /api/v1/sup/ticket", middleware.Chain(http.HandlerFunc(h.createTicket), auth, view))
	mux.Handle("GET /api/v1/sup/ticket/{ticketId}", middleware.Chain(http.HandlerFunc(h.getTicket), auth, view))
	mux.Handle("PATCH /api/v1/sup/ticket/{ticketId}", middleware.Chain(http.HandlerFunc(h.updateTicket), auth, view))
	mux.Handle("GET /api/v1/sup/doc", middleware.Chain(http.HandlerFunc(h.listDocs), auth, view))
}

func (h *Handler) listTickets(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListTickets(), offset, limit))
}

func (h *Handler) createTicket(w http.ResponseWriter, r *http.Request) {
	var req Ticket
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, h.store.CreateTicket(req))
}

func (h *Handler) getTicket(w http.ResponseWriter, r *http.Request) {
	t, ok := h.store.GetTicket(r.PathValue("ticketId"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "ticket not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) updateTicket(w http.ResponseWriter, r *http.Request) {
	var req Ticket
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	t, ok := h.store.UpdateTicket(r.PathValue("ticketId"), req)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "ticket not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) listDocs(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListArticles(), offset, limit))
}
