package amgr

import "testing"

func TestStoreList(t *testing.T) {
	s := NewStore()
	agents := s.List()
	if len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(agents))
	}
}

func TestStoreGet_Happy(t *testing.T) {
	s := NewStore()
	a, ok := s.Get("agt-7f3a-01")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if a.Project != "demo-app" {
		t.Errorf("expected project 'demo-app', got %q", a.Project)
	}
}

func TestStoreGet_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreCreate(t *testing.T) {
	s := NewStore()
	a := s.Create(Agent{Project: "test", NodeID: "n1"})
	if a.ID == "" {
		t.Error("expected ID to be set")
	}
	if a.Status != "running" {
		t.Errorf("expected status 'running', got %q", a.Status)
	}
}

func TestStoreUpdate_Happy(t *testing.T) {
	s := NewStore()
	project := "updated"
	a, ok := s.Update("agt-7f3a-01", &project, nil, nil)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if a.Project != "updated" {
		t.Errorf("expected project 'updated', got %q", a.Project)
	}
}

func TestStoreUpdate_AllFields(t *testing.T) {
	s := NewStore()
	project := "updated"
	nodeID := "n2"
	expose := "new.local:3000"
	a, ok := s.Update("agt-7f3a-01", &project, &nodeID, &expose)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if a.NodeID != "n2" {
		t.Errorf("expected nodeId 'n2', got %q", a.NodeID)
	}
	if a.Expose != "new.local:3000" {
		t.Errorf("expected expose 'new.local:3000', got %q", a.Expose)
	}
}

func TestStoreUpdate_NotFound(t *testing.T) {
	s := NewStore()
	p := "x"
	_, ok := s.Update("nonexistent", &p, nil, nil)
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreDelete_Happy(t *testing.T) {
	s := NewStore()
	if !s.Delete("agt-7f3a-01") {
		t.Error("expected true")
	}
	if s.Delete("agt-7f3a-01") {
		t.Error("expected false after deletion")
	}
}

func TestStoreDelete_NotFound(t *testing.T) {
	s := NewStore()
	if s.Delete("nonexistent") {
		t.Error("expected false")
	}
}

func TestStoreSetStatus_Happy(t *testing.T) {
	s := NewStore()
	if !s.SetStatus("agt-7f3a-01", "stopped") {
		t.Error("expected true")
	}
	a, _ := s.Get("agt-7f3a-01")
	if a.Status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", a.Status)
	}
}

func TestStoreSetStatus_NotFound(t *testing.T) {
	s := NewStore()
	if s.SetStatus("nonexistent", "stopped") {
		t.Error("expected false")
	}
}
