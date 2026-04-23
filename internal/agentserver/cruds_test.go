package agentserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/session"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

// newTestSessionsDir returns (base, skel) paths for a session.Manager that
// works in a temp dir — enough for CRUD ops that don't actually start docker.
func newTestSessionsDir(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	skel := filepath.Join(base, "skel")
	if err := os.MkdirAll(skel, 0750); err != nil {
		t.Fatal(err)
	}
	return base, skel
}

func testUserInfo() user.Info {
	return user.Info{UID: 1337, GID: 1337, Username: "claude", HomeDir: "/home/claude"}
}

func testPaths() config.Paths {
	return config.Paths{
		ClaudeHome:   "/home/claude",
		SessionsBase: "/home/claude",
		SkelDir:      "/home/claude/.skel",
		ClaudeConfig: "/home/claude/.claude",
		SSHDir:       "/home/claude/.ssh",
		GitConfig:    "/home/claude/.gitconfig",
	}
}

// fakeOrch is a minimal Orchestrator that records every call and returns
// canned results. Lets us exercise the RPC layer without touching real
// sessions/containers.
type fakeOrch struct {
	listErr      error
	sessions     []session.Metadata
	getErr       error
	getMeta      session.Metadata
	createErr    error
	createCalled session.CreateOptions
	createName   string
	updateErr    error
	updateID     string
	updateOpts   session.UpdateOptions
	cloneSrc     string
	cloneName    string
	deleted      string
	overridden   string
	killed       string
	backgrounded string
	restarted    string
	restartErr   error
}

func (f *fakeOrch) List() ([]session.Metadata, error) { return f.sessions, f.listErr }
func (f *fakeOrch) Get(id string) (session.Metadata, error) {
	if f.getErr != nil {
		return session.Metadata{}, f.getErr
	}
	return f.getMeta, nil
}
func (f *fakeOrch) Create(opts session.CreateOptions, name string) (session.Metadata, error) {
	f.createCalled = opts
	f.createName = name
	if f.createErr != nil {
		return session.Metadata{}, f.createErr
	}
	return session.Metadata{UUID: "new-uuid", Name: name, Port: opts.Port, Protocol: opts.Protocol, DNSName: opts.DNSName}, nil
}
func (f *fakeOrch) Update(id string, opts session.UpdateOptions) (session.Metadata, error) {
	f.updateID = id
	f.updateOpts = opts
	if f.updateErr != nil {
		return session.Metadata{}, f.updateErr
	}
	return session.Metadata{UUID: id, Name: opts.Name, Port: opts.Port, Protocol: opts.Protocol, DNSName: opts.DNSName}, nil
}
func (f *fakeOrch) Clone(src, name string) (session.Metadata, error) {
	f.cloneSrc = src
	f.cloneName = name
	return session.Metadata{UUID: "cloned", Name: name}, nil
}
func (f *fakeOrch) Delete(id string) error       { f.deleted = id; return nil }
func (f *fakeOrch) OverrideLock(id string) error { f.overridden = id; return nil }
func (f *fakeOrch) Kill(id string) error         { f.killed = id; return nil }
func (f *fakeOrch) Background(id string) error   { f.backgrounded = id; return nil }
func (f *fakeOrch) Restart(id string) error      { f.restarted = id; return f.restartErr }

// callOp drives the dispatcher the same way an SSH client would — JSON
// request in, JSON response out.
func callOp(t *testing.T, d *Dispatcher, op string, params any) Response {
	t.Helper()
	body := Request{Op: op}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		body.Params = raw
	}
	reqBytes, _ := json.Marshal(body)
	var out bytes.Buffer
	d.Handle(bytes.NewReader(reqBytes), &out)
	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	return resp
}

func newTestDispatcher(o Orchestrator) *Dispatcher {
	d := NewDispatcher()
	RegisterCoreOps(d, "agent-id", "v-test")
	RegisterCRUDOps(d, o)
	return d
}

