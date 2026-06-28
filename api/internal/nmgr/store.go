package nmgr

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

type Node struct {
	ID          string   `json:"id"`
	Location    string   `json:"location"`
	IP          string   `json:"ip"`
	Status      string   `json:"status"`
	Agents      int      `json:"agents"`
	LoadAvg     LoadAvg  `json:"loadAvg"`
	MemUsedGB   float64  `json:"memUsedGB"`
	MemTotalGB  float64  `json:"memTotalGB"`
	DiskUsedGB  float64  `json:"diskUsedGB"`
	DiskTotalGB float64  `json:"diskTotalGB"`
	Tags        []string `json:"tags"`
}

type LoadAvg struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

type Note struct {
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
	Text      string `json:"text"`
}

type Store struct {
	mu    sync.Mutex
	nodes []Node
	notes map[string][]Note
}

func NewStore() *Store {
	return &Store{
		nodes: []Node{
			{ID: "cnn-7f3a", Location: "us-east-1", IP: "10.0.1.10", Status: "Ready", Agents: 3, LoadAvg: LoadAvg{0.5, 0.7, 0.6}, MemUsedGB: 12.5, MemTotalGB: 32, DiskUsedGB: 80, DiskTotalGB: 500, Tags: []string{"cpu:amd64", "os:linux"}},
			{ID: "cnn-a1b2", Location: "us-west-2", IP: "10.0.2.20", Status: "NotReady", Agents: 0, LoadAvg: LoadAvg{0, 0, 0}, MemUsedGB: 0, MemTotalGB: 64, DiskUsedGB: 120, DiskTotalGB: 1000, Tags: []string{"cpu:amd64", "os:linux", "gpu:nvidia"}},
		},
		notes: map[string][]Note{
			"cnn-7f3a": {{Author: "system", CreatedAt: time.Now().UTC().Format(time.RFC3339), Text: "Node provisioned successfully"}},
		},
	}
}

func (s *Store) List() []Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Node, len(s.nodes))
	copy(out, s.nodes)
	return out
}

func (s *Store) Get(id string) (Node, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

func (s *Store) Create(n Node) Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	n.ID = fmt.Sprintf("cnn-%04d", len(s.nodes)+1)
	if n.Status == "" {
		n.Status = "Pending"
	}
	s.nodes = append(s.nodes, n)
	return n
}

func (s *Store) Update(id string, location *string, tags []string) (Node, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, n := range s.nodes {
		if n.ID == id {
			if location != nil {
				s.nodes[i].Location = *location
			}
			if tags != nil {
				s.nodes[i].Tags = tags
			}
			return s.nodes[i], true
		}
	}
	return Node{}, false
}

func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, n := range s.nodes {
		if n.ID == id {
			s.nodes = append(s.nodes[:i], s.nodes[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) SetStatus(id, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, n := range s.nodes {
		if n.ID == id {
			s.nodes[i].Status = status
			return true
		}
	}
	return false
}

func (s *Store) ListNotes(nodeID string) []Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	notes := s.notes[nodeID]
	out := make([]Note, len(notes))
	copy(out, notes)
	return out
}

func (s *Store) AddNote(nodeID string, note Note) Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	note.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	s.notes[nodeID] = append(s.notes[nodeID], note)
	return note
}

// JitterMetrics applies small random changes to node metrics to
// simulate real-time resource usage in mock mode.
func (s *Store) JitterMetrics() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.nodes {
		if s.nodes[i].Status != "Ready" {
			continue
		}
		n := &s.nodes[i]
		n.LoadAvg.One = math.Max(0, n.LoadAvg.One+(rand.Float64()-0.5)*0.3)
		n.LoadAvg.Five = math.Max(0, n.LoadAvg.Five+(rand.Float64()-0.5)*0.2)
		n.LoadAvg.Fifteen = math.Max(0, n.LoadAvg.Fifteen+(rand.Float64()-0.5)*0.1)
		n.MemUsedGB = math.Max(0, math.Min(n.MemTotalGB, n.MemUsedGB+(rand.Float64()-0.5)*1.0))
		n.DiskUsedGB = math.Max(0, math.Min(n.DiskTotalGB, n.DiskUsedGB+(rand.Float64()-0.5)*0.5))
	}
}
