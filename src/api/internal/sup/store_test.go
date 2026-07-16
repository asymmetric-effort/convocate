package sup

import "testing"

func TestStoreCreateAndGet(t *testing.T) {
	s := NewStore()
	ticket := s.CreateTicket(Ticket{Subject: "Test", Body: "body", Priority: "high"})
	if ticket.ID == "" {
		t.Error("expected ID to be set")
	}
	if ticket.Status != "open" {
		t.Errorf("expected status 'open', got %q", ticket.Status)
	}

	got, ok := s.GetTicket(ticket.ID)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Subject != "Test" {
		t.Errorf("expected subject 'Test', got %q", got.Subject)
	}
}

func TestStoreGetTicket_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.GetTicket("nonexistent")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreUpdateTicket(t *testing.T) {
	s := NewStore()
	s.CreateTicket(Ticket{Subject: "Original", Body: "body"})

	updated, ok := s.UpdateTicket("tkt-001", Ticket{Subject: "Updated", Status: "resolved", Priority: "low", Body: "new body"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if updated.Subject != "Updated" {
		t.Errorf("expected subject 'Updated', got %q", updated.Subject)
	}
	if updated.Status != "resolved" {
		t.Errorf("expected status 'resolved', got %q", updated.Status)
	}
	if updated.Priority != "low" {
		t.Errorf("expected priority 'low', got %q", updated.Priority)
	}
	if updated.Body != "new body" {
		t.Errorf("expected body 'new body', got %q", updated.Body)
	}
}

func TestStoreUpdateTicket_PartialFields(t *testing.T) {
	s := NewStore()
	s.CreateTicket(Ticket{Subject: "Original", Body: "body", Priority: "high"})

	// Only update subject, leave others as-is
	updated, ok := s.UpdateTicket("tkt-001", Ticket{Subject: "Changed"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if updated.Subject != "Changed" {
		t.Errorf("expected subject 'Changed', got %q", updated.Subject)
	}
}

func TestStoreUpdateTicket_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.UpdateTicket("nonexistent", Ticket{Subject: "x"})
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreDeleteTicket(t *testing.T) {
	s := NewStore()
	s.CreateTicket(Ticket{Subject: "To delete", Body: "body"})
	if !s.DeleteTicket("tkt-001") {
		t.Error("expected true")
	}
	if s.DeleteTicket("tkt-001") {
		t.Error("expected false after deletion")
	}
}

func TestStoreDeleteTicket_NotFound(t *testing.T) {
	s := NewStore()
	if s.DeleteTicket("nonexistent") {
		t.Error("expected false")
	}
}

func TestStoreListArticles(t *testing.T) {
	s := NewStore()
	articles := s.ListArticles()
	if len(articles) != 3 {
		t.Errorf("expected 3 articles, got %d", len(articles))
	}
}

func TestStoreListTickets(t *testing.T) {
	s := NewStore()
	tickets := s.ListTickets()
	if len(tickets) != 0 {
		t.Errorf("expected 0 tickets, got %d", len(tickets))
	}
	s.CreateTicket(Ticket{Subject: "a"})
	s.CreateTicket(Ticket{Subject: "b"})
	tickets = s.ListTickets()
	if len(tickets) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(tickets))
	}
}
