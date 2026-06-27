package types

// Repo represents a git repository.
type Repo struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	DefaultBranch string     `json:"defaultBranch"`
	Visibility    Visibility `json:"visibility"`
	UpdatedAt     string     `json:"updatedAt"`
}

// RepoFile represents a file or directory in a repository.
type RepoFile struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int    `json:"size"`
	Path string `json:"path"`
}

// PullRequest represents a pull request.
type PullRequest struct {
	ID           string   `json:"id"`
	RepoID       string   `json:"repoId"`
	Title        string   `json:"title"`
	Branch       string   `json:"branch"`
	TargetBranch string   `json:"targetBranch"`
	Status       PrStatus `json:"status"`
	Author       string   `json:"author"`
	Files        []string `json:"files"`
	Checks       []Check  `json:"checks"`
}

// Check represents a CI check on a pull request.
type Check struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
}

// Project represents an IDE project.
type Project struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	RepoID              string `json:"repoId"`
	SpecificationFileID string `json:"specificationFileId"`
	BoardID             string `json:"boardId,omitempty"`
}

// FileContent represents a file's content and metadata.
type FileContent struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Language  string `json:"language,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}
