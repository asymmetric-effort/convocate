package pb

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
		ID: "usr-001", Username: "testuser", Roles: []string{"admin"},
	})
	return req.WithContext(ctx)
}

func newHandler() *Handler {
	return &Handler{store: NewStore()}
}

func TestListBoards(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/pb/board", nil)
	rec := httptest.NewRecorder()
	h.listBoards(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 1 {
		t.Errorf("expected 1 board, got %d", page.Total)
	}
}

func TestCreateBoard(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board", map[string]string{
		"name": "New Board", "repoId": "repo-001",
	})
	rec := httptest.NewRecorder()
	h.createBoard(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var b Board
	json.NewDecoder(rec.Body).Decode(&b)
	if b.Name != "New Board" {
		t.Errorf("expected name 'New Board', got %q", b.Name)
	}
}

func TestCreateBoard_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("POST", "/api/v1/pb/board", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createBoard(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetBoard_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/pb/board/brd-001", nil)
	req.SetPathValue("boardId", "brd-001")
	rec := httptest.NewRecorder()
	h.getBoard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var b Board
	json.NewDecoder(rec.Body).Decode(&b)
	if b.Name != "Demo Project" {
		t.Errorf("expected name 'Demo Project', got %q", b.Name)
	}
}

func TestGetBoard_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/pb/board/brd-999", nil)
	req.SetPathValue("boardId", "brd-999")
	rec := httptest.NewRecorder()
	h.getBoard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRenameBoard_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PATCH", "/api/v1/pb/board/brd-001", map[string]string{"name": "Renamed"})
	req.SetPathValue("boardId", "brd-001")
	rec := httptest.NewRecorder()
	h.renameBoard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var b Board
	json.NewDecoder(rec.Body).Decode(&b)
	if b.Name != "Renamed" {
		t.Errorf("expected name 'Renamed', got %q", b.Name)
	}
}

func TestRenameBoard_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PATCH", "/api/v1/pb/board/brd-999", map[string]string{"name": "x"})
	req.SetPathValue("boardId", "brd-999")
	rec := httptest.NewRecorder()
	h.renameBoard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRenameBoard_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("PATCH", "/api/v1/pb/board/brd-001", bytes.NewReader([]byte("bad")))
	req.SetPathValue("boardId", "brd-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.renameBoard(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSaveAsRepo(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-001/save-as-repo", nil)
	req.SetPathValue("boardId", "brd-001")
	rec := httptest.NewRecorder()
	h.saveAsRepo(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestImplement(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-001/implement", nil)
	req.SetPathValue("boardId", "brd-001")
	rec := httptest.NewRecorder()
	h.implement(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	var run ExecutionRun
	json.NewDecoder(rec.Body).Decode(&run)
	if run.BoardID != "brd-001" {
		t.Errorf("expected boardId 'brd-001', got %q", run.BoardID)
	}
}

func TestCreateCard_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-001/card", Card{
		Title:   "New Card",
		Content: "Do something",
	})
	req.SetPathValue("boardId", "brd-001")
	rec := httptest.NewRecorder()
	h.createCard(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var c Card
	json.NewDecoder(rec.Body).Decode(&c)
	if c.Title != "New Card" {
		t.Errorf("expected title 'New Card', got %q", c.Title)
	}
	if c.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", c.Status)
	}
}

func TestCreateCard_BoardNotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-999/card", Card{Title: "x"})
	req.SetPathValue("boardId", "brd-999")
	rec := httptest.NewRecorder()
	h.createCard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateCard_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("POST", "/api/v1/pb/board/brd-001/card", bytes.NewReader([]byte("bad")))
	req.SetPathValue("boardId", "brd-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createCard(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetCard_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/pb/board/brd-001/card/card-001", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-001")
	rec := httptest.NewRecorder()
	h.getCard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetCard_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/pb/board/brd-001/card/card-999", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-999")
	rec := httptest.NewRecorder()
	h.getCard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateCard_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PUT", "/api/v1/pb/board/brd-001/card/card-001", Card{
		Title: "Updated Card", Status: "active", Content: "Updated content",
	})
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-001")
	rec := httptest.NewRecorder()
	h.updateCard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var c Card
	json.NewDecoder(rec.Body).Decode(&c)
	if c.Title != "Updated Card" {
		t.Errorf("expected title 'Updated Card', got %q", c.Title)
	}
}

func TestUpdateCard_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PUT", "/api/v1/pb/board/brd-001/card/card-999", Card{Title: "x"})
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-999")
	rec := httptest.NewRecorder()
	h.updateCard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateCard_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("PUT", "/api/v1/pb/board/brd-001/card/card-001", bytes.NewReader([]byte("bad")))
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.updateCard(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteCard_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/pb/board/brd-001/card/card-001", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-001")
	rec := httptest.NewRecorder()
	h.deleteCard(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestDeleteCard_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/pb/board/brd-001/card/card-999", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-999")
	rec := httptest.NewRecorder()
	h.deleteCard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSendCard_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-001/card/card-001/send", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-001")
	rec := httptest.NewRecorder()
	h.sendCard(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	var c Card
	json.NewDecoder(rec.Body).Decode(&c)
	if c.Status != "active" {
		t.Errorf("expected status 'active', got %q", c.Status)
	}
}

func TestSendCard_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-001/card/card-999/send", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("cardId", "card-999")
	rec := httptest.NewRecorder()
	h.sendCard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateEdge_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-001/edge", Edge{
		From: "card-001", To: "card-002", Type: "DependsOn",
	})
	req.SetPathValue("boardId", "brd-001")
	rec := httptest.NewRecorder()
	h.createEdge(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var e Edge
	json.NewDecoder(rec.Body).Decode(&e)
	if e.ID == "" {
		t.Error("edge ID should not be empty")
	}
}

func TestCreateEdge_BoardNotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/pb/board/brd-999/edge", Edge{From: "a", To: "b"})
	req.SetPathValue("boardId", "brd-999")
	rec := httptest.NewRecorder()
	h.createEdge(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateEdge_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("POST", "/api/v1/pb/board/brd-001/edge", bytes.NewReader([]byte("bad")))
	req.SetPathValue("boardId", "brd-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createEdge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateEdge_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PATCH", "/api/v1/pb/board/brd-001/edge/edge-001", map[string]string{"type": "RelatesTo"})
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("edgeId", "edge-001")
	rec := httptest.NewRecorder()
	h.updateEdge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var e Edge
	json.NewDecoder(rec.Body).Decode(&e)
	if e.Type != "RelatesTo" {
		t.Errorf("expected type 'RelatesTo', got %q", e.Type)
	}
}

func TestUpdateEdge_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PATCH", "/api/v1/pb/board/brd-001/edge/edge-999", map[string]string{"type": "x"})
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("edgeId", "edge-999")
	rec := httptest.NewRecorder()
	h.updateEdge(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateEdge_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("PATCH", "/api/v1/pb/board/brd-001/edge/edge-001", bytes.NewReader([]byte("bad")))
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("edgeId", "edge-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.updateEdge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteEdge_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/pb/board/brd-001/edge/edge-001", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("edgeId", "edge-001")
	rec := httptest.NewRecorder()
	h.deleteEdge(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/pb/board", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestDeleteEdge_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/pb/board/brd-001/edge/edge-999", nil)
	req.SetPathValue("boardId", "brd-001")
	req.SetPathValue("edgeId", "edge-999")
	rec := httptest.NewRecorder()
	h.deleteEdge(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
