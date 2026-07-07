package ide

import "testing"

func TestStoreListProjects(t *testing.T) {
	s := NewStore()
	projects := s.ListProjects()
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}

func TestStoreCreateProject(t *testing.T) {
	s := NewStore()
	p := s.CreateProject("test")
	if p.Name != "test" {
		t.Errorf("expected name 'test', got %q", p.Name)
	}
	if p.ID == "" {
		t.Error("expected ID to be set")
	}
	// Should have a SPECIFICATION.md file
	files := s.ListTree(p.ID)
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestStoreDeleteProject_Happy(t *testing.T) {
	s := NewStore()
	if !s.DeleteProject("prj-001") {
		t.Error("expected true")
	}
	if s.DeleteProject("prj-001") {
		t.Error("expected false after deletion")
	}
}

func TestStoreDeleteProject_NotFound(t *testing.T) {
	s := NewStore()
	if s.DeleteProject("nonexistent") {
		t.Error("expected false")
	}
}

func TestStoreUpdateProject_AllFields(t *testing.T) {
	s := NewStore()
	name := "updated"
	repoID := "repo-new"
	boardID := "brd-new"
	agentID := "agt-new"
	p, ok := s.UpdateProject("prj-001", &name, &repoID, &boardID, &agentID)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", p.Name)
	}
	if p.RepoID != "repo-new" {
		t.Errorf("expected repoId 'repo-new', got %q", p.RepoID)
	}
	if p.BoardID != "brd-new" {
		t.Errorf("expected boardId 'brd-new', got %q", p.BoardID)
	}
	if p.AgentID != "agt-new" {
		t.Errorf("expected agentId 'agt-new', got %q", p.AgentID)
	}
}

func TestStoreUpdateProject_NotFound(t *testing.T) {
	s := NewStore()
	name := "x"
	_, ok := s.UpdateProject("nonexistent", &name, nil, nil, nil)
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreListTree_Empty(t *testing.T) {
	s := NewStore()
	entries := s.ListTree("nonexistent")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestStoreGetFile_NoProject(t *testing.T) {
	s := NewStore()
	_, ok := s.GetFile("nonexistent", "file.txt")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStorePutFile_NewProject(t *testing.T) {
	s := NewStore()
	f := s.PutFile("new-project", "file.txt", "content")
	if f.Content != "content" {
		t.Errorf("expected content 'content', got %q", f.Content)
	}
}

func TestStoreDeleteFile_NoProject(t *testing.T) {
	s := NewStore()
	if s.DeleteFile("nonexistent", "file.txt") {
		t.Error("expected false")
	}
}

func TestStoreDeleteFile_NoFile(t *testing.T) {
	s := NewStore()
	if s.DeleteFile("prj-001", "nonexistent.txt") {
		t.Error("expected false")
	}
}

func TestStoreRenameFile_NoProject(t *testing.T) {
	s := NewStore()
	_, ok := s.RenameFile("nonexistent", "a.txt", "b.txt")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreRenameFile_NoSourceFile(t *testing.T) {
	s := NewStore()
	_, ok := s.RenameFile("prj-001", "nonexistent.txt", "b.txt")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreRenameFile_DestExists(t *testing.T) {
	s := NewStore()
	_, ok := s.RenameFile("prj-001", "src/main.ts", "SPECIFICATION.md")
	if ok {
		t.Error("expected ok=false when destination exists")
	}
}
