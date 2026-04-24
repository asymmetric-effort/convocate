package multihost

import (
	"errors"
	"strings"
	"testing"

	"github.com/asymmetric-effort/claude-shell/internal/agentclient"
	"github.com/asymmetric-effort/claude-shell/internal/agentserver"
	"github.com/asymmetric-effort/claude-shell/internal/session"
)

// --- stubs ------------------------------------------------------------------

type fakeLocal struct {
	sessions     []session.Metadata
	deleted      string
	overridden   string
	updated      string
	cloneSrc     string
	cloneName    string
	listErr      error
	locked       map[string]bool
}

func (f *fakeLocal) List() ([]session.Metadata, error) { return f.sessions, f.listErr }
func (f *fakeLocal) Get(id string) (session.Metadata, error) {
	for _, s := range f.sessions {
		if s.UUID == id {
			return s, nil
		}
	}
	return session.Metadata{}, errors.New("not found")
}
func (f *fakeLocal) Delete(id string) error       { f.deleted = id; return nil }
func (f *fakeLocal) OverrideLock(id string) error { f.overridden = id; return nil }
func (f *fakeLocal) UpdateWithOptions(id string, opts session.UpdateOptions) (session.Metadata, error) {
	f.updated = id
	return session.Metadata{UUID: id, Name: opts.Name, Port: opts.Port, Protocol: opts.Protocol, DNSName: opts.DNSName}, nil
}
func (f *fakeLocal) CreateWithOptions(name string, opts session.CreateOptions) (session.Metadata, error) {
	return session.Metadata{UUID: "new", Name: name, Port: opts.Port}, nil
}
func (f *fakeLocal) Clone(src, name string) (session.Metadata, error) {
	f.cloneSrc = src
	f.cloneName = name
	return session.Metadata{UUID: "local-clone", Name: name}, nil
}
func (f *fakeLocal) IsLocked(id string) bool { return f.locked[id] }

type fakeAgentClient struct {
	sessions   []session.Metadata
	listErr    error
	deleted    string
	killed     string
	backgrnd   string
	overridden string
	restarted  string
	editID     string
	editOpts   agentserver.EditRequest
	cloneSrc   string
	cloneName  string
}

func (f *fakeAgentClient) List() ([]session.Metadata, error) { return f.sessions, f.listErr }
func (f *fakeAgentClient) Create(req agentserver.CreateRequest) (session.Metadata, error) {
	return session.Metadata{UUID: "agent-new", Name: req.Name}, nil
}
func (f *fakeAgentClient) Edit(req agentserver.EditRequest) (session.Metadata, error) {
	f.editID = req.ID
	f.editOpts = req
	return session.Metadata{UUID: req.ID, Name: req.Name}, nil
}
func (f *fakeAgentClient) Clone(src, name string) (session.Metadata, error) {
	f.cloneSrc = src
	f.cloneName = name
	return session.Metadata{UUID: "remote-clone", Name: name}, nil
}
func (f *fakeAgentClient) Delete(id string) error     { f.deleted = id; return nil }
func (f *fakeAgentClient) Kill(id string) error       { f.killed = id; return nil }
func (f *fakeAgentClient) Background(id string) error { f.backgrnd = id; return nil }
func (f *fakeAgentClient) Override(id string) error   { f.overridden = id; return nil }
func (f *fakeAgentClient) Restart(id string) error    { f.restarted = id; return nil }
func (f *fakeAgentClient) Close() error               { return nil }

// --- tests -----------------------------------------------------------------

func TestRouter_ListNoAgents_IsLocalOnly(t *testing.T) {
	local := &fakeLocal{sessions: []session.Metadata{{UUID: "a", Name: "one"}, {UUID: "b", Name: "two"}}}
	r := &Router{Local: local}
	got, err := r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("got %d, want 2", len(got))
	}
	for _, s := range got {
		if s.IsRemote() {
			t.Errorf("unexpected remote flag on %s", s.UUID)
		}
	}
}

func TestRouter_ListStampsAgentIdentity(t *testing.T) {
	local := &fakeLocal{sessions: []session.Metadata{{UUID: "l1", Name: "local"}}}
	ag := &fakeAgentClient{sessions: []session.Metadata{
		{UUID: "r1", Name: "remote-one"},
		{UUID: "r2", Name: "remote-two"},
	}}
	r := &Router{
		Local: local,
		Agents: []AgentRef{{
			Record: agentclient.AgentRecord{ID: "alpha", Host: "a.host"},
			Client: ag,
		}},
	}
	got, err := r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	// First entry is local (no stamp).
	if got[0].IsRemote() {
		t.Errorf("local entry marked remote")
	}
	// Remote entries stamped with agent id + host.
	for _, s := range got[1:] {
		if s.AgentID != "alpha" || s.AgentHost != "a.host" {
			t.Errorf("%s: agent=(%q,%q)", s.UUID, s.AgentID, s.AgentHost)
		}
	}
}

