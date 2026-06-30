package pb

import (
	"fmt"
	"sync"
	"time"
)

type Geometry struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Size struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type BoardSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	RepoID    string `json:"repoId,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

type Board struct {
	BoardSummary
	Containers []Container `json:"containers"`
	Cards      []Card      `json:"cards"`
	Edges      []Edge      `json:"edges"`
}

type Container struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	AgentID   *string   `json:"agentId"`
	Minimized bool      `json:"minimized"`
	Geometry  *Geometry `json:"geometry,omitempty"`
}

type Card struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	Content     string    `json:"content"`
	ContainerID *string   `json:"containerId"`
	Position    *Position `json:"position,omitempty"`
	Size        *Size     `json:"size,omitempty"`
	SourceRefs  []string  `json:"sourceRefs,omitempty"`
	Note        *string   `json:"note"`
	Links       []Edge    `json:"links"`
}

type Edge struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	From string `json:"from"`
	To   string `json:"to"`
}

type ExecutionRun struct {
	ID              string   `json:"id"`
	BoardID         string   `json:"boardId"`
	DispatchedCards []string `json:"dispatchedCards"`
	PullRequestID   *string  `json:"pullRequestId"`
	StartedAt       string   `json:"startedAt"`
}

type Store struct {
	mu        sync.Mutex
	boards    []Board
	nextCardN int
	nextContN int
	nextEdgeN int
}

func NewStore() *Store {
	agentID := "agt-7f3a-01"
	contID := "cont-001"
	return &Store{
		boards: []Board{{
			BoardSummary: BoardSummary{ID: "brd-001", Name: "Demo Project", RepoID: "repo-001", UpdatedAt: time.Now().UTC().Format(time.RFC3339)},
			Containers: []Container{
				{ID: contID, Title: "Backend Tasks", AgentID: &agentID, Geometry: &Geometry{X: 50, Y: 50, W: 400, H: 300}},
			},
			Cards: []Card{
				{ID: "card-001", Title: "Set up database schema", Status: "todo", Content: "Create initial PostgreSQL schema", ContainerID: &contID, Position: &Position{X: 70, Y: 80}, Size: &Size{W: 200, H: 120}, Links: []Edge{}},
				{ID: "card-002", Title: "Implement auth endpoints", Status: "todo", Content: "JWT login, refresh, logout", ContainerID: &contID, Position: &Position{X: 70, Y: 220}, Size: &Size{W: 200, H: 120}, Links: []Edge{}},
			},
			Edges: []Edge{
				{ID: "edge-001", Type: "DependsOn", From: "card-002", To: "card-001"},
			},
		}},
		nextCardN: 3,
		nextContN: 2,
		nextEdgeN: 2,
	}
}

func (s *Store) ListBoards() []BoardSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]BoardSummary, len(s.boards))
	for i, b := range s.boards {
		out[i] = b.BoardSummary
	}
	return out
}

func (s *Store) GetBoard(id string) (Board, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.boards {
		if b.ID == id {
			return b, true
		}
	}
	return Board{}, false
}

func (s *Store) CreateBoard(name, repoID string) Board {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := Board{BoardSummary: BoardSummary{ID: fmt.Sprintf("brd-%03d", len(s.boards)+1), Name: name, RepoID: repoID, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}, Containers: []Container{}, Cards: []Card{}, Edges: []Edge{}}
	s.boards = append(s.boards, b)
	return b
}

func (s *Store) RenameBoard(id, name string) (Board, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.boards {
		if b.ID == id {
			s.boards[i].Name = name
			s.boards[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return s.boards[i], true
		}
	}
	return Board{}, false
}

func (s *Store) boardIndex(id string) int {
	for i, b := range s.boards {
		if b.ID == id {
			return i
		}
	}
	return -1
}

func (s *Store) CreateContainer(boardID string, c Container) (Container, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.boardIndex(boardID)
	if i < 0 {
		return Container{}, false
	}
	c.ID = fmt.Sprintf("cont-%03d", s.nextContN)
	s.nextContN++
	s.boards[i].Containers = append(s.boards[i].Containers, c)
	return c, true
}

func (s *Store) UpdateContainer(boardID, contID string, c Container) (Container, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return Container{}, false
	}
	for ci, existing := range s.boards[bi].Containers {
		if existing.ID == contID {
			if c.Title != "" {
				s.boards[bi].Containers[ci].Title = c.Title
			}
			if c.AgentID != nil {
				s.boards[bi].Containers[ci].AgentID = c.AgentID
			}
			if c.Geometry != nil {
				s.boards[bi].Containers[ci].Geometry = c.Geometry
			}
			s.boards[bi].Containers[ci].Minimized = c.Minimized
			return s.boards[bi].Containers[ci], true
		}
	}
	return Container{}, false
}

func (s *Store) DeleteContainer(boardID, contID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return false
	}
	for ci, c := range s.boards[bi].Containers {
		if c.ID == contID {
			s.boards[bi].Containers = append(s.boards[bi].Containers[:ci], s.boards[bi].Containers[ci+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) CreateCard(boardID string, c Card) (Card, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return Card{}, false
	}
	c.ID = fmt.Sprintf("card-%03d", s.nextCardN)
	s.nextCardN++
	if c.Status == "" {
		c.Status = "todo"
	}
	if c.Links == nil {
		c.Links = []Edge{}
	}
	s.boards[bi].Cards = append(s.boards[bi].Cards, c)
	return c, true
}

func (s *Store) GetCard(boardID, cardID string) (Card, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return Card{}, false
	}
	for _, c := range s.boards[bi].Cards {
		if c.ID == cardID {
			return c, true
		}
	}
	return Card{}, false
}

func (s *Store) UpdateCard(boardID string, card Card) (Card, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return Card{}, false
	}
	for ci, c := range s.boards[bi].Cards {
		if c.ID == card.ID {
			s.boards[bi].Cards[ci] = card
			return card, true
		}
	}
	return Card{}, false
}

func (s *Store) DeleteCard(boardID, cardID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return false
	}
	for ci, c := range s.boards[bi].Cards {
		if c.ID == cardID {
			s.boards[bi].Cards = append(s.boards[bi].Cards[:ci], s.boards[bi].Cards[ci+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) CreateEdge(boardID string, e Edge) (Edge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return Edge{}, false
	}
	e.ID = fmt.Sprintf("edge-%03d", s.nextEdgeN)
	s.nextEdgeN++
	if e.Type == "" {
		e.Type = "RelatesTo"
	}
	s.boards[bi].Edges = append(s.boards[bi].Edges, e)
	return e, true
}

func (s *Store) UpdateEdge(boardID, edgeID, edgeType string) (Edge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return Edge{}, false
	}
	for ei, e := range s.boards[bi].Edges {
		if e.ID == edgeID {
			s.boards[bi].Edges[ei].Type = edgeType
			return s.boards[bi].Edges[ei], true
		}
	}
	return Edge{}, false
}

func (s *Store) DeleteEdge(boardID, edgeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bi := s.boardIndex(boardID)
	if bi < 0 {
		return false
	}
	for ei, e := range s.boards[bi].Edges {
		if e.ID == edgeID {
			s.boards[bi].Edges = append(s.boards[bi].Edges[:ei], s.boards[bi].Edges[ei+1:]...)
			return true
		}
	}
	return false
}
