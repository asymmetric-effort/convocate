package repo

import (
	"fmt"
	"sync"
	"time"
)

type Repo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"defaultBranch"`
	Visibility    string `json:"visibility"`
	UpdatedAt     string `json:"updatedAt"`
}

type RepoFile struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int    `json:"size"`
	Path string `json:"path"`
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type PullRequest struct {
	ID           string  `json:"id"`
	RepoID       string  `json:"repoId"`
	Title        string  `json:"title"`
	Branch       string  `json:"branch"`
	TargetBranch string  `json:"targetBranch"`
	Status       string  `json:"status"`
	Author       string  `json:"author"`
	Files        []string `json:"files"`
	Checks       []Check `json:"checks"`
}

type Store struct {
	mu    sync.Mutex
	repos []Repo
	prs   []PullRequest
}

func NewStore() *Store {
	now := time.Now().UTC().Format(time.RFC3339)
	return &Store{
		repos: []Repo{
			{ID: "repo-001", Name: "demo-app", Description: "Demo application", DefaultBranch: "main", Visibility: "private", UpdatedAt: now},
			{ID: "repo-002", Name: "api-svc", Description: "API service", DefaultBranch: "main", Visibility: "private", UpdatedAt: now},
		},
		prs: []PullRequest{
			{ID: "pr-001", RepoID: "repo-001", Title: "Add user auth", Branch: "feature/auth", TargetBranch: "main", Status: "open", Author: "admin", Files: []string{"src/auth.ts", "src/login.ts"}, Checks: []Check{{Name: "ci/build", Status: "passing"}, {Name: "ci/test", Status: "passing"}}},
		},
	}
}

func (s *Store) ListRepos() []Repo { s.mu.Lock(); defer s.mu.Unlock(); o := make([]Repo, len(s.repos)); copy(o, s.repos); return o }

func (s *Store) CreateRepo(name, visibility string) Repo {
	s.mu.Lock(); defer s.mu.Unlock()
	r := Repo{ID: fmt.Sprintf("repo-%03d", len(s.repos)+1), Name: name, DefaultBranch: "main", Visibility: visibility, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	s.repos = append(s.repos, r)
	return r
}

func (s *Store) ListFiles(repoID string) []RepoFile {
	return []RepoFile{
		{Name: "README.md", Type: "file", Size: 1024, Path: "README.md"},
		{Name: "src", Type: "dir", Size: 0, Path: "src"},
		{Name: "SPECIFICATION.md", Type: "file", Size: 4096, Path: "SPECIFICATION.md"},
	}
}

func (s *Store) ListPRs(repoID string) []PullRequest {
	s.mu.Lock(); defer s.mu.Unlock()
	var out []PullRequest
	for _, pr := range s.prs { if pr.RepoID == repoID { out = append(out, pr) } }
	if out == nil { out = []PullRequest{} }
	return out
}

func (s *Store) GetPR(repoID, prID string) (PullRequest, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	for _, pr := range s.prs { if pr.RepoID == repoID && pr.ID == prID { return pr, true } }
	return PullRequest{}, false
}

func (s *Store) MergePR(repoID, prID string) (PullRequest, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	for i, pr := range s.prs {
		if pr.RepoID == repoID && pr.ID == prID {
			s.prs[i].Status = "merged"
			return s.prs[i], true
		}
	}
	return PullRequest{}, false
}
