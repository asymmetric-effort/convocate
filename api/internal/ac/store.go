package ac

import (
	"fmt"
	"sync"
)

type User struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Groups []string `json:"groups"`
}

type Group struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Builtin   bool     `json:"builtin"`
	UserCount int      `json:"userCount"`
	Roles     []string `json:"roles"`
}

type Role struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Applet      string `json:"applet"`
}

type GlobalSettings struct {
	RequireMFA           bool `json:"requireMfa"`
	SessionTimeoutMin    int  `json:"sessionTimeoutMinutes"`
	PasswordMinLength    int  `json:"passwordMinLength"`
	PasswordRotationDays int  `json:"passwordRotationDays"`
}

type Store struct {
	mu       sync.Mutex
	users    []User
	groups   []Group
	roles    []Role
	settings GlobalSettings
}

func NewStore() *Store {
	return &Store{
		users: []User{
			{ID: "usr-001", Email: "admin@convocate.local", Name: "Admin User", Status: "active", Groups: []string{"grp-admins"}},
			{ID: "usr-002", Email: "dev@convocate.local", Name: "Developer", Status: "active", Groups: []string{"grp-devs"}},
		},
		groups: []Group{
			{ID: "grp-admins", Name: "admins", Builtin: true, UserCount: 1, Roles: []string{"admin"}},
			{ID: "grp-devs", Name: "developers", Builtin: false, UserCount: 1, Roles: []string{"node-view", "agent-view", "agent-update", "pb-view", "pb-update", "ide-view", "ide-update", "repo-view"}},
		},
		roles: []Role{
			{ID: "admin", Description: "Full access to all features", Applet: "all"},
			{ID: "node-view", Description: "View nodes", Applet: "nmgr"},
			{ID: "node-create", Description: "Provision nodes", Applet: "nmgr"},
			{ID: "node-update", Description: "Start/stop/edit nodes", Applet: "nmgr"},
			{ID: "node-delete", Description: "Decommission nodes", Applet: "nmgr"},
			{ID: "agent-view", Description: "View agents", Applet: "amgr"},
			{ID: "agent-update", Description: "Create/start/stop/configure agents", Applet: "amgr"},
			{ID: "pb-view", Description: "View project boards", Applet: "pb"},
			{ID: "pb-update", Description: "Edit boards, cards, containers, edges", Applet: "pb"},
			{ID: "pb-execute", Description: "Implement/send cards to agents", Applet: "pb"},
			{ID: "ide-view", Description: "View projects and files", Applet: "ide"},
			{ID: "ide-update", Description: "Edit files and create projects", Applet: "ide"},
			{ID: "repo-view", Description: "View repositories and PRs", Applet: "repo"},
			{ID: "repo-update", Description: "Create repositories", Applet: "repo"},
			{ID: "repo-merge", Description: "Merge pull requests", Applet: "repo"},
			{ID: "access-view", Description: "View users, groups, settings", Applet: "ac"},
			{ID: "access-update", Description: "Manage users, groups, settings", Applet: "ac"},
			{ID: "support-view", Description: "View and create tickets", Applet: "sup"},
		},
		settings: GlobalSettings{RequireMFA: false, SessionTimeoutMin: 30, PasswordMinLength: 12, PasswordRotationDays: 90},
	}
}

func (s *Store) ListUsers() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := make([]User, len(s.users))
	copy(o, s.users)
	return o
}
func (s *Store) ListGroups() []Group {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := make([]Group, len(s.groups))
	copy(o, s.groups)
	return o
}
func (s *Store) ListRoles() []Role {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := make([]Role, len(s.roles))
	copy(o, s.roles)
	return o
}
func (s *Store) GetSettings() GlobalSettings { s.mu.Lock(); defer s.mu.Unlock(); return s.settings }

func (s *Store) SetSettings(gs GlobalSettings) GlobalSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = gs
	return s.settings
}

func (s *Store) CreateUser(u User) User {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.ID = fmt.Sprintf("usr-%03d", len(s.users)+1)
	u.Status = "active"
	s.users = append(s.users, u)
	return u
}

func (s *Store) UpdateUser(id string, u User) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.users {
		if existing.ID == id {
			if u.Email != "" {
				s.users[i].Email = u.Email
			}
			if u.Name != "" {
				s.users[i].Name = u.Name
			}
			if u.Status != "" {
				s.users[i].Status = u.Status
			}
			if u.Groups != nil {
				s.users[i].Groups = u.Groups
			}
			return s.users[i], true
		}
	}
	return User{}, false
}

func (s *Store) DeleteUser(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.users {
		if u.ID == id {
			s.users = append(s.users[:i], s.users[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) CreateGroup(name string) Group {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := Group{ID: fmt.Sprintf("grp-%03d", len(s.groups)+1), Name: name}
	s.groups = append(s.groups, g)
	return g
}

func (s *Store) DeleteGroup(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, g := range s.groups {
		if g.ID == id && !g.Builtin {
			s.groups = append(s.groups[:i], s.groups[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) SetGroupUsers(id string, userIDs []string) (Group, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, g := range s.groups {
		if g.ID == id {
			s.groups[i].UserCount = len(userIDs)
			return s.groups[i], true
		}
	}
	return Group{}, false
}

func (s *Store) SetGroupRoles(id string, roles []string) (Group, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, g := range s.groups {
		if g.ID == id {
			s.groups[i].Roles = roles
			return s.groups[i], true
		}
	}
	return Group{}, false
}
