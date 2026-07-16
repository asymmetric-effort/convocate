package sup

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

func newAuthRequest(method, path string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{
		ID:       "usr-001",
		Username: "testuser",
		Name:     "Test User",
		Roles:    []string{"admin"},
	})
	return req.WithContext(ctx)
}

func newHandler() *Handler {
	return &Handler{store: NewStore()}
}

func TestListTickets_Empty(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/sup/ticket", nil)
	rec := httptest.NewRecorder()
	h.listTickets(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 0 {
		t.Errorf("expected 0 total, got %d", page.Total)
	}
}

func TestCreateTicket_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/sup/ticket", Ticket{
		Subject:  "Test ticket",
		Priority: "high",
		Body:     "Something broke",
	})
	rec := httptest.NewRecorder()
	h.createTicket(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var ticket Ticket
	json.NewDecoder(rec.Body).Decode(&ticket)
	if ticket.ID == "" {
		t.Error("ticket ID should not be empty")
	}
	if ticket.Status != "open" {
		t.Errorf("expected status open, got %q", ticket.Status)
	}
	if ticket.Subject != "Test ticket" {
		t.Errorf("expected subject 'Test ticket', got %q", ticket.Subject)
	}
}

func TestCreateTicket_SetsReporter(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/sup/ticket", Ticket{
		Subject: "Bug report",
		Body:    "Details",
	})
	rec := httptest.NewRecorder()
	h.createTicket(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var ticket Ticket
	json.NewDecoder(rec.Body).Decode(&ticket)
	if ticket.Reporter != "testuser" {
		t.Errorf("expected reporter 'testuser', got %q", ticket.Reporter)
	}
}

func TestCreateTicket_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("POST", "/api/v1/sup/ticket", bytes.NewReader([]byte("not json")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{
		Roles: []string{"admin"},
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createTicket(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetTicket_Happy(t *testing.T) {
	h := newHandler()
	h.store.CreateTicket(Ticket{Subject: "Existing", Body: "body"})

	req := newAuthRequest("GET", "/api/v1/sup/ticket/tkt-001", nil)
	req.SetPathValue("ticketId", "tkt-001")
	rec := httptest.NewRecorder()
	h.getTicket(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetTicket_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/sup/ticket/nonexistent", nil)
	req.SetPathValue("ticketId", "nonexistent")
	rec := httptest.NewRecorder()
	h.getTicket(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateTicket_Happy(t *testing.T) {
	h := newHandler()
	h.store.CreateTicket(Ticket{Subject: "Original", Body: "body"})

	req := newAuthRequest("PATCH", "/api/v1/sup/ticket/tkt-001", Ticket{Subject: "Updated"})
	req.SetPathValue("ticketId", "tkt-001")
	rec := httptest.NewRecorder()
	h.updateTicket(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var ticket Ticket
	json.NewDecoder(rec.Body).Decode(&ticket)
	if ticket.Subject != "Updated" {
		t.Errorf("expected subject 'Updated', got %q", ticket.Subject)
	}
}

func TestUpdateTicket_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PATCH", "/api/v1/sup/ticket/nonexistent", Ticket{Subject: "x"})
	req.SetPathValue("ticketId", "nonexistent")
	rec := httptest.NewRecorder()
	h.updateTicket(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateTicket_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("PATCH", "/api/v1/sup/ticket/tkt-001", bytes.NewReader([]byte("bad")))
	req.SetPathValue("ticketId", "tkt-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.updateTicket(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteTicket_Happy(t *testing.T) {
	h := newHandler()
	h.store.CreateTicket(Ticket{Subject: "To delete", Body: "body"})

	req := newAuthRequest("DELETE", "/api/v1/sup/ticket/tkt-001", nil)
	req.SetPathValue("ticketId", "tkt-001")
	rec := httptest.NewRecorder()
	h.deleteTicket(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestDeleteTicket_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/sup/ticket/nonexistent", nil)
	req.SetPathValue("ticketId", "nonexistent")
	rec := httptest.NewRecorder()
	h.deleteTicket(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	// Test through registered routes using mock auth
	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/api/v1/sup/ticket", http.StatusUnauthorized},
		{"POST", "/api/v1/sup/ticket", http.StatusUnauthorized},
		{"GET", "/api/v1/sup/doc", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tt.method, tt.path, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != tt.status {
			t.Errorf("%s %s: expected %d, got %d", tt.method, tt.path, tt.status, rec.Code)
		}
	}
}

func TestListDocs(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/sup/doc", nil)
	rec := httptest.NewRecorder()
	h.listDocs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 3 {
		t.Errorf("expected 3 docs, got %d", page.Total)
	}
}
