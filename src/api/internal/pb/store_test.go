package pb

import "testing"

func TestStoreListBoards(t *testing.T) {
	s := NewStore()
	boards := s.ListBoards()
	if len(boards) != 1 {
		t.Errorf("expected 1 board, got %d", len(boards))
	}
}

func TestStoreGetBoard_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.GetBoard("nonexistent")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreCreateBoard(t *testing.T) {
	s := NewStore()
	b := s.CreateBoard("test-board", "repo-001")
	if b.Name != "test-board" {
		t.Errorf("expected name 'test-board', got %q", b.Name)
	}
	if len(b.Cards) != 0 {
		t.Errorf("expected 0 cards, got %d", len(b.Cards))
	}
}

func TestStoreRenameBoard_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.RenameBoard("nonexistent", "x")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreCreateCard_BoardNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.CreateCard("nonexistent", Card{Title: "x"})
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreGetCard_BoardNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.GetCard("nonexistent", "card-001")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreGetCard_CardNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.GetCard("brd-001", "card-999")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreUpdateCard_BoardNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.UpdateCard("nonexistent", Card{ID: "card-001"})
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreUpdateCard_CardNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.UpdateCard("brd-001", Card{ID: "card-999"})
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreDeleteCard_BoardNotFound(t *testing.T) {
	s := NewStore()
	if s.DeleteCard("nonexistent", "card-001") {
		t.Error("expected false")
	}
}

func TestStoreDeleteCard_CardNotFound(t *testing.T) {
	s := NewStore()
	if s.DeleteCard("brd-001", "card-999") {
		t.Error("expected false")
	}
}

func TestStoreCreateEdge_BoardNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.CreateEdge("nonexistent", Edge{From: "a", To: "b"})
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreCreateEdge_DefaultType(t *testing.T) {
	s := NewStore()
	e, ok := s.CreateEdge("brd-001", Edge{From: "card-001", To: "card-002"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if e.Type != "RelatesTo" {
		t.Errorf("expected default type 'RelatesTo', got %q", e.Type)
	}
}

func TestStoreUpdateEdge_BoardNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.UpdateEdge("nonexistent", "edge-001", "x")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreUpdateEdge_EdgeNotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.UpdateEdge("brd-001", "edge-999", "x")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreDeleteEdge_BoardNotFound(t *testing.T) {
	s := NewStore()
	if s.DeleteEdge("nonexistent", "edge-001") {
		t.Error("expected false")
	}
}

func TestStoreDeleteEdge_EdgeNotFound(t *testing.T) {
	s := NewStore()
	if s.DeleteEdge("brd-001", "edge-999") {
		t.Error("expected false")
	}
}

func TestStoreCreateCard_DefaultStatus(t *testing.T) {
	s := NewStore()
	c, ok := s.CreateCard("brd-001", Card{Title: "test"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if c.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", c.Status)
	}
	if c.Links == nil {
		t.Error("Links should be initialized")
	}
}
