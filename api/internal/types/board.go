package types

// BoardSummary is a lightweight representation of a project board.
type BoardSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	RepoID    string `json:"repoId,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// Board is a full project board with cards and edges.
// Each project has exactly one agent-container (managed by Agent Manager),
// not multiple containers on the board.
type Board struct {
	BoardSummary
	Cards []Card `json:"cards"`
	Edges []Edge `json:"edges"`
}

// Geometry represents position and size of a board element.
type Geometry struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Position represents a 2D coordinate.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Size represents width and height.
type Size struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Card represents a task on a project board.
type Card struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     CardStatus `json:"status"`
	Content    string     `json:"content"`
	Position   *Position  `json:"position,omitempty"`
	Size       *Size      `json:"size,omitempty"`
	SourceRefs []string   `json:"sourceRefs,omitempty"`
	Note       *string    `json:"note"`
	Links      []Edge     `json:"links"`
}

// CardInput is the write model for card creation.
type CardInput struct {
	Title    string     `json:"title"`
	Content  string     `json:"content,omitempty"`
	Status   CardStatus `json:"status,omitempty"`
	Position *Position  `json:"position,omitempty"`
	Size     *Size      `json:"size,omitempty"`
}

// Edge represents a typed link between two cards.
type Edge struct {
	ID   string   `json:"id"`
	Type EdgeType `json:"type"`
	From string   `json:"from"`
	To   string   `json:"to"`
}

// EdgeInput is the write model for edge creation.
type EdgeInput struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Type EdgeType `json:"type,omitempty"`
}

// ExecutionRun represents an in-progress board implementation.
type ExecutionRun struct {
	ID              string   `json:"id"`
	BoardID         string   `json:"boardId"`
	DispatchedCards []string `json:"dispatchedCards"`
	PullRequestID   *string  `json:"pullRequestId"`
	StartedAt       string   `json:"startedAt"`
}
