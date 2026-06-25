package amgr

import (
	"fmt"
	"sync"
)

type Agent struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	NodeID  string `json:"nodeId"`
	Status  string `json:"status"`
	Expose  string `json:"expose,omitempty"`
	Owner   string `json:"owner"`
}

type Store struct {
	mu     sync.Mutex
	agents []Agent
}

func NewStore() *Store {
	return &Store{agents: []Agent{
		{ID: "agt-7f3a-01", Project: "demo-app", NodeID: "cnn-7f3a", Status: "running", Expose: "demo.convocate.local:3000", Owner: "admin:admins"},
		{ID: "agt-7f3a-02", Project: "api-svc", NodeID: "cnn-7f3a", Status: "stopped", Owner: "admin:admins"},
		{ID: "agt-7f3a-03", Project: "ml-pipeline", NodeID: "cnn-7f3a", Status: "running", Owner: "admin:admins"},
	}}
}

func (s *Store) List() []Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Agent, len(s.agents))
	copy(out, s.agents)
	return out
}

func (s *Store) Get(id string) (Agent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.agents {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}

func (s *Store) Create(a Agent) Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.ID = fmt.Sprintf("agt-%04d", len(s.agents)+1)
	a.Status = "running"
	s.agents = append(s.agents, a)
	return a
}

func (s *Store) Update(id string, project, nodeID, expose *string) (Agent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.agents {
		if a.ID == id {
			if project != nil {
				s.agents[i].Project = *project
			}
			if nodeID != nil {
				s.agents[i].NodeID = *nodeID
			}
			if expose != nil {
				s.agents[i].Expose = *expose
			}
			return s.agents[i], true
		}
	}
	return Agent{}, false
}

func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.agents {
		if a.ID == id {
			s.agents = append(s.agents[:i], s.agents[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) SetStatus(id, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.agents {
		if a.ID == id {
			s.agents[i].Status = status
			return true
		}
	}
	return false
}
