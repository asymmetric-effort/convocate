package sup

import (
	"fmt"
	"sync"
	"time"
)

type Ticket struct {
	ID        string `json:"id"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Body      string `json:"body"`
	Reporter  string `json:"reporter"`
	UpdatedAt string `json:"updatedAt"`
}

type DocArticle struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type Store struct {
	mu       sync.Mutex
	tickets  []Ticket
	articles []DocArticle
}

func NewStore() *Store {
	return &Store{
		tickets: []Ticket{},
		articles: []DocArticle{
			{ID: "doc-001", Title: "Getting Started", Slug: "getting-started"},
			{ID: "doc-002", Title: "Node Provisioning Guide", Slug: "node-provisioning"},
			{ID: "doc-003", Title: "Agent Management", Slug: "agent-management"},
		},
	}
}

func (s *Store) ListTickets() []Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := make([]Ticket, len(s.tickets))
	copy(o, s.tickets)
	return o
}
func (s *Store) ListArticles() []DocArticle {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := make([]DocArticle, len(s.articles))
	copy(o, s.articles)
	return o
}

func (s *Store) GetTicket(id string) (Ticket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tickets {
		if t.ID == id {
			return t, true
		}
	}
	return Ticket{}, false
}

func (s *Store) CreateTicket(t Ticket) Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = fmt.Sprintf("tkt-%03d", len(s.tickets)+1)
	t.Status = "open"
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.tickets = append(s.tickets, t)
	return t
}

func (s *Store) UpdateTicket(id string, t Ticket) (Ticket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.tickets {
		if existing.ID == id {
			if t.Subject != "" {
				s.tickets[i].Subject = t.Subject
			}
			if t.Status != "" {
				s.tickets[i].Status = t.Status
			}
			if t.Priority != "" {
				s.tickets[i].Priority = t.Priority
			}
			if t.Body != "" {
				s.tickets[i].Body = t.Body
			}
			s.tickets[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return s.tickets[i], true
		}
	}
	return Ticket{}, false
}

func (s *Store) DeleteTicket(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.tickets {
		if t.ID == id {
			s.tickets = append(s.tickets[:i], s.tickets[i+1:]...)
			return true
		}
	}
	return false
}
