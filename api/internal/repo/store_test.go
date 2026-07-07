package repo

import "testing"

func TestStoreListRepos(t *testing.T) {
	s := NewStore()
	repos := s.ListRepos()
	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos))
	}
}

func TestStoreCreateRepo(t *testing.T) {
	s := NewStore()
	r := s.CreateRepo("test", "private")
	if r.Name != "test" {
		t.Errorf("expected name 'test', got %q", r.Name)
	}
	if r.DefaultBranch != "main" {
		t.Errorf("expected default branch 'main', got %q", r.DefaultBranch)
	}
}

func TestStoreListFiles(t *testing.T) {
	s := NewStore()
	files := s.ListFiles("repo-001")
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestStoreListPRs_Happy(t *testing.T) {
	s := NewStore()
	prs := s.ListPRs("repo-001")
	if len(prs) != 1 {
		t.Errorf("expected 1 PR, got %d", len(prs))
	}
}

func TestStoreListPRs_Empty(t *testing.T) {
	s := NewStore()
	prs := s.ListPRs("repo-999")
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}

func TestStoreGetPR_Happy(t *testing.T) {
	s := NewStore()
	pr, ok := s.GetPR("repo-001", "pr-001")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if pr.Title != "Add user auth" {
		t.Errorf("expected title 'Add user auth', got %q", pr.Title)
	}
}

func TestStoreGetPR_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.GetPR("repo-001", "pr-999")
	if ok {
		t.Error("expected ok=false")
	}
}

func TestStoreMergePR_Happy(t *testing.T) {
	s := NewStore()
	pr, ok := s.MergePR("repo-001", "pr-001")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if pr.Status != "merged" {
		t.Errorf("expected status 'merged', got %q", pr.Status)
	}
}

func TestStoreMergePR_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.MergePR("repo-001", "pr-999")
	if ok {
		t.Error("expected ok=false")
	}
}
