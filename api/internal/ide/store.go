package ide

import (
	"fmt"
	"sync"
	"time"
)

type Project struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	RepoID              string `json:"repoId"`
	SpecificationFileID string `json:"specificationFileId"`
	BoardID             string `json:"boardId,omitempty"`
}

type FileContent struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Language  string `json:"language,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int    `json:"size"`
	Path string `json:"path"`
}

type Store struct {
	mu       sync.Mutex
	projects []Project
	files    map[string]map[string]FileContent // projectID -> path -> content
}

func NewStore() *Store {
	now := time.Now().UTC().Format(time.RFC3339)
	return &Store{
		projects: []Project{
			{ID: "prj-001", Name: "demo-app", RepoID: "repo-001", SpecificationFileID: "SPECIFICATION.md"},
		},
		files: map[string]map[string]FileContent{
			"prj-001": {
				"SPECIFICATION.md": {Path: "SPECIFICATION.md", Content: "# Demo App\n\nA sample application.", Language: "markdown", UpdatedAt: now},
				"src/main.ts":      {Path: "src/main.ts", Content: "console.log('hello');", Language: "typescript", UpdatedAt: now},
			},
		},
	}
}

func (s *Store) ListProjects() []Project { s.mu.Lock(); defer s.mu.Unlock(); o := make([]Project, len(s.projects)); copy(o, s.projects); return o }

func (s *Store) CreateProject(name string) Project {
	s.mu.Lock(); defer s.mu.Unlock()
	p := Project{ID: fmt.Sprintf("prj-%03d", len(s.projects)+1), Name: name, RepoID: fmt.Sprintf("repo-%03d", len(s.projects)+1), SpecificationFileID: "SPECIFICATION.md"}
	s.projects = append(s.projects, p)
	now := time.Now().UTC().Format(time.RFC3339)
	s.files[p.ID] = map[string]FileContent{
		"SPECIFICATION.md": {Path: "SPECIFICATION.md", Content: fmt.Sprintf("# %s\n\nDescribe your application here.", name), Language: "markdown", UpdatedAt: now},
	}
	return p
}

func (s *Store) ListTree(projectID string) []FileEntry {
	s.mu.Lock(); defer s.mu.Unlock()
	files := s.files[projectID]
	var entries []FileEntry
	for path, f := range files {
		entries = append(entries, FileEntry{Name: path, Type: "file", Size: len(f.Content), Path: path})
	}
	if entries == nil { entries = []FileEntry{} }
	return entries
}

func (s *Store) GetFile(projectID, path string) (FileContent, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	files := s.files[projectID]
	if files == nil { return FileContent{}, false }
	f, ok := files[path]
	return f, ok
}

func (s *Store) PutFile(projectID, path, content string) FileContent {
	s.mu.Lock(); defer s.mu.Unlock()
	if s.files[projectID] == nil { s.files[projectID] = make(map[string]FileContent) }
	f := FileContent{Path: path, Content: content, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	s.files[projectID][path] = f
	return f
}

func (s *Store) DeleteFile(projectID, path string) bool {
	s.mu.Lock(); defer s.mu.Unlock()
	files := s.files[projectID]
	if files == nil { return false }
	if _, ok := files[path]; !ok { return false }
	delete(files, path)
	return true
}

func (s *Store) RenameFile(projectID, oldPath, newPath string) (FileContent, bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	files := s.files[projectID]
	if files == nil { return FileContent{}, false }
	f, ok := files[oldPath]
	if !ok { return FileContent{}, false }
	if _, exists := files[newPath]; exists { return FileContent{}, false }
	delete(files, oldPath)
	f.Path = newPath
	f.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	files[newPath] = f
	return f, true
}