func TestRouter_OpsRouteByID(t *testing.T) {
	local := &fakeLocal{sessions: []session.Metadata{{UUID: "l1"}}}
	ag := &fakeAgentClient{sessions: []session.Metadata{{UUID: "r1"}}}
	r := &Router{
		Local:           local,
		LocalKill:       func(id string) error { return nil },
		LocalBackground: func(id string) error { return nil },
		LocalRestart:    func(id string) error { return nil },
		Agents: []AgentRef{{
			Record: agentclient.AgentRecord{ID: "A", Host: "h"},
			Client: ag,
		}},
	}
	// Build the routing map.
	if _, err := r.List(); err != nil {
		t.Fatal(err)
	}

	// Delete remote → goes to agent.
	if err := r.Delete("r1"); err != nil {
		t.Fatal(err)
	}
	if ag.deleted != "r1" || local.deleted != "" {
		t.Errorf("delete routing wrong: agent=%q local=%q", ag.deleted, local.deleted)
	}

	// Delete local → goes to Manager.
	if err := r.Delete("l1"); err != nil {
		t.Fatal(err)
	}
	if local.deleted != "l1" {
		t.Errorf("local delete not routed: %q", local.deleted)
	}

	// Kill remote → agent.Kill.
	if err := r.Kill("r1"); err != nil {
		t.Fatal(err)
	}
	if ag.killed != "r1" {
		t.Errorf("agent kill not invoked: %q", ag.killed)
	}

	// Override remote → agent.Override.
	if err := r.OverrideLock("r1"); err != nil {
		t.Fatal(err)
	}
	if ag.overridden != "r1" {
		t.Errorf("agent override not invoked: %q", ag.overridden)
	}

	// Restart remote → agent.Restart.
	if err := r.Restart("r1"); err != nil {
		t.Fatal(err)
	}
	if ag.restarted != "r1" {
		t.Errorf("agent restart not invoked")
	}

	// Update remote → agent.Edit with the right fields.
	if err := r.Update("r1", "new-name", "tcp", "svc.local", 8080); err != nil {
		t.Fatal(err)
	}
	if ag.editID != "r1" || ag.editOpts.Name != "new-name" || ag.editOpts.Port != 8080 {
		t.Errorf("agent edit wrong: %+v", ag.editOpts)
	}

	// Clone remote → agent.Clone.
	cloned, err := r.Clone("r1", "dup")
	if err != nil {
		t.Fatal(err)
	}
	if cloned.AgentID != "A" || cloned.Name != "dup" {
		t.Errorf("clone result: %+v", cloned)
	}
}

func TestRouter_UnknownIDFallsLocal(t *testing.T) {
	local := &fakeLocal{}
	r := &Router{Local: local}
	if err := r.Delete("brand-new"); err != nil {
		t.Fatal(err)
	}
	if local.deleted != "brand-new" {
		t.Errorf("unknown id should fall through to local delete, got %q", local.deleted)
	}
}

func TestRouter_UnreachableAgent_SurfacesInList(t *testing.T) {
	local := &fakeLocal{sessions: []session.Metadata{{UUID: "l1"}}}
	ag := &fakeAgentClient{listErr: errors.New("dial tcp: refused")}
	r := &Router{
		Local: local,
		Agents: []AgentRef{{
			Record: agentclient.AgentRecord{ID: "broken", Host: "down.example"},
			Client: ag,
		}},
	}
	got, err := r.List()
	if err != nil {
		t.Fatalf("List should not error on unreachable agent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (local + sentinel)", len(got))
	}
	sentinel := got[1]
	if !strings.Contains(sentinel.UUID, "agent-unreachable") {
		t.Errorf("sentinel UUID = %q", sentinel.UUID)
	}
	if sentinel.AgentID != "broken" {
		t.Errorf("sentinel AgentID = %q", sentinel.AgentID)
	}
	if !strings.Contains(sentinel.Name, "refused") {
		t.Errorf("sentinel Name = %q; want error text", sentinel.Name)
	}
}

func TestRouter_LocalHandlersRequired(t *testing.T) {
	r := &Router{Local: &fakeLocal{}}
	if err := r.Kill("id"); err == nil || !strings.Contains(err.Error(), "local kill") {
		t.Errorf("expected local-kill-missing error, got %v", err)
	}
}

func TestRouter_IsRunningIsRemoteReturnsFalse(t *testing.T) {
	local := &fakeLocal{sessions: []session.Metadata{{UUID: "l1"}}, locked: map[string]bool{"l1": true}}
	ag := &fakeAgentClient{sessions: []session.Metadata{{UUID: "r1"}}}
	r := &Router{
		Local: local,
		LocalIsRunning: func(id string) bool {
			return id == "l1"
		},
		Agents: []AgentRef{{
			Record: agentclient.AgentRecord{ID: "A"},
			Client: ag,
		}},
	}
	_, _ = r.List()

	if !r.IsRunning("l1") {
		t.Error("local running should be true")
	}
	if r.IsRunning("r1") {
		t.Error("remote running should default to false for now")
	}
	if !r.IsLocked("l1") {
		t.Error("local lock should pass through")
	}
	if r.IsLocked("r1") {
		t.Error("remote lock should be false")
	}
}