func TestCRUD_List(t *testing.T) {
	f := &fakeOrch{sessions: []session.Metadata{{UUID: "a", Name: "one"}, {UUID: "b", Name: "two"}}}
	d := newTestDispatcher(f)
	resp := callOp(t, d, "list", nil)
	if !resp.OK {
		t.Fatalf("list failed: %s", resp.Error)
	}
	var metas []session.Metadata
	if err := json.Unmarshal(resp.Result, &metas); err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Errorf("got %d metas, want 2", len(metas))
	}
}

func TestCRUD_List_Error(t *testing.T) {
	f := &fakeOrch{listErr: errors.New("disk gone")}
	d := newTestDispatcher(f)
	resp := callOp(t, d, "list", nil)
	if resp.OK {
		t.Fatal("expected error")
	}
	if !strings.Contains(resp.Error, "disk gone") {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestCRUD_Get(t *testing.T) {
	f := &fakeOrch{getMeta: session.Metadata{UUID: "u1", Name: "pname", Port: 8080, Protocol: "tcp"}}
	d := newTestDispatcher(f)
	resp := callOp(t, d, "get", IDRequest{ID: "u1"})
	if !resp.OK {
		t.Fatalf("get failed: %s", resp.Error)
	}
	var meta session.Metadata
	if err := json.Unmarshal(resp.Result, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.UUID != "u1" || meta.Port != 8080 {
		t.Errorf("unexpected: %+v", meta)
	}
}

func TestCRUD_Create_PassesAllFields(t *testing.T) {
	f := &fakeOrch{}
	d := newTestDispatcher(f)
	params := CreateRequest{Name: "dns", Port: 53, Protocol: "udp", DNSName: "dns.local"}
	resp := callOp(t, d, "create", params)
	if !resp.OK {
		t.Fatalf("create failed: %s", resp.Error)
	}
	if f.createName != "dns" {
		t.Errorf("name = %q", f.createName)
	}
	if f.createCalled.Port != 53 || f.createCalled.Protocol != "udp" || f.createCalled.DNSName != "dns.local" {
		t.Errorf("create opts = %+v", f.createCalled)
	}
}

func TestCRUD_Edit(t *testing.T) {
	f := &fakeOrch{}
	d := newTestDispatcher(f)
	params := EditRequest{ID: "u1", Name: "renamed", Port: 9090, Protocol: "tcp", DNSName: "svc.local"}
	resp := callOp(t, d, "edit", params)
	if !resp.OK {
		t.Fatalf("edit: %s", resp.Error)
	}
	if f.updateID != "u1" {
		t.Errorf("id = %q", f.updateID)
	}
	if f.updateOpts.Name != "renamed" || f.updateOpts.DNSName != "svc.local" {
		t.Errorf("opts = %+v", f.updateOpts)
	}
}

func TestCRUD_Clone(t *testing.T) {
	f := &fakeOrch{}
	d := newTestDispatcher(f)
	resp := callOp(t, d, "clone", CloneRequest{SourceID: "src", Name: "new"})
	if !resp.OK {
		t.Fatalf("clone: %s", resp.Error)
	}
	if f.cloneSrc != "src" || f.cloneName != "new" {
		t.Errorf("clone args src=%q name=%q", f.cloneSrc, f.cloneName)
	}
}

func TestCRUD_Delete_Kill_Background_Override_Restart(t *testing.T) {
	// These ops all share the same IDRequest shape — sweep them together.
	f := &fakeOrch{}
	d := newTestDispatcher(f)
	ops := []struct {
		op     string
		assign func() string
	}{
		{"delete", func() string { return f.deleted }},
		{"kill", func() string { return f.killed }},
		{"background", func() string { return f.backgrounded }},
		{"override", func() string { return f.overridden }},
		{"restart", func() string { return f.restarted }},
	}
	for _, op := range ops {
		resp := callOp(t, d, op.op, IDRequest{ID: "the-id"})
		if !resp.OK {
			t.Fatalf("%s: %s", op.op, resp.Error)
		}
		if op.assign() != "the-id" {
			t.Errorf("%s: orchestrator id = %q, want 'the-id'", op.op, op.assign())
		}
	}
}

func TestCRUD_Restart_PropagatesError(t *testing.T) {
	f := &fakeOrch{restartErr: errors.New("already running")}
	d := newTestDispatcher(f)
	resp := callOp(t, d, "restart", IDRequest{ID: "x"})
	if resp.OK {
		t.Fatal("expected error")
	}
	if !strings.Contains(resp.Error, "already running") {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestCRUD_RejectsUnknownFields(t *testing.T) {
	d := newTestDispatcher(&fakeOrch{})
	// Hand-crafted JSON with a typo'd field — must not silently succeed.
	reqBytes := []byte(`{"op":"get","params":{"id":"u1","typo":"boom"}}`)
	var out bytes.Buffer
	d.Handle(bytes.NewReader(reqBytes), &out)
	var resp Response
	_ = json.Unmarshal(out.Bytes(), &resp)
	if resp.OK {
		t.Error("expected unknown-field rejection")
	}
}

func TestCRUD_SettingsPlaceholders(t *testing.T) {
	d := newTestDispatcher(&fakeOrch{})
	for _, op := range []string{"settings-get", "settings-set"} {
		resp := callOp(t, d, op, nil)
		if !resp.OK {
			t.Errorf("%s returned ok=false: %s", op, resp.Error)
		}
	}
}

// --- SessionOrchestrator integration tests --------------------------------
//
// These drive the production SessionOrchestrator against a real
// session.Manager on a temp dir, but stub out container-starting so docker
// isn't required.

func TestSessionOrchestrator_CRUDRoundTrip(t *testing.T) {
	base, skelDir := newTestSessionsDir(t)
	mgr := session.NewManager(base, skelDir)
	o := NewSessionOrchestrator(mgr, testUserInfo(), testPaths(), "")

	// Create
	meta, err := o.Create(session.CreateOptions{Port: 8080, Protocol: "tcp"}, "svc")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if meta.Port != 8080 {
		t.Errorf("Port = %d, want 8080", meta.Port)
	}

	// Get
	got, err := o.Get(meta.UUID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "svc" {
		t.Errorf("Name = %q", got.Name)
	}

	// List
	list, err := o.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}

	// Update
	upd, err := o.Update(meta.UUID, session.UpdateOptions{Name: "renamed", Port: 8081, Protocol: "tcp"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Name != "renamed" || upd.Port != 8081 {
		t.Errorf("update result = %+v", upd)
	}

	// Clone
	cloned, err := o.Clone(meta.UUID, "cloned")
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if cloned.UUID == meta.UUID {
		t.Error("clone should have a new UUID")
	}

	// Delete (need to delete clone first to leave the source, then source)
	if err := o.Delete(cloned.UUID); err != nil {
		t.Errorf("Delete clone: %v", err)
	}
	if err := o.Delete(meta.UUID); err != nil {
		t.Errorf("Delete source: %v", err)
	}
}

func TestSessionOrchestrator_KillBackground_UseHooks(t *testing.T) {
	var killed, detached string
	o := &SessionOrchestrator{
		StopFn:   func(id string) error { killed = id; return nil },
		DetachFn: func(id string) error { detached = id; return nil },
	}
	if err := o.Kill("abc"); err != nil {
		t.Fatal(err)
	}
	if err := o.Background("def"); err != nil {
		t.Fatal(err)
	}
	if killed != "abc" || detached != "def" {
		t.Errorf("Kill=%q Background=%q", killed, detached)
	}
}

func TestSessionOrchestrator_Restart_MissingSession(t *testing.T) {
	base, skelDir := newTestSessionsDir(t)
	mgr := session.NewManager(base, skelDir)
	o := NewSessionOrchestrator(mgr, testUserInfo(), testPaths(), "")
	err := o.Restart("missing-uuid")
	if err == nil {
		t.Error("expected error restarting missing session")
	}
}

func TestCRUD_OpListIsStable(t *testing.T) {
	d := newTestDispatcher(&fakeOrch{})
	ops := d.Ops()
	// Spot-check: must include every op we promised the client.
	required := []string{
		"ping", "list", "get", "create", "edit", "clone",
		"delete", "kill", "background", "override", "restart",
		"settings-get", "settings-set",
	}
	for _, r := range required {
		found := false
		for _, o := range ops {
			if o == r {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing op %q in registered set: %v", r, ops)
		}
	}
}
